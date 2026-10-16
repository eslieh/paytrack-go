package app

import (
	"context"
	"crypto/hmac"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/identity/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
)

type RegisterCmd struct {
	BusinessName string
	Email        string
	Password     string
}

// Register creates a tenant and its owner, emails a verification link and
// signs the owner in (unverified accounts can explore but not transact).
func (s *Service) Register(ctx context.Context, cmd RegisterCmd, c Client) (*Auth, error) {
	if err := s.limit(ctx, "signup:ip:"+c.IP, 10, time.Hour); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(cmd.BusinessName)
	if name == "" || len(name) > 100 {
		return nil, errs.Invalid("invalid_business_name", "business_name must be 1-100 characters")
	}
	email, err := domain.NormalizeEmail(cmd.Email)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePassword(cmd.Password); err != nil {
		return nil, err
	}
	taken := errs.Conflict("email_taken", "an account with this email already exists")
	if _, err := s.repo.GetUserByEmail(ctx, email); err == nil {
		return nil, taken
	} else if !notFound(err) {
		return nil, err
	}
	hash, err := s.hasher.Hash(cmd.Password)
	if err != nil {
		return nil, err
	}
	u, err := s.createOwner(ctx, name, email, hash, nil)
	if errors.Is(err, ErrEmailTaken) {
		return nil, taken
	}
	if err != nil {
		return nil, err
	}
	s.event(ctx, u, domain.EventSignup, c, map[string]string{"method": "password"})
	if err := s.sendVerification(ctx, u, name); err != nil {
		return nil, err
	}
	return s.issue(ctx, u, domain.MethodSignup, c)
}

func (s *Service) createOwner(ctx context.Context, businessName, email, passwordHash string, verifiedAt *time.Time) (*domain.User, error) {
	tenantID, err := s.tenants.CreateTenant(ctx, businessName)
	if err != nil {
		return nil, err
	}
	now := s.now()
	u := &domain.User{
		ID: ids.New("usr"), TenantID: tenantID, Email: email, PasswordHash: passwordHash, Role: domain.RoleOwner,
		EmailVerifiedAt: verifiedAt, TwoStepEnabled: true, CreatedAt: now, UpdatedAt: now,
	}
	// A concurrent sign-up with the same email can still lose here; the
	// tenant row it created is then unused (harmless).
	return u, s.repo.CreateUser(ctx, u)
}

// Challenge asks the client for the emailed sign-in code.
type Challenge struct {
	ID          string
	MaskedEmail string
	ExpiresAt   time.Time
}

// LoginResult is either a signed-in session or a code challenge.
type LoginResult struct {
	Auth      *Auth
	Challenge *Challenge
}

func (s *Service) Login(ctx context.Context, email, password string, c Client, deviceToken string) (*LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := s.limit(ctx, "login:ip:"+c.IP, 30, 15*time.Minute); err != nil {
		return nil, err
	}
	if err := s.limit(ctx, "login:email:"+email, 10, 15*time.Minute); err != nil {
		return nil, err
	}
	invalid := errs.Unauthorized("invalid_credentials", "email or password is incorrect")
	u, err := s.repo.GetUserByEmail(ctx, email)
	if notFound(err) {
		s.hasher.Compare(s.dummyHash, password)
		return nil, invalid
	}
	if err != nil {
		return nil, err
	}
	if !u.HasPassword() || !s.hasher.Compare(u.PasswordHash, password) {
		if !u.HasPassword() {
			s.hasher.Compare(s.dummyHash, password)
		}
		s.event(ctx, u, domain.EventLoginFailed, c, nil)
		return nil, invalid
	}
	trusted := s.isTrusted(ctx, u, deviceToken)
	if u.TwoStepEnabled && !trusted {
		ch, err := s.startChallenge(ctx, u, c)
		if err != nil {
			return nil, err
		}
		return &LoginResult{Challenge: ch}, nil
	}
	a, err := s.issue(ctx, u, domain.MethodPassword, c)
	if err != nil {
		return nil, err
	}
	if !trusted {
		s.alertNewSignin(ctx, u, c, "your password", a.SessionID)
	}
	return &LoginResult{Auth: a}, nil
}

func (s *Service) alertNewSignin(ctx context.Context, u *domain.User, c Client, method, sessionID string) {
	d := s.deviceData(c)
	d["Method"] = method
	s.mail(ctx, u.Email, "new_signin", contracts.EmailSecurity, d, "new_signin:"+sessionID)
}

