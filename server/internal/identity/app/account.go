package app

import (
	"context"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/identity/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
)

// ---------------------------------------------------------------- email verification

func (s *Service) sendVerification(ctx context.Context, u *domain.User, businessName string) error {
	now := s.now()
	if err := s.repo.InvalidateTokens(ctx, u.ID, domain.PurposeVerifyEmail, now); err != nil {
		return err
	}
	raw := ids.SecretBase32(32)
	if err := s.repo.CreateToken(ctx, &domain.Token{
		ID: ids.New("tok"), UserID: u.ID, Purpose: domain.PurposeVerifyEmail, TokenHash: hashToken(raw),
		Sends: 1, LastSentAt: now, CreatedAt: now, ExpiresAt: now.Add(domain.VerifyEmailTTL),
	}); err != nil {
		return err
	}
	s.mail(ctx, u.Email, "verify_email", contracts.EmailSecurity, map[string]any{
		"BusinessName": businessName,
		"VerifyURL":    s.cfg.AppBaseURL + "/auth/verify-email?token=" + raw,
		"ExpiresHours": int(domain.VerifyEmailTTL / time.Hour),
	}, "")
	return nil
}

func (s *Service) ResendVerification(ctx context.Context, p auth.Principal) error {
	if err := s.limit(ctx, "verify:user:"+p.UserID, 5, time.Hour); err != nil {
		return err
	}
	u, err := s.repo.GetUser(ctx, p.UserID)
	if err != nil {
		return err
	}
	if u.Verified() {
		return errs.Conflict("already_verified", "your email is already verified")
	}
	name, err := s.tenants.TenantName(ctx, u.TenantID)
	if err != nil {
		return err
	}
	return s.sendVerification(ctx, u, name)
}

