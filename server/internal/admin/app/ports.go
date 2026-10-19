package app

import (
	"context"
	"errors"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/admin/domain"
	"github.com/eslieh/paytrack-go/server/internal/contracts"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrEmailTaken = errors.New("staff email already exists")
)

type Repository interface {
	CreateStaff(ctx context.Context, s *domain.Staff) error
	GetStaff(ctx context.Context, id string) (*domain.Staff, error)
	GetStaffByEmail(ctx context.Context, email string) (*domain.Staff, error)
	SaveStaff(ctx context.Context, s *domain.Staff) error

	CreateChallenge(ctx context.Context, c *domain.Challenge) error
	GetChallenge(ctx context.Context, id string) (*domain.Challenge, error)
	SaveChallenge(ctx context.Context, c *domain.Challenge) error

	CreateSession(ctx context.Context, s *domain.Session) error
	GetSessionByToken(ctx context.Context, hash []byte) (*domain.Session, error)
	TouchSession(ctx context.Context, id string, at time.Time) error
	RevokeSession(ctx context.Context, id string, at time.Time) error
	RevokeStaffSessions(ctx context.Context, staffID string, at time.Time) error

	// AddAudit records an entry and publishes notifications in one transaction.
	AddAudit(ctx context.Context, e *domain.AuditEntry, out []contracts.Notification) error
	ListAudit(ctx context.Context, f AuditFilter) ([]*domain.AuditEntry, error)
}

// AuditFilter narrows the audit log; zero fields match everything.
type AuditFilter struct {
	TargetType string
	TargetID   string
	BeforeID   int64 // pagination cursor
	Limit      int
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

type Mailer interface {
	Enqueue(ctx context.Context, e contracts.Email) error
}

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// Tenants is the tenant module, seen from admin.
type Tenants interface {
	TenantName(ctx context.Context, id string) (string, error)
	SuspendTenant(ctx context.Context, id, reason string) error
	ReinstateTenant(ctx context.Context, id string) error
}

// Users is the identity module, seen from admin.
type Users interface {
	OwnerContact(ctx context.Context, tenantID string) (email string, verified bool, err error)
	UserContact(ctx context.Context, id string) (email, tenantID string, err error)
	DisableUser(ctx context.Context, id, reason string, c Client) error
	EnableUser(ctx context.Context, id string, c Client) error

	SendPasswordReset(ctx context.Context, id string) error
	ResendVerification(ctx context.Context, id string) error
	RevokeAllSessions(ctx context.Context, id string, c Client) error
	ResetTwoStep(ctx context.Context, id string, c Client) error
	UnlinkGoogle(ctx context.Context, id string, c Client) error
	ClearSignInLimits(ctx context.Context, id string) error
	// ChangeEmail returns the previous address so it can be warned.
	ChangeEmail(ctx context.Context, id, email string, c Client) (oldEmail string, err error)
}
