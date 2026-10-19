// Package app holds the identity module's use cases: sign-up, sign-in with
// email codes, sessions, email verification, password recovery and Google
// sign-in.
package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/identity/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
)

type Config struct {
	AppBaseURL    string // dashboard URL used in email links
	RefreshTTL    time.Duration
	CodeKey       []byte // HMAC key for sign-in codes
	GoogleEnabled bool
}

type Service struct {
	repo    Repository
	hasher  PasswordHasher
	tokens  TokenIssuer
	mailer  Mailer
	limiter RateLimiter
	tenants Tenants
	google  Google
	cfg     Config
	now     func() time.Time

	dummyHash string // keeps Login timing similar whether or not the email exists
}

func NewService(repo Repository, hasher PasswordHasher, tokens TokenIssuer, mailer Mailer, limiter RateLimiter,
	tenants Tenants, google Google, cfg Config) (*Service, error) {
	dummy, err := hasher.Hash("not-a-real-password")
	if err != nil {
		return nil, err
	}
	return &Service{repo: repo, hasher: hasher, tokens: tokens, mailer: mailer, limiter: limiter, tenants: tenants,
		google: google, cfg: cfg, now: time.Now, dummyHash: dummy}, nil
}

// Client describes the caller, for rate limits, sessions and audit.
type Client struct {
	IP        string
	UserAgent string
}

// Auth is the result of a successful sign-in.
type Auth struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	DeviceToken      string // set when this device should be remembered
	User             *domain.User
	TenantName       string
	TenantSuspended  bool
	SessionID        string
}

// ---------------------------------------------------------------- helpers

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func (s *Service) hashCode(challengeID, code string) []byte {
	m := hmac.New(sha256.New, s.cfg.CodeKey)
	m.Write([]byte(challengeID + ":" + code))
	return m.Sum(nil)
}

func newCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func (s *Service) limit(ctx context.Context, key string, n int, window time.Duration) error {
	ok, err := s.limiter.Allow(ctx, key, n, window)
	if err != nil {
		slog.WarnContext(ctx, "rate limiter unavailable", "err", err)
		return nil // fail open: availability of sign-in beats strict limiting
	}
	if !ok {
		return errs.RateLimited("too many attempts; please wait a few minutes and try again")
	}
	return nil
}

func (s *Service) event(ctx context.Context, u *domain.User, typ string, c Client, data map[string]string) {
	if err := s.repo.AddEvent(ctx, &domain.SecurityEvent{
		UserID: u.ID, TenantID: u.TenantID, Type: typ, IP: c.IP, UserAgent: c.UserAgent, Data: data, CreatedAt: s.now(),
	}); err != nil {
		slog.WarnContext(ctx, "audit event failed", "type", typ, "err", err)
	}
}

var eat = time.FixedZone("EAT", 3*60*60)

func (s *Service) when() string { return s.now().In(eat).Format("2 Jan 2006, 15:04 MST") }

func (s *Service) deviceData(c Client) map[string]any {
	return map[string]any{"When": s.when(), "Device": domain.DescribeDevice(c.UserAgent), "IP": c.IP}
}

func (s *Service) mail(ctx context.Context, to, template, category string, data map[string]any, dedupe string) {
	if err := s.mailer.Enqueue(ctx, contracts.Email{
		Template: template, Category: category, To: to, Data: data, DedupeKey: dedupe,
	}); err != nil {
		slog.ErrorContext(ctx, "enqueue email failed", "template", template, "err", err)
	}
}

// issue creates a session and its tokens for u.
func (s *Service) issue(ctx context.Context, u *domain.User, method domain.AuthMethod, c Client) (*Auth, error) {
	if u.Disabled() {
		return nil, domain.ErrDisabled
	}
	now := s.now()
	refresh := ids.SecretBase32(32)
	sess := &domain.Session{
		ID: ids.New("ses"), UserID: u.ID, TenantID: u.TenantID, RefreshHash: hashToken(refresh), AuthMethod: method,
		UserAgent: truncate(c.UserAgent, 300), IP: c.IP, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(s.cfg.RefreshTTL),
	}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return nil, err
	}
	access, exp, err := s.tokens.Issue(u.ID, u.TenantID, sess.ID)
	if err != nil {
		return nil, err
	}
	u.LastLoginAt, u.UpdatedAt = &now, now
	if err := s.repo.SaveUser(ctx, u); err != nil {
		return nil, err
	}
	name, err := s.tenants.TenantName(ctx, u.TenantID)
	if err != nil {
		return nil, err
	}
	suspended, err := s.tenants.TenantSuspended(ctx, u.TenantID)
	if err != nil {
		return nil, err
	}
	s.event(ctx, u, domain.EventLogin, c, map[string]string{"method": string(method), "session_id": sess.ID})
	return &Auth{AccessToken: access, AccessExpiresAt: exp, RefreshToken: refresh, RefreshExpiresAt: sess.ExpiresAt,
		User: u, TenantName: name, TenantSuspended: suspended, SessionID: sess.ID}, nil
}

func (s *Service) trustDevice(ctx context.Context, u *domain.User, c Client) (string, error) {
	raw := ids.SecretBase32(32)
	now := s.now()
	return raw, s.repo.CreateTrustedDevice(ctx, &domain.TrustedDevice{
		ID: ids.New("dev"), UserID: u.ID, TokenHash: hashToken(raw), UserAgent: truncate(c.UserAgent, 300),
		CreatedAt: now, ExpiresAt: now.Add(domain.TrustedDeviceTTL),
	})
}

func (s *Service) isTrusted(ctx context.Context, u *domain.User, deviceToken string) bool {
	if deviceToken == "" {
		return false
	}
	ok, err := s.repo.IsTrustedDevice(ctx, u.ID, hashToken(deviceToken), s.now())
	return err == nil && ok
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func notFound(err error) bool { return errors.Is(err, ErrNotFound) }

// ---------------------------------------------------------------- authentication

// Authenticate implements auth.Authenticator for dashboard access tokens.
// The session is checked on every request so sign-out takes effect at once.
func (s *Service) Authenticate(ctx context.Context, bearer string) (auth.Principal, error) {
	invalid := errs.Unauthorized("invalid_token", "session is invalid or expired")
	userID, tenantID, sessionID, err := s.tokens.Verify(bearer)
	if err != nil {
		return auth.Principal{}, invalid
	}
	sess, u, err := s.repo.GetActiveSession(ctx, sessionID, s.now())
	if notFound(err) || (err == nil && (sess.UserID != userID || u.TenantID != tenantID || u.Disabled())) {
		return auth.Principal{}, invalid
	}
	if err != nil {
		return auth.Principal{}, err
	}
	suspended, err := s.tenants.TenantSuspended(ctx, tenantID)
	if err != nil {
		return auth.Principal{}, err
	}
	return auth.Principal{Kind: auth.KindUser, TenantID: tenantID, UserID: userID, SessionID: sessionID,
		EmailVerified: u.Verified(), Suspended: suspended}, nil
}

// OwnerContact returns a tenant owner's email for notifications.
func (s *Service) OwnerContact(ctx context.Context, tenantID string) (email string, verified bool, err error) {
	u, err := s.repo.GetOwner(ctx, tenantID)
	if err != nil {
		return "", false, err
	}
	return u.Email, u.Verified(), nil
}
