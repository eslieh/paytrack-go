// Package app holds the inbox module's use cases. Deliver fans a
// contracts.Notification out to email (through the mail module) and to the
// dashboard bell; dashboard users list and mark their notifications.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/inbox/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
)

var ErrNotFound = errors.New("not found")

type Repository interface {
	// Insert ignores a notification whose ID already exists.
	Insert(ctx context.Context, n *domain.Notification) error
	List(ctx context.Context, tenantID, userID string, before time.Time, limit int) ([]*domain.Notification, error)
	CountUnread(ctx context.Context, tenantID, userID string) (int, error)
	MarkRead(ctx context.Context, tenantID, userID, id string, at time.Time) error
	MarkAllRead(ctx context.Context, tenantID, userID string, at time.Time) error
}

type Mailer interface {
	Enqueue(ctx context.Context, e contracts.Email) error
}

type Service struct {
	repo   Repository
	mailer Mailer
	now    func() time.Time
}

func NewService(repo Repository, mailer Mailer) *Service {
	return &Service{repo: repo, mailer: mailer, now: time.Now}
}

// Deliver sends a notification's email and stores its in-app part. Both are
// idempotent on the notification ID, so redelivery is harmless.
func (s *Service) Deliver(ctx context.Context, n contracts.Notification) error {
	if n.ID == "" {
		return errs.Invalid("invalid_notification", "notification id is required")
	}
	if n.Email != nil {
		e := *n.Email
		if e.DedupeKey == "" {
			e.DedupeKey = "ntf:" + n.ID
		}
		if err := s.mailer.Enqueue(ctx, e); err != nil {
			return err
		}
	}
	if n.InApp != nil && n.TenantID != "" {
		return s.repo.Insert(ctx, &domain.Notification{
			ID: n.ID, TenantID: n.TenantID, UserID: n.UserID, Kind: n.InApp.Kind,
			Title: domain.Clip(n.InApp.Title, domain.MaxTitle), Body: domain.Clip(n.InApp.Body, domain.MaxBody),
			Link: n.InApp.Link, CreatedAt: s.now(),
		})
	}
	return nil
}

func (s *Service) List(ctx context.Context, p auth.Principal, before time.Time, limit int) ([]*domain.Notification, error) {
	return s.repo.List(ctx, p.TenantID, p.UserID, before, limit)
}

func (s *Service) UnreadCount(ctx context.Context, p auth.Principal) (int, error) {
	return s.repo.CountUnread(ctx, p.TenantID, p.UserID)
}

func (s *Service) MarkRead(ctx context.Context, p auth.Principal, id string) error {
	err := s.repo.MarkRead(ctx, p.TenantID, p.UserID, id, s.now())
	if errors.Is(err, ErrNotFound) {
		return errs.NotFound("notification_not_found", "notification not found")
	}
	return err
}

func (s *Service) MarkAllRead(ctx context.Context, p auth.Principal) error {
	return s.repo.MarkAllRead(ctx, p.TenantID, p.UserID, s.now())
}
