package app

import (
	"context"
	"errors"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/identity/domain"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrEmailTaken    = errors.New("email already registered")
	ErrStaleRefresh  = errors.New("refresh token already rotated")
	ErrIdentityTaken = errors.New("identity linked to another user")
)

type Repository interface {
	UserStats(ctx context.Context, since time.Time) (*Stats, error)
	CreateUser(ctx context.Context, u *domain.User) error
	GetUser(ctx context.Context, id string) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetOwner(ctx context.Context, tenantID string) (*domain.User, error)
	ListUsers(ctx context.Context, f UserFilter) ([]*domain.User, error)
	SaveUser(ctx context.Context, u *domain.User) error

	CreateSession(ctx context.Context, s *domain.Session) error
	// GetActiveSession returns the session and its user if the session is
	// neither revoked nor expired.
	GetActiveSession(ctx context.Context, id string, now time.Time) (*domain.Session, *domain.User, error)
	GetSessionByRefresh(ctx context.Context, hash []byte) (*domain.Session, error)
	GetSessionByPrevRefresh(ctx context.Context, hash []byte) (*domain.Session, error)
	// RotateRefresh swaps the refresh hash only if it is still oldHash.
	RotateRefresh(ctx context.Context, id string, oldHash, newHash []byte, now time.Time, ip, userAgent string) error
	ListSessions(ctx context.Context, userID string, now time.Time) ([]*domain.Session, error)
	RevokeSession(ctx context.Context, userID, id string, now time.Time) error
	RevokeUserSessions(ctx context.Context, userID, exceptID string, now time.Time) error

	CreateToken(ctx context.Context, t *domain.Token) error
	GetToken(ctx context.Context, id string) (*domain.Token, error)
	GetTokenByHash(ctx context.Context, purpose domain.TokenPurpose, hash []byte) (*domain.Token, error)
	SaveToken(ctx context.Context, t *domain.Token) error
	InvalidateTokens(ctx context.Context, userID string, purpose domain.TokenPurpose, now time.Time) error

	CreateTrustedDevice(ctx context.Context, d *domain.TrustedDevice) error
	IsTrustedDevice(ctx context.Context, userID string, hash []byte, now time.Time) (bool, error)
	DeleteTrustedDevices(ctx context.Context, userID string) error

	GetIdentity(ctx context.Context, provider, subject string) (*domain.OAuthIdentity, error)
	ListIdentities(ctx context.Context, userID string) ([]*domain.OAuthIdentity, error)
	CreateIdentity(ctx context.Context, i *domain.OAuthIdentity) error
	DeleteIdentity(ctx context.Context, userID, provider string) error

	AddEvent(ctx context.Context, e *domain.SecurityEvent) error
	ListEvents(ctx context.Context, userID string, limit int) ([]*domain.SecurityEvent, error)
}

// UserFilter narrows the admin user search; zero fields match everything.
type UserFilter struct {
	TenantID   string
	EmailQuery string // substring, case-insensitive
	Limit      int
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

// TokenIssuer issues and verifies short-lived access tokens bound to a session.
type TokenIssuer interface {
	Issue(userID, tenantID, sessionID string) (token string, expiresAt time.Time, err error)
	Verify(token string) (userID, tenantID, sessionID string, err error)
}

type Mailer interface {
	Enqueue(ctx context.Context, e contracts.Email) error
}

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
	Reset(ctx context.Context, keys ...string) error
}

// Tenants is the tenant module, seen from identity.
type Tenants interface {
	CreateTenant(ctx context.Context, name string) (id string, err error)
	TenantName(ctx context.Context, id string) (string, error)
	TenantSuspended(ctx context.Context, id string) (bool, error)
}

type GoogleProfile struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// Google is the OAuth 2.0 / OpenID Connect provider port.
type Google interface {
	AuthCodeURL(state, nonce, codeChallenge, redirectURI string) string
	Exchange(ctx context.Context, code, codeVerifier, redirectURI, nonce string) (GoogleProfile, error)
}
