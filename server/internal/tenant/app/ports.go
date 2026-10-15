package app

import (
	"context"
	"errors"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/tenant/domain"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrEmailTaken = errors.New("email already registered")
)

// Repository is the tenant module's persistence port. Implementations
// encrypt WebhookSecret at rest.
type Repository interface {
	CreateTenantWithOwner(ctx context.Context, t *domain.Tenant, u *domain.User) error
	GetTenant(ctx context.Context, id string) (*domain.Tenant, error)
	UpdateWebhook(ctx context.Context, tenantID, url, secret string) error

	GetUser(ctx context.Context, id string) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)

	CreateAPIKey(ctx context.Context, k *domain.APIKey) error
	ListAPIKeys(ctx context.Context, tenantID string) ([]*domain.APIKey, error)
	GetAPIKeyByHash(ctx context.Context, hash []byte) (*domain.APIKey, error)
	RevokeAPIKey(ctx context.Context, tenantID, id string, at time.Time) error
	TouchAPIKey(ctx context.Context, id string, at time.Time) error

	CreateChannel(ctx context.Context, c *domain.Channel) error
	ListChannels(ctx context.Context, tenantID string) ([]*domain.Channel, error)
	GetChannel(ctx context.Context, tenantID, id string) (*domain.Channel, error)
	DeactivateChannel(ctx context.Context, tenantID, id string) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

// TokenIssuer issues and verifies dashboard session tokens.
type TokenIssuer interface {
	Issue(userID, tenantID string) (token string, expiresAt time.Time, err error)
	Verify(token string) (userID, tenantID string, err error)
}
