package app

import (
	"context"
	"errors"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/tenant/domain"
)

var ErrNotFound = errors.New("not found")

// Repository is the tenant module's persistence port. Implementations
// encrypt WebhookSecret at rest.
type Repository interface {
	CreateTenant(ctx context.Context, t *domain.Tenant) error
	GetTenant(ctx context.Context, id string) (*domain.Tenant, error)
	UpdateWebhook(ctx context.Context, tenantID, url, secret string) error
	ListTenantsCreatedBetween(ctx context.Context, from, to time.Time) ([]*domain.Tenant, error)

	GetNotificationSettings(ctx context.Context, tenantID string) (domain.NotificationSettings, error)
	SaveNotificationSettings(ctx context.Context, s domain.NotificationSettings) error

	HasActiveChannel(ctx context.Context, tenantID string) (bool, error)
	HasActiveAPIKey(ctx context.Context, tenantID string) (bool, error)

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

// Owners is the identity module, seen from tenant (owner contact details).
type Owners interface {
	OwnerContact(ctx context.Context, tenantID string) (email string, verified bool, err error)
}

// PaymentStats is the payment module, seen from tenant.
type PaymentStats interface {
	HasSucceededPayment(ctx context.Context, tenantID string) (bool, error)
}

type Mailer interface {
	Enqueue(ctx context.Context, e contracts.Email) error
}

// Previewer renders an email template for the notification-settings preview.
type Previewer interface {
	Preview(template string, data map[string]any) (Rendered, error)
}

type Rendered struct {
	Subject string
	HTML    string
	Text    string
}