func (s *Service) startChallenge(ctx context.Context, u *domain.User, c Client) (*Challenge, error) {
	now := s.now()
	id := ids.New("chl")
	code := newCode()
	t := &domain.Token{
		ID: id, UserID: u.ID, Purpose: domain.PurposeLoginChallenge, TokenHash: hashToken(id),
		CodeHash: s.hashCode(id, code), Sends: 1, LastSentAt: now, CreatedAt: now,
		ExpiresAt: now.Add(domain.LoginChallengeTTL), Data: map[string]string{"ip": c.IP},
	}
	if err := s.repo.CreateToken(ctx, t); err != nil {
		return nil, err
	}
	s.sendCode(ctx, u, t, code, c)
	return &Challenge{ID: id, MaskedEmail: domain.MaskEmail(u.Email), ExpiresAt: t.ExpiresAt}, nil
}

func (s *Service) sendCode(ctx context.Context, u *domain.User, t *domain.Token, code string, c Client) {
	d := s.deviceData(c)
	d["Code"], d["ExpiresMinutes"] = code, int(domain.LoginChallengeTTL/time.Minute)
	s.mail(ctx, u.Email, "login_code", contracts.EmailSecurity, d, "")
	s.event(ctx, u, domain.EventCodeSent, c, map[string]string{"challenge_id": t.ID})
}

func (s *Service) challenge(ctx context.Context, id string) (*domain.Token, *domain.User, error) {
	t, err := s.repo.GetToken(ctx, id)
	if notFound(err) || (err == nil && t.Purpose != domain.PurposeLoginChallenge) {
		return nil, nil, errs.Invalid("invalid_challenge", "sign-in session not found; sign in again")
	}
	if err != nil {
		return nil, nil, err
	}
	u, err := s.repo.GetUser(ctx, t.UserID)
	if err != nil {
		return nil, nil, err
	}
	return t, u, nil
}

