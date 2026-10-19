package app

import (
	"context"
	"strings"

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

// ---------------------------------------------------------------- lookups

func (s *Service) ListUsers(ctx context.Context, f UserFilter) ([]*domain.User, error) {
	f.EmailQuery = strings.ToLower(strings.TrimSpace(f.EmailQuery))
	// LIKE wildcards in the query are literal text, not patterns.
	f.EmailQuery = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(f.EmailQuery)
	return s.repo.ListUsers(ctx, f)
}

// UserDetail is everything support needs to help a user.
type UserDetail struct {
	User       *domain.User
	Identities []*domain.OAuthIdentity
	Sessions   []SessionView
	Events     []*domain.SecurityEvent
}

func (s *Service) UserDetail(ctx context.Context, id string) (*UserDetail, error) {
	u, err := s.user(ctx, id)
	if err != nil {
		return nil, err
	}
	d := &UserDetail{User: u}
	if d.Identities, err = s.repo.ListIdentities(ctx, id); err != nil {
		return nil, err
	}
	sessions, err := s.repo.ListSessions(ctx, id, s.now())
	if err != nil {
		return nil, err
	}
	for _, sess := range sessions {
		d.Sessions = append(d.Sessions, SessionView{Session: sess, Device: domain.DescribeDevice(sess.UserAgent)})
	}
	if d.Events, err = s.repo.ListEvents(ctx, id, 30); err != nil {
		return nil, err
	}
	return d, nil
}

// ---------------------------------------------------------------- account recovery

// SendPasswordReset emails the user a reset link. Staff never set passwords.
func (s *Service) SendPasswordReset(ctx context.Context, id string) error {
	u, err := s.user(ctx, id)
	if err != nil {
		return err
	}
	if u.Disabled() {
		return errs.Conflict("user_disabled", "re-enable the user first")
	}
	return s.sendReset(ctx, u)
}

func (s *Service) ResendVerificationFor(ctx context.Context, id string) error {
	u, err := s.user(ctx, id)
	if err != nil {
		return err
	}
	if u.Verified() {
		return errs.Conflict("already_verified", "the email is already verified")
	}
	name, err := s.tenants.TenantName(ctx, u.TenantID)
	if err != nil {
		return err
	}
	return s.sendVerification(ctx, u, name)
}

// RevokeAllSessions signs the user out everywhere and forgets trusted devices.
func (s *Service) RevokeAllSessions(ctx context.Context, id string, c Client) error {
	u, err := s.user(ctx, id)
	if err != nil {
		return err
	}
	if err := s.signOutEverywhere(ctx, u, ""); err != nil {
		return err
	}
	s.event(ctx, u, domain.EventSessionRevoked, c, map[string]string{"by": "support", "scope": "all"})
	return nil
}

// ResetTwoStep turns email codes back on and forgets trusted devices, so
// the next sign-in on any device needs a code.
func (s *Service) ResetTwoStep(ctx context.Context, id string, c Client) error {
	u, err := s.user(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteTrustedDevices(ctx, u.ID); err != nil {
		return err
	}
	if !u.TwoStepEnabled {
		u.TwoStepEnabled, u.UpdatedAt = true, s.now()
		if err := s.repo.SaveUser(ctx, u); err != nil {
			return err
		}
	}
	s.event(ctx, u, domain.EventTwoStepEnabled, c, map[string]string{"by": "support"})
	return nil
}

func (s *Service) UnlinkGoogleFor(ctx context.Context, id string, c Client) error {
	u, err := s.user(ctx, id)
	if err != nil {
		return err
	}
	if !u.HasPassword() {
		return errs.Invalid("password_required", "the user has no password; send a password reset first or they'll be locked out")
	}
	if err := s.repo.DeleteIdentity(ctx, u.ID, domain.ProviderGoogle); notFound(err) {
		return errs.NotFound("not_linked", "Google is not linked")
	} else if err != nil {
		return err
	}
	s.event(ctx, u, domain.EventGoogleUnlinked, c, map[string]string{"by": "support"})
	return nil
}

// ChangeEmail moves the account to a new address: it must be verified
// again, every session ends, and the old address is returned so the caller
// can warn it.
func (s *Service) ChangeEmail(ctx context.Context, id, newEmail string, c Client) (oldEmail string, err error) {
	newEmail, err = domain.NormalizeEmail(newEmail)
	if err != nil {
		return "", err
	}
	u, err := s.user(ctx, id)
	if err != nil {
		return "", err
	}
	if u.Email == newEmail {
		return "", errs.Conflict("same_email", "that is already the user's email")
	}
	if _, err := s.repo.GetUserByEmail(ctx, newEmail); err == nil {
		return "", errs.Conflict("email_taken", "another account uses that email")
	} else if !notFound(err) {
		return "", err
	}
	oldEmail = u.Email
	u.Email, u.EmailVerifiedAt, u.UpdatedAt = newEmail, nil, s.now()
	if err := s.repo.SaveUser(ctx, u); err != nil {
		return "", err
	}
	if err := s.signOutEverywhere(ctx, u, ""); err != nil {
		return "", err
	}
	s.event(ctx, u, domain.EventEmailChanged, c, map[string]string{"by": "support", "from": oldEmail})
	name, err := s.tenants.TenantName(ctx, u.TenantID)
	if err != nil {
		return "", err
	}
	return oldEmail, s.sendVerification(ctx, u, name)
}

// ClearSignInLimits lifts the per-email and per-user rate limits that lock a
// user out after too many attempts.
func (s *Service) ClearSignInLimits(ctx context.Context, id string) error {
	u, err := s.user(ctx, id)
	if err != nil {
		return err
	}
	return s.limiter.Reset(ctx, "login:email:"+u.Email, "forgot:email:"+u.Email, "verify:user:"+u.ID, "pwchange:user:"+u.ID)
}
