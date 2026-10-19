// Package domain holds the identity module's entities and rules: users,
// sessions, single-use tokens (verification, reset, sign-in codes) and
// trusted devices.
package domain

import (
	"net/mail"
	"strings"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
)

type Role string

const RoleOwner Role = "owner"

type User struct {
	ID              string
	TenantID        string
	Email           string
	PasswordHash    string // empty for Google-only accounts
	Role            Role
	EmailVerifiedAt *time.Time
	TwoStepEnabled  bool
	LastLoginAt     *time.Time
	DisabledAt      *time.Time // set by a platform admin; blocks sign-in
	DisabledReason  string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (u *User) Verified() bool    { return u.EmailVerifiedAt != nil }
func (u *User) HasPassword() bool { return u.PasswordHash != "" }
func (u *User) Disabled() bool    { return u.DisabledAt != nil }

// ErrDisabled is returned wherever a disabled user would otherwise sign in.
var ErrDisabled = errs.Forbidden("account_disabled", "this account has been disabled; contact support")

func (u *User) Disable(reason string, at time.Time) error {
	if u.Disabled() {
		return errs.Conflict("already_disabled", "user is already disabled")
	}
	u.DisabledAt, u.DisabledReason, u.UpdatedAt = &at, reason, at
	return nil
}

func (u *User) Enable(at time.Time) error {
	if !u.Disabled() {
		return errs.Conflict("not_disabled", "user is not disabled")
	}
	u.DisabledAt, u.DisabledReason, u.UpdatedAt = nil, "", at
	return nil
}

func NormalizeEmail(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s {
		return "", errs.Invalid("invalid_email", "email address is invalid")
	}
	return s, nil
}

func ValidatePassword(pw string) error {
	if len(pw) < 10 || len(pw) > 72 { // bcrypt only uses the first 72 bytes
		return errs.Invalid("weak_password", "password must be 10-72 characters")
	}
	return nil
}

// AuthMethod records how a session was established.
type AuthMethod string

const (
	MethodPassword AuthMethod = "password"       // password, no code (trusted device or two-step off)
	MethodTwoStep  AuthMethod = "password+code"  // password + email code
	MethodGoogle   AuthMethod = "google"         //
	MethodReset    AuthMethod = "password_reset" // signed in by completing a reset
	MethodSignup   AuthMethod = "signup"         //
)

// Session is a revocable dashboard sign-in. The browser holds a rotating
// refresh token (cookie); access JWTs carry the session ID.
type Session struct {
	ID              string
	UserID          string
	TenantID        string
	RefreshHash     []byte
	PrevRefreshHash []byte
	AuthMethod      AuthMethod
	UserAgent       string
	IP              string
	CreatedAt       time.Time
	LastSeenAt      time.Time
	ExpiresAt       time.Time
	RevokedAt       *time.Time
}

func (s *Session) Active(now time.Time) bool { return s.RevokedAt == nil && now.Before(s.ExpiresAt) }

// TokenPurpose distinguishes single-use tokens.
type TokenPurpose string

const (
	PurposeVerifyEmail    TokenPurpose = "verify_email"
	PurposeResetPassword  TokenPurpose = "reset_password"
	PurposeLoginChallenge TokenPurpose = "login_challenge"
	PurposeGoogleSignup   TokenPurpose = "google_signup"
	PurposeOAuthState     TokenPurpose = "oauth_state"
)

// Token lifetimes and limits.
const (
	VerifyEmailTTL     = 24 * time.Hour
	ResetPasswordTTL   = 30 * time.Minute
	LoginChallengeTTL  = 10 * time.Minute
	GoogleSignupTTL    = 30 * time.Minute
	OAuthStateTTL      = 10 * time.Minute
	TrustedDeviceTTL   = 30 * 24 * time.Hour
	MaxCodeAttempts    = 5
	MaxCodeSends       = 5
	CodeResendCooldown = 30 * time.Second
)

// Token is a single-use secret: a link token, a sign-in code challenge or
// OAuth state. Only hashes are stored.
type Token struct {
	ID         string
	UserID     string
	Purpose    TokenPurpose
	TokenHash  []byte
	CodeHash   []byte
	Attempts   int
	Sends      int
	LastSentAt time.Time
	Data       map[string]string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}

// Usable reports whether the token can still be redeemed.
func (t *Token) Usable(now time.Time) error {
	switch {
	case t.ConsumedAt != nil:
		return errs.Invalid("token_used", "this link or code has already been used")
	case !now.Before(t.ExpiresAt):
		return errs.Invalid("token_expired", "this link or code has expired")
	case t.Attempts >= MaxCodeAttempts:
		return errs.Invalid("too_many_attempts", "too many incorrect codes; sign in again to get a new one")
	}
	return nil
}

// CanResend enforces the resend cooldown and cap for sign-in codes.
func (t *Token) CanResend(now time.Time) error {
	if err := t.Usable(now); err != nil {
		return err
	}
	if t.Sends >= MaxCodeSends {
		return errs.Invalid("too_many_resends", "too many codes sent; sign in again")
	}
	if now.Sub(t.LastSentAt) < CodeResendCooldown {
		return errs.Invalid("resend_too_soon", "wait a few seconds before requesting another code")
	}
	return nil
}

type TrustedDevice struct {
	ID        string
	UserID    string
	TokenHash []byte
	UserAgent string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// OAuthIdentity links an external provider account to a user.
type OAuthIdentity struct {
	Provider  string
	Subject   string
	UserID    string
	Email     string
	CreatedAt time.Time
}

const ProviderGoogle = "google"

// SecurityEvent is an audit-log entry shown in Settings → Security.
type SecurityEvent struct {
	ID        int64
	UserID    string
	TenantID  string
	Type      string
	IP        string
	UserAgent string
	Data      map[string]string
	CreatedAt time.Time
}

const (
	EventSignup          = "signup"
	EventLogin           = "login"
	EventLoginFailed     = "login_failed"
	EventCodeSent        = "login_code_sent"
	EventCodeFailed      = "login_code_failed"
	EventLogout          = "logout"
	EventSessionRevoked  = "session_revoked"
	EventRefreshReuse    = "refresh_token_reuse"
	EventEmailVerified   = "email_verified"
	EventPasswordReset   = "password_reset"
	EventPasswordChanged = "password_changed"
	EventTwoStepEnabled  = "two_step_enabled"
	EventTwoStepDisabled = "two_step_disabled"
	EventGoogleLinked    = "google_linked"
	EventGoogleUnlinked  = "google_unlinked"
	EventAccountDisabled = "account_disabled" // by a platform admin
	EventAccountEnabled  = "account_enabled"  //
)

// MaskEmail renders j•••@gmail.com for "we sent a code to" messages.
func MaskEmail(email string) string {
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" {
		return email
	}
	return local[:1] + strings.Repeat("•", max(1, min(len(local)-1, 4))) + "@" + domain
}