// VerifyLogin completes a two-step sign-in.
func (s *Service) VerifyLogin(ctx context.Context, challengeID, code string, remember bool, c Client) (*Auth, error) {
	if err := s.limit(ctx, "code:ip:"+c.IP, 30, 15*time.Minute); err != nil {
		return nil, err
	}
	t, u, err := s.challenge(ctx, challengeID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	if err := t.Usable(now); err != nil {
		return nil, err
	}
	code = strings.TrimSpace(code)
	if !hmac.Equal(t.CodeHash, s.hashCode(t.ID, code)) {
		t.Attempts++
		if err := s.repo.SaveToken(ctx, t); err != nil {
			return nil, err
		}
		s.event(ctx, u, domain.EventCodeFailed, c, nil)
		if left := domain.MaxCodeAttempts - t.Attempts; left > 0 {
			return nil, errs.Invalid("invalid_code", "that code is incorrect; "+plural(left, "attempt")+" left")
		}
		return nil, errs.Invalid("too_many_attempts", "too many incorrect codes; sign in again to get a new one")
	}
	t.ConsumedAt = &now
	if err := s.repo.SaveToken(ctx, t); err != nil {
		return nil, err
	}
	a, err := s.issue(ctx, u, domain.MethodTwoStep, c)
	if err != nil {
		return nil, err
	}
	if remember {
		if a.DeviceToken, err = s.trustDevice(ctx, u, c); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// ResendLoginCode emails a fresh code for an open challenge.
func (s *Service) ResendLoginCode(ctx context.Context, challengeID string, c Client) (*Challenge, error) {
	t, u, err := s.challenge(ctx, challengeID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	if err := t.CanResend(now); err != nil {
		return nil, err
	}
	code := newCode()
	t.CodeHash, t.Sends, t.LastSentAt = s.hashCode(t.ID, code), t.Sends+1, now
	if err := s.repo.SaveToken(ctx, t); err != nil {
		return nil, err
	}
	s.sendCode(ctx, u, t, code, c)
	return &Challenge{ID: t.ID, MaskedEmail: domain.MaskEmail(u.Email), ExpiresAt: t.ExpiresAt}, nil
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

// ---------------------------------------------------------------- sessions

// Refresh rotates the refresh token and issues a new access token. Reuse of
// an already-rotated token revokes the session (likely theft), except within
// a short grace window that covers two tabs refreshing at once.
func (s *Service) Refresh(ctx context.Context, refresh string, c Client) (*Auth, error) {
	invalid := errs.Unauthorized("invalid_refresh", "session expired; sign in again")
	if refresh == "" {
		return nil, invalid
	}
	now := s.now()
	old := hashToken(refresh)
	sess, err := s.repo.GetSessionByRefresh(ctx, old)
	if notFound(err) {
		if prev, perr := s.repo.GetSessionByPrevRefresh(ctx, old); perr == nil && prev.RevokedAt == nil {
			if now.Sub(prev.LastSeenAt) < 20*time.Second {
				return nil, errs.Conflict("refresh_race", "session was refreshed by another tab; retry")
			}
			_ = s.repo.RevokeSession(ctx, prev.UserID, prev.ID, now)
			if u, uerr := s.repo.GetUser(ctx, prev.UserID); uerr == nil {
				s.event(ctx, u, domain.EventRefreshReuse, c, map[string]string{"session_id": prev.ID})
			}
		}
		return nil, invalid
	}
	if err != nil {
		return nil, err
	}
	if !sess.Active(now) {
		return nil, invalid
	}
	next := ids.SecretBase32(32)
	if err := s.repo.RotateRefresh(ctx, sess.ID, old, hashToken(next), now, c.IP, truncate(c.UserAgent, 300)); err != nil {
		if errors.Is(err, ErrStaleRefresh) {
			return nil, errs.Conflict("refresh_race", "session was refreshed by another tab; retry")
		}
		return nil, err
	}
	u, err := s.repo.GetUser(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	access, exp, err := s.tokens.Issue(u.ID, u.TenantID, sess.ID)
	if err != nil {
		return nil, err
	}
	name, err := s.tenants.TenantName(ctx, u.TenantID)
	if err != nil {
		return nil, err
	}
	return &Auth{AccessToken: access, AccessExpiresAt: exp, RefreshToken: next, RefreshExpiresAt: sess.ExpiresAt,
		User: u, TenantName: name, SessionID: sess.ID}, nil
}

// Logout revokes the caller's session, found by access token or refresh cookie.
func (s *Service) Logout(ctx context.Context, p *auth.Principal, refresh string, c Client) error {
	now := s.now()
	var sess *domain.Session
	if p != nil && p.SessionID != "" {
		sess = &domain.Session{ID: p.SessionID, UserID: p.UserID}
	} else if refresh != "" {
		found, err := s.repo.GetSessionByRefresh(ctx, hashToken(refresh))
		if err != nil {
			return nil // nothing to revoke
		}
		sess = found
	} else {
		return nil
	}
	if err := s.repo.RevokeSession(ctx, sess.UserID, sess.ID, now); err != nil && !notFound(err) {
		return err
	}
	if u, err := s.repo.GetUser(ctx, sess.UserID); err == nil {
		s.event(ctx, u, domain.EventLogout, c, map[string]string{"session_id": sess.ID})
	}
	return nil
}

type SessionView struct {
	*domain.Session
	Current bool
	Device  string
}

func (s *Service) ListSessions(ctx context.Context, p auth.Principal) ([]SessionView, error) {
	list, err := s.repo.ListSessions(ctx, p.UserID, s.now())
	if err != nil {
		return nil, err
	}
	out := make([]SessionView, len(list))
	for i, sess := range list {
		out[i] = SessionView{Session: sess, Current: sess.ID == p.SessionID, Device: domain.DescribeDevice(sess.UserAgent)}
	}
	return out, nil
}

func (s *Service) RevokeSession(ctx context.Context, p auth.Principal, id string, c Client) error {
	if err := s.repo.RevokeSession(ctx, p.UserID, id, s.now()); notFound(err) {
		return errs.NotFound("session_not_found", "session not found")
	} else if err != nil {
		return err
	}
	if u, err := s.repo.GetUser(ctx, p.UserID); err == nil {
		s.event(ctx, u, domain.EventSessionRevoked, c, map[string]string{"session_id": id})
	}
	return nil
}

// RevokeOtherSessions signs out every session but the caller's and forgets
// trusted devices.
func (s *Service) RevokeOtherSessions(ctx context.Context, p auth.Principal, c Client) error {
	if err := s.repo.RevokeUserSessions(ctx, p.UserID, p.SessionID, s.now()); err != nil {
		return err
	}
	if err := s.repo.DeleteTrustedDevices(ctx, p.UserID); err != nil {
		return err
	}
	if u, err := s.repo.GetUser(ctx, p.UserID); err == nil {
		s.event(ctx, u, domain.EventSessionRevoked, c, map[string]string{"scope": "others"})
	}
	return nil
}
