// Package app holds the support module's use cases: tenant users open
// tickets and reply; staff answer, add internal notes, and change status
// and priority. Customers are emailed an excerpt and a link (and notified
// in the dashboard) when staff reply or resolve; staff are alerted at
// SUPPORT_NOTIFY_EMAIL when customers write. Notifications go out through
// the outbox with the change that caused them.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
	"github.com/eslieh/paytrack-go/server/internal/support/domain"
)

type Config struct {
	AppBaseURL string // dashboard URL used in email links
	StaffEmail string // where new-ticket and customer-reply alerts go; empty disables them
}

type Service struct {
	repo     Repository
	accounts Accounts
	limiter  RateLimiter
	cfg      Config
	now      func() time.Time
}

func NewService(repo Repository, accounts Accounts, limiter RateLimiter, cfg Config) *Service {
	return &Service{repo: repo, accounts: accounts, limiter: limiter, cfg: cfg, now: time.Now}
}

const excerptLen = 280

var errTicketNotFound = errs.NotFound("ticket_not_found", "ticket not found")

// ---------------------------------------------------------------- tenant users

func (s *Service) CreateTicket(ctx context.Context, p auth.Principal, n domain.NewTicket) (*domain.Ticket, error) {
	subject, cat, body, err := n.Validate()
	if err != nil {
		return nil, err
	}
	if err := s.limit(ctx, "support-ticket:"+p.UserID, 10, time.Hour); err != nil {
		return nil, err
	}
	email, _, err := s.accounts.UserContact(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	name, err := s.accounts.TenantName(ctx, p.TenantID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	t := &domain.Ticket{
		ID: ids.New("tkt"), TenantID: p.TenantID, UserID: p.UserID, RequesterEmail: email, TenantName: name,
		Subject: subject, Category: cat, Priority: domain.PriorityNormal, Status: domain.StatusAwaitingStaff,
		LastMessageAt: now, CreatedAt: now, UpdatedAt: now,
	}
	m := &domain.Message{ID: ids.New("msg"), TicketID: t.ID, AuthorKind: domain.AuthorUser, AuthorID: p.UserID,
		AuthorName: email, Body: body, CreatedAt: now}
	err = s.repo.CreateTicket(ctx, t, m, func() []contracts.Notification {
		return append([]contracts.Notification{s.email(t, "ticket_received", s.ticketData(t, body, false))},
			s.staffAlert(t, body, "New ticket")...)
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) ListTickets(ctx context.Context, p auth.Principal, before string, limit int) ([]*domain.Ticket, error) {
	return s.repo.ListTickets(ctx, TicketFilter{TenantID: p.TenantID, Before: before, Limit: limit})
}

// GetTicket returns one of the caller's tenant's tickets, without internal notes.
func (s *Service) GetTicket(ctx context.Context, p auth.Principal, id string) (*domain.Ticket, []*domain.Message, error) {
	t, err := s.tenantTicket(ctx, p, id)
	if err != nil {
		return nil, nil, err
	}
	msgs, err := s.repo.ListMessages(ctx, t.ID, false)
	return t, msgs, err
}

func (s *Service) Reply(ctx context.Context, p auth.Principal, id, body string) (*domain.Message, error) {
	body, err := domain.ValidateBody(body)
	if err != nil {
		return nil, err
	}
	if err := s.limit(ctx, "support-reply:"+p.UserID, 60, time.Hour); err != nil {
		return nil, err
	}
	t, err := s.tenantTicket(ctx, p, id)
	if err != nil {
		return nil, err
	}
	now := s.now()
	if err := t.UserReplied(now); err != nil {
		return nil, err
	}
	email, _, err := s.accounts.UserContact(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	m := &domain.Message{ID: ids.New("msg"), TicketID: t.ID, AuthorKind: domain.AuthorUser, AuthorID: p.UserID,
		AuthorName: email, Body: body, CreatedAt: now}
	if err := s.repo.AddMessage(ctx, t, m, s.staffAlert(t, body, "New reply")); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Service) Rate(ctx context.Context, p auth.Principal, id string, rating int, comment string) (*domain.Ticket, error) {
	t, err := s.tenantTicket(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if err := t.Rate(rating, comment, s.now()); err != nil {
		return nil, err
	}
	return t, s.repo.SaveTicket(ctx, t, nil)
}

func (s *Service) SendFeedback(ctx context.Context, p auth.Principal, rating *int, message, page string) error {
	if err := s.limit(ctx, "support-feedback:"+p.UserID, 10, time.Hour); err != nil {
		return err
	}
	email, _, err := s.accounts.UserContact(ctx, p.UserID)
	if err != nil {
		return err
	}
	f := &domain.Feedback{ID: ids.New("fbk"), TenantID: p.TenantID, UserID: p.UserID, UserEmail: email,
		Rating: rating, Message: message, Page: page, CreatedAt: s.now()}
	if err := f.Validate(); err != nil {
		return err
	}
	return s.repo.CreateFeedback(ctx, f)
}

// tenantTicket hides other tenants' tickets behind not-found.
func (s *Service) tenantTicket(ctx context.Context, p auth.Principal, id string) (*domain.Ticket, error) {
	t, err := s.ticket(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.TenantID != p.TenantID {
		return nil, errTicketNotFound
	}
	return t, nil
}

// ---------------------------------------------------------------- staff

func (s *Service) StaffListTickets(ctx context.Context, f TicketFilter) ([]*domain.Ticket, error) {
	if f.Status != "" && !f.Status.Valid() {
		return nil, errs.Invalid("invalid_status", "unknown status filter")
	}
	return s.repo.ListTickets(ctx, f)
}

// StaffGetTicket includes internal notes.
func (s *Service) StaffGetTicket(ctx context.Context, id string) (*domain.Ticket, []*domain.Message, error) {
	t, err := s.ticket(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	msgs, err := s.repo.ListMessages(ctx, t.ID, true)
	return t, msgs, err
}

// StaffReply answers the customer, or adds an internal note that only staff see.
func (s *Service) StaffReply(ctx context.Context, p auth.Principal, id, body string, internal bool) (*domain.Message, error) {
	body, err := domain.ValidateBody(body)
	if err != nil {
		return nil, err
	}
	t, err := s.ticket(ctx, id)
	if err != nil {
		return nil, err
	}
	now := s.now()
	t.StaffReplied(internal, now)
	m := &domain.Message{ID: ids.New("msg"), TicketID: t.ID, AuthorKind: domain.AuthorStaff, AuthorID: p.StaffID,
		AuthorName: domain.StaffDisplayName, Body: body, Internal: internal, CreatedAt: now}
	var out []contracts.Notification
	if !internal {
		n := s.email(t, "ticket_reply", s.ticketData(t, body, false))
		n.InApp = &contracts.InApp{Kind: "support.reply", Title: fmt.Sprintf("New reply on #%d", t.Number),
			Body: domain.Excerpt(body, 140), Link: s.ticketPath(t)}
		out = append(out, n)
	}
	if err := s.repo.AddMessage(ctx, t, m, out); err != nil {
		return nil, err
	}
	return m, nil
}

type TicketUpdate struct {
	Status   *domain.Status
	Priority *domain.Priority
}

func (s *Service) StaffUpdate(ctx context.Context, id string, u TicketUpdate) (*domain.Ticket, error) {
	t, err := s.ticket(ctx, id)
	if err != nil {
		return nil, err
	}
	now := s.now()
	wasResolved := t.Status == domain.StatusResolved
	if u.Status != nil {
		if err := t.SetStatus(*u.Status, now); err != nil {
			return nil, err
		}
	}
	if u.Priority != nil {
		if err := t.SetPriority(*u.Priority, now); err != nil {
			return nil, err
		}
	}
	var out []contracts.Notification
	if !wasResolved && t.Status == domain.StatusResolved {
		n := s.email(t, "ticket_resolved", s.ticketData(t, "", true))
		n.InApp = &contracts.InApp{Kind: "support.resolved", Title: fmt.Sprintf("#%d was resolved", t.Number),
			Body: "How did we do? Rate the help you got.", Link: s.ticketPath(t) + "#rate"}
		out = append(out, n)
	}
	if err := s.repo.SaveTicket(ctx, t, out); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) Summary(ctx context.Context) (map[domain.Status]int, error) {
	return s.repo.CountByStatus(ctx)
}

func (s *Service) ListFeedback(ctx context.Context, before string, limit int) ([]*domain.Feedback, error) {
	return s.repo.ListFeedback(ctx, before, limit)
}

// ---------------------------------------------------------------- helpers

func (s *Service) ticket(ctx context.Context, id string) (*domain.Ticket, error) {
	t, err := s.repo.GetTicket(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, errTicketNotFound
	}
	return t, err
}

func (s *Service) ticketPath(t *domain.Ticket) string { return "/dashboard/support/" + t.ID }

func (s *Service) ticketData(t *domain.Ticket, body string, rate bool) map[string]any {
	url := s.cfg.AppBaseURL + s.ticketPath(t)
	if rate {
		url += "#rate"
	}
	return map[string]any{
		"Ref": fmt.Sprintf("#%d", t.Number), "Subject": t.Subject, "BusinessName": t.TenantName,
		"Excerpt": domain.Excerpt(body, excerptLen), "TicketURL": url,
	}
}

// email is a notification to the ticket's requester.
func (s *Service) email(t *domain.Ticket, template string, data map[string]any) contracts.Notification {
	return contracts.Notification{ID: ids.New("ntf"), TenantID: t.TenantID, UserID: t.UserID,
		Email: &contracts.Email{Template: template, Category: contracts.EmailOperations, To: t.RequesterEmail, Data: data}}
}

// staffAlert emails SUPPORT_NOTIFY_EMAIL, if set.
func (s *Service) staffAlert(t *domain.Ticket, body, kind string) []contracts.Notification {
	if s.cfg.StaffEmail == "" {
		return nil
	}
	d := s.ticketData(t, body, false)
	d["Kind"], d["RequesterEmail"] = kind, t.RequesterEmail
	d["TicketURL"] = s.cfg.AppBaseURL + "/admin/support/" + t.ID
	return []contracts.Notification{{ID: ids.New("ntf"),
		Email: &contracts.Email{Template: "support_staff_alert", Category: contracts.EmailOperations, To: s.cfg.StaffEmail, Data: d}}}
}

func (s *Service) limit(ctx context.Context, key string, n int, window time.Duration) error {
	ok, err := s.limiter.Allow(ctx, key, n, window)
	if err != nil {
		slog.WarnContext(ctx, "rate limiter unavailable", "err", err)
		return nil
	}
	if !ok {
		return errs.RateLimited("too many messages; please wait a few minutes and try again")
	}
	return nil
}
