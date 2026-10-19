package app

import (
	"context"

	"github.com/eslieh/paytrack-go/server/internal/identity/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
)

// Platform-admin use cases. The admin module calls these through its own
// port after authorizing and auditing the staff member.

func (s *Service) user(ctx context.Context, id string) (*domain.User, error) {
	u, err := s.repo.GetUser(ctx, id)
	if notFound(err) {
		return nil, errs.NotFound("user_not_found", "user not found")
	}
	return u, err
}

// DisableUser blocks sign-in and ends every session and trusted device.
func (s *Service) DisableUser(ctx context.Context, id, reason string, c Client) error {
	u, err := s.user(ctx, id)
	if err != nil {
		return err
	}
	if err := u.Disable(reason, s.now()); err != nil {
		return err
	}
	if err := s.repo.SaveUser(ctx, u); err != nil {
		return err
	}
	if err := s.signOutEverywhere(ctx, u, ""); err != nil {
		return err
	}
	s.event(ctx, u, domain.EventAccountDisabled, c, map[string]string{"reason": reason})
	return nil
}

func (s *Service) EnableUser(ctx context.Context, id string, c Client) error {
	u, err := s.user(ctx, id)
	if err != nil {
		return err
	}
	if err := u.Enable(s.now()); err != nil {
		return err
	}
	if err := s.repo.SaveUser(ctx, u); err != nil {
		return err
	}
	s.event(ctx, u, domain.EventAccountEnabled, c, nil)
	return nil
}

// UserContact returns where to notify a user.
func (s *Service) UserContact(ctx context.Context, id string) (email, tenantID string, err error) {
	u, err := s.user(ctx, id)
	if err != nil {
		return "", "", err
	}
	return u.Email, u.TenantID, nil
}
