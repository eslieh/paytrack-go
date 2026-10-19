// Package app holds the notify module's use cases: turning payment events
// into webhook deliveries and dispatching them.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/notify/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
)

var ErrNotFound = errors.New("delivery not found")

type Repository interface {
	DeliveryStats(ctx context.Context, since time.Time) (*Stats, error)
	// Create inserts d; a second delivery for the same event ID is ignored.
	Create(ctx context.Context, d *domain.Delivery) error
	Get(ctx context.Context, tenantID, id string) (*domain.Delivery, error)
	Save(ctx context.Context, d *domain.Delivery) error
	List(ctx context.Context, f ListFilter) ([]*domain.Delivery, error)
	// ClaimDue marks due deliveries as delivering with a lease so other
	// dispatcher replicas skip them; expired leases are reclaimed.
	ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]*domain.Delivery, error)
}

type ListFilter struct {
	TenantID  string
	PaymentID string
	Status    string
	Before    string
	Limit     int
}

type Target struct {
	URL    string
	Secret string
}

// TargetProvider returns a tenant's current webhook endpoint. It is looked
// up per attempt so URL changes and secret rotations apply to retries.
type TargetProvider interface {
	Target(ctx context.Context, tenantID string) (Target, error)
}

type Sender interface {
	Send(ctx context.Context, url string, headers map[string]string, body []byte) (statusCode int, err error)
}

// Prefs are a tenant's email preferences and where to send them.
type Prefs struct {
	BusinessName         string
	Recipient            string // receipt email or owner email
	PaymentReceipts      bool
	WebhookFailureAlerts bool
}

type PrefsProvider interface {
	Prefs(ctx context.Context, tenantID string) (Prefs, error)
}

type Mailer interface {
	Enqueue(ctx context.Context, e contracts.Email) error
}

// Inbox posts dashboard notifications (the inbox module).
type Inbox interface {
	Deliver(ctx context.Context, n contracts.Notification) error
}

type Service struct {
	repo        Repository
	targets     TargetProvider
	sender      Sender
	prefs       PrefsProvider
	mailer      Mailer
	inbox       Inbox // optional
	appURL      string
	concurrency int
	now         func() time.Time
}

func NewService(repo Repository, targets TargetProvider, sender Sender, prefs PrefsProvider, mailer Mailer, appURL string) *Service {
	return &Service{repo: repo, targets: targets, sender: sender, prefs: prefs, mailer: mailer, appURL: appURL,
		concurrency: 16, now: time.Now}
}

// Wire connects the optional dashboard inbox.
func (s *Service) Wire(inbox Inbox) { s.inbox = inbox }

// notifyDashboard posts an in-app notification; id makes it idempotent.
func (s *Service) notifyDashboard(ctx context.Context, id, tenantID string, n contracts.InApp) {
	if s.inbox == nil {
		return
	}
	if err := s.inbox.Deliver(ctx, contracts.Notification{ID: id, TenantID: tenantID, InApp: &n}); err != nil {
		slog.ErrorContext(ctx, "dashboard notification failed", "id", id, "err", err)
	}
}

// Enqueue records a delivery for a payment event (idempotent per event ID).
func (s *Service) Enqueue(ctx context.Context, ev contracts.PaymentEvent) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return s.repo.Create(ctx, domain.NewDelivery(ids.New("whd"), ev.ID, ev.TenantID, ev.Data.ID, ev.Type, body, s.now()))
}

// Dispatch sends one batch of due deliveries concurrently.
func (s *Service) Dispatch(ctx context.Context) (more bool, err error) {
	batch := s.concurrency * 4
	due, err := s.repo.ClaimDue(ctx, s.now(), batch, 2*time.Minute)
	if err != nil {
		return false, err
	}
	sem := make(chan struct{}, s.concurrency)
	var wg sync.WaitGroup
	for _, d := range due {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			s.attempt(ctx, d)
		})
	}
	wg.Wait()
	return len(due) == batch, nil
}

func (s *Service) attempt(ctx context.Context, d *domain.Delivery) {
	log := slog.With("delivery_id", d.ID, "tenant_id", d.TenantID, "event_id", d.EventID)
	t, err := s.targets.Target(ctx, d.TenantID)
	switch {
	case err != nil:
		d.RecordFailure(0, fmt.Sprintf("loading webhook target: %v", err), s.now())
	case t.URL == "":
		d.RecordFailure(0, "no webhook URL configured", s.now())
	default:
		code, sendErr := s.sender.Send(ctx, t.URL, map[string]string{
			"Content-Type":         "application/json",
			"User-Agent":           "PayTrack-Webhooks/1",
			"PayTrack-Event-Id":    d.EventID,
			"PayTrack-Event-Type":  d.EventType,
			"PayTrack-Delivery-Id": d.ID,
			domain.SignatureHeader: domain.Sign(t.Secret, s.now(), d.Payload),
		}, d.Payload)
		switch {
		case sendErr != nil:
			d.RecordFailure(code, sendErr.Error(), s.now())
		case code >= 200 && code < 300:
			d.RecordSuccess(code, s.now())
		default:
			d.RecordFailure(code, fmt.Sprintf("endpoint returned HTTP %d", code), s.now())
		}
	}
	if d.Status == domain.StatusFailed {
		log.Warn("webhook delivery exhausted retries", "last_error", d.LastError)
		s.alertWebhookFailed(ctx, d, t.URL)
	}
	// Saving after a cancelled ctx would lose the result; the lease then
	// expires and the delivery is retried (receivers dedupe on event ID).
	if err := s.repo.Save(context.WithoutCancel(ctx), d); err != nil {
		log.Error("saving delivery failed", "err", err)
	}
}

func (s *Service) List(ctx context.Context, f ListFilter) ([]*domain.Delivery, error) {
	return s.repo.List(ctx, f)
}

func (s *Service) Redeliver(ctx context.Context, tenantID, id string) (*domain.Delivery, error) {
	d, err := s.repo.Get(ctx, tenantID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, errs.NotFound("delivery_not_found", "delivery not found")
	}
	if err != nil {
		return nil, err
	}
	if d.Status == domain.StatusDelivering {
		return nil, errs.Conflict("delivery_in_progress", "delivery is currently being attempted")
	}
	d.Redeliver(s.now())
	return d, s.repo.Save(ctx, d)
}
