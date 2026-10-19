// Package app holds the mail module's use cases: queue an email, render it
// with the shared templates, and deliver it through a provider.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"sync"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/mail/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
)

type Repository interface {
	MailStats(ctx context.Context, since time.Time) (*Stats, error)
	Create(ctx context.Context, m *domain.Message) error // ignores duplicate dedupe keys
	Save(ctx context.Context, m *domain.Message) error
	ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]*domain.Message, error)
}

type Rendered struct {
	Subject string
	HTML    string
	Text    string
}

type Renderer interface {
	Render(template string, data map[string]any) (Rendered, error)
}

type Outgoing struct {
	From, To, ReplyTo string
	Rendered
	IdempotencyKey string
	Tags           map[string]string
}

// Sender delivers one email. PermanentError marks failures not worth
// retrying (invalid address, rejected content).
type Sender interface {
	Send(ctx context.Context, msg Outgoing) (providerID string, err error)
}

type PermanentError struct{ Err error }

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

type Service struct {
	repo     Repository
	renderer Renderer
	sender   Sender
	from     string
	replyTo  string
	appURL   string
	now      func() time.Time
}

func NewService(repo Repository, renderer Renderer, sender Sender, from, replyTo, appURL string) *Service {
	return &Service{repo: repo, renderer: renderer, sender: sender, from: from, replyTo: replyTo, appURL: appURL, now: time.Now}
}

// Enqueue validates and queues an email; the worker sends it. It satisfies
// every module's Mailer port.
func (s *Service) Enqueue(ctx context.Context, e contracts.Email) error {
	if _, err := mail.ParseAddress(e.To); err != nil {
		return fmt.Errorf("mail: invalid recipient %q", e.To)
	}
	// Render once up front so template/data bugs surface at the call site.
	if _, err := s.renderer.Render(e.Template, s.withDefaults(e.Data)); err != nil {
		return fmt.Errorf("mail: %w", err)
	}
	now := s.now()
	return s.repo.Create(ctx, &domain.Message{
		ID: ids.New("eml"), DedupeKey: e.DedupeKey, Template: e.Template, Category: e.Category,
		ToEmail: e.To, ToName: e.ToName, Data: e.Data, Status: domain.StatusQueued,
		NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now,
	})
}

// Preview renders a template with sample or provided data (dashboard preview, dev).
func (s *Service) Preview(template string, data map[string]any) (Rendered, error) {
	return s.renderer.Render(template, s.withDefaults(data))
}

func (s *Service) withDefaults(data map[string]any) map[string]any {
	out := map[string]any{"AppURL": s.appURL, "Year": s.now().Year()}
	for k, v := range data {
		out[k] = v
	}
	return out
}

// Dispatch sends one batch of due messages. Safe on every worker replica.
func (s *Service) Dispatch(ctx context.Context) (more bool, err error) {
	const batch = 25
	due, err := s.repo.ClaimDue(ctx, s.now(), batch, 2*time.Minute)
	if err != nil {
		return false, err
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for _, m := range due {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			s.send(ctx, m)
		})
	}
	wg.Wait()
	return len(due) == batch, nil
}

func (s *Service) send(ctx context.Context, m *domain.Message) {
	log := slog.With("email_id", m.ID, "template", m.Template)
	r, err := s.renderer.Render(m.Template, s.withDefaults(m.Data))
	if err != nil {
		m.RecordFailure(err.Error(), true, s.now())
	} else {
		to := m.ToEmail
		if m.ToName != "" {
			to = (&mail.Address{Name: m.ToName, Address: m.ToEmail}).String()
		}
		sctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		id, sendErr := s.sender.Send(sctx, Outgoing{
			From: s.from, To: to, ReplyTo: s.replyTo, Rendered: r, IdempotencyKey: m.ID,
			Tags: map[string]string{"template": m.Template, "category": m.Category},
		})
		cancel()
		var perm *PermanentError
		switch {
		case sendErr == nil:
			m.RecordSent(id, s.now())
		case errors.As(sendErr, &perm):
			m.RecordFailure(sendErr.Error(), true, s.now())
		default:
			m.RecordFailure(sendErr.Error(), false, s.now())
		}
	}
	if m.Status == domain.StatusFailed {
		log.Error("email failed", "err", m.LastError)
	}
	if err := s.repo.Save(context.WithoutCancel(ctx), m); err != nil {
		log.Error("saving email failed", "err", err)
	}
}