// VerifyEmail redeems a verification link and sends the welcome email.
func (s *Service) VerifyEmail(ctx context.Context, raw string, c Client) (*domain.User, error) {
	if err := s.limit(ctx, "verify:ip:"+c.IP, 30, 15*time.Minute); err != nil {
		return nil, err
	}
	t, err := s.repo.GetTokenByHash(ctx, domain.PurposeVerifyEmail, hashToken(raw))
	if notFound(err) {
		return nil, errs.Invalid("invalid_token", "this verification link is invalid; request a new one")
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	if err := t.Usable(now); err != nil {
		return nil, err
	}
	t.ConsumedAt = &now
	if err := s.repo.SaveToken(ctx, t); err != nil {
		return nil, err
	}
	u, err := s.repo.GetUser(ctx, t.UserID)
	if err != nil {
		return nil, err
	}
	if !u.Verified() {
		u.EmailVerifiedAt, u.UpdatedAt = &now, now
		if err := s.repo.SaveUser(ctx, u); err != nil {
			return nil, err
		}
		s.event(ctx, u, domain.EventEmailVerified, c, nil)
		s.sendWelcome(ctx, u)
	}
	return u, nil
}

func (s *Service) sendWelcome(ctx context.Context, u *domain.User) {
	name, err := s.tenants.TenantName(ctx, u.TenantID)
	if err != nil {
		return
	}
	base := s.cfg.AppBaseURL + "/dashboard"
	steps := []map[string]any{
		{"Label": "Verify your email", "Done": true, "URL": base},
		{"Label": "Add a channel (bank account, PayBill or Till)", "Done": false, "URL": base + "/channels"},
		{"Label": "Create an API key for your backend", "Done": false, "URL": base + "/api-keys"},
		{"Label": "Set your webhook URL", "Done": false, "URL": base + "/webhooks"},
		{"Label": "Receive your first payment", "Done": false, "URL": base + "/payments"},
	}
	s.mail(ctx, u.Email, "welcome", contracts.EmailLifecycle, map[string]any{
		"BusinessName": name, "Steps": steps, "DashboardURL": base,
	}, "welcome:"+u.ID)
}

// ---------------------------------------------------------------- password recovery

// ForgotPassword emails a reset link if the account exists. It never reveals
// whether it does.
func (s *Service) ForgotPassword(ctx context.Context, email string, c Client) error {
	email = stringsLowerTrim(email)
	if err := s.limit(ctx, "forgot:ip:"+c.IP, 10, time.Hour); err != nil {
		return err
	}
	if ok, _ := s.limiter.Allow(ctx, "forgot:email:"+email, 3, time.Hour); !ok {
		return nil // silently drop; same response as success
	}
	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil || u.Disabled() {
		return nil
	}
	now := s.now()
	if err := s.repo.InvalidateTokens(ctx, u.ID, domain.PurposeResetPassword, now); err != nil {
		return err
	}
	raw := ids.SecretBase32(32)
	if err := s.repo.CreateToken(ctx, &domain.Token{
		ID: ids.New("tok"), UserID: u.ID, Purpose: domain.PurposeResetPassword, TokenHash: hashToken(raw),
		Sends: 1, LastSentAt: now, CreatedAt: now, ExpiresAt: now.Add(domain.ResetPasswordTTL),
	}); err != nil {
		return err
	}
	s.mail(ctx, u.Email, "password_reset", contracts.EmailSecurity, map[string]any{
		"ResetURL":       s.cfg.AppBaseURL + "/auth/reset?token=" + raw,
		"ExpiresMinutes": int(domain.ResetPasswordTTL / time.Minute),
	}, "")
	return nil
}

// ResetPassword sets a new password from a reset link, signs out every
// session and trusted device, and signs the user in.
func (s *Service) ResetPassword(ctx context.Context, raw, newPassword string, c Client) (*Auth, error) {
	if err := s.limit(ctx, "reset:ip:"+c.IP, 20, time.Hour); err != nil {
		return nil, err
	}
	if err := domain.ValidatePassword(newPassword); err != nil {
		return nil, err
	}
	t, err := s.repo.GetTokenByHash(ctx, domain.PurposeResetPassword, hashToken(raw))
	if notFound(err) {
		return nil, errs.Invalid("invalid_token", "this reset link is invalid; request a new one")
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	if err := t.Usable(now); err != nil {
		return nil, err
	}
	t.ConsumedAt = &now
	if err := s.repo.SaveToken(ctx, t); err != nil {
		return nil, err
	}
	u, err := s.repo.GetUser(ctx, t.UserID)
	if err != nil {
		return nil, err
	}
	if u.Disabled() {
		return nil, domain.ErrDisabled
	}
	if u.PasswordHash, err = s.hasher.Hash(newPassword); err != nil {
		return nil, err
	}
	if !u.Verified() { // they just proved they control the inbox
		u.EmailVerifiedAt = &now
	}
	u.UpdatedAt = now
	if err := s.repo.SaveUser(ctx, u); err != nil {
		return nil, err
	}
	if err := s.signOutEverywhere(ctx, u, ""); err != nil {
		return nil, err
	}
	s.event(ctx, u, domain.EventPasswordReset, c, nil)
	s.mail(ctx, u.Email, "password_changed", contracts.EmailSecurity, s.deviceData(c), "")
	return s.issue(ctx, u, domain.MethodReset, c)
}

func (s *Service) signOutEverywhere(ctx context.Context, u *domain.User, exceptSession string) error {
	if err := s.repo.RevokeUserSessions(ctx, u.ID, exceptSession, s.now()); err != nil {
		return err
	}
	return s.repo.DeleteTrustedDevices(ctx, u.ID)
}

// ChangePassword updates the password for a signed-in user (or sets the
// first one for a Google-only account) and signs out other sessions.
func (s *Service) ChangePassword(ctx context.Context, p auth.Principal, current, next string, c Client) error {
	if err := s.limit(ctx, "pwchange:user:"+p.UserID, 10, time.Hour); err != nil {
		return err
	}
	if err := domain.ValidatePassword(next); err != nil {
		return err
	}
	u, err := s.repo.GetUser(ctx, p.UserID)
	if err != nil {
		return err
	}
	if u.HasPassword() && !s.hasher.Compare(u.PasswordHash, current) {
		return errs.Invalid("wrong_password", "current password is incorrect")
	}
	if u.PasswordHash, err = s.hasher.Hash(next); err != nil {
		return err
	}
	u.UpdatedAt = s.now()
	if err := s.repo.SaveUser(ctx, u); err != nil {
		return err
	}
	if err := s.signOutEverywhere(ctx, u, p.SessionID); err != nil {
		return err
	}
	s.event(ctx, u, domain.EventPasswordChanged, c, nil)
	s.mail(ctx, u.Email, "password_changed", contracts.EmailSecurity, s.deviceData(c), "")
	return nil
}

// SetTwoStep turns email codes on or off. Turning them off requires the
// password (for accounts that have one) and sends an alert.
func (s *Service) SetTwoStep(ctx context.Context, p auth.Principal, enabled bool, password string, c Client) error {
	u, err := s.repo.GetUser(ctx, p.UserID)
	if err != nil {
		return err
	}
	if u.TwoStepEnabled == enabled {
		return nil
	}
	if !enabled && u.HasPassword() && !s.hasher.Compare(u.PasswordHash, password) {
		return errs.Invalid("wrong_password", "password is incorrect")
	}
	u.TwoStepEnabled, u.UpdatedAt = enabled, s.now()
	if err := s.repo.SaveUser(ctx, u); err != nil {
		return err
	}
	if enabled {
		s.event(ctx, u, domain.EventTwoStepEnabled, c, nil)
		return nil
	}
	s.event(ctx, u, domain.EventTwoStepDisabled, c, nil)
	s.mail(ctx, u.Email, "two_step_disabled", contracts.EmailSecurity, s.deviceData(c), "")
	return nil
}

// ---------------------------------------------------------------- profile

type Profile struct {
	User       *domain.User
	TenantName string
	Identities []*domain.OAuthIdentity
}

func (s *Service) Me(ctx context.Context, p auth.Principal) (*Profile, error) {
	u, err := s.repo.GetUser(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	name, err := s.tenants.TenantName(ctx, u.TenantID)
	if err != nil {
		return nil, err
	}
	idents, err := s.repo.ListIdentities(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	return &Profile{User: u, TenantName: name, Identities: idents}, nil
}

func (s *Service) SecurityEvents(ctx context.Context, p auth.Principal, limit int) ([]*domain.SecurityEvent, error) {
	return s.repo.ListEvents(ctx, p.UserID, limit)
}

func (s *Service) GoogleEnabled() bool { return s.cfg.GoogleEnabled }
