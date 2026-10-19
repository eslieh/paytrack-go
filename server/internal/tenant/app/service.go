// Package app holds the tenant module's use cases.
package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
	"github.com/eslieh/paytrack-go/server/internal/tenant/domain"
)

const apiKeyPrefix = "ptk_"

type Service struct {
	repo     Repository
	owners   Owners
	payments PaymentStats
	mailer   Mailer
	preview  Previewer
	appURL   string
	now      func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// Wire connects the ports to other modules. It is separate from NewService
// because those modules depend on this one (identity needs tenants).
func (s *Service) Wire(owners Owners, payments PaymentStats, mailer Mailer, preview Previewer, appURL string) {
	s.owners, s.payments, s.mailer, s.preview, s.appURL = owners, payments, mailer, preview, appURL
}

// CreateTenant creates a business; called by identity during sign-up.
func (s *Service) CreateTenant(ctx context.Context, name string) (string, error) {
	name, err := domain.ValidateName("business_name", name)
	if err != nil {
		return "", err
	}
	t := &domain.Tenant{ID: ids.New("ten"), Name: name, WebhookSecret: newWebhookSecret(), CreatedAt: s.now()}
	return t.ID, s.repo.CreateTenant(ctx, t)
}

func (s *Service) TenantName(ctx context.Context, id string) (string, error) {
	t, err := s.repo.GetTenant(ctx, id)
	if err != nil {
		return "", err
	}
	return t.Name, nil
}

// TenantSuspended is checked on every authenticated request.
func (s *Service) TenantSuspended(ctx context.Context, id string) (bool, error) {
	st, err := s.repo.GetTenantStatus(ctx, id)
	return st == domain.StatusSuspended, err
}

// SuspendTenant is a platform-admin action; see the admin module.
func (s *Service) SuspendTenant(ctx context.Context, id, reason string) error {
	t, err := s.tenant(ctx, id)
	if err != nil {
		return err
	}
	if err := t.Suspend(reason, s.now()); err != nil {
		return err
	}
	return s.repo.SaveTenantStatus(ctx, t)
}

func (s *Service) ReinstateTenant(ctx context.Context, id string) error {
	t, err := s.tenant(ctx, id)
	if err != nil {
		return err
	}
	if err := t.Reinstate(); err != nil {
		return err
	}
	return s.repo.SaveTenantStatus(ctx, t)
}

func (s *Service) tenant(ctx context.Context, id string) (*domain.Tenant, error) {
	t, err := s.repo.GetTenant(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, errs.NotFound("tenant_not_found", "tenant not found")
	}
	return t, err
}

// AuthenticateAPIKey resolves a tenant backend's API key.
func (s *Service) AuthenticateAPIKey(ctx context.Context, bearer string) (auth.Principal, error) {
	sum := sha256.Sum256([]byte(bearer))
	k, err := s.repo.GetAPIKeyByHash(ctx, sum[:])
	if errors.Is(err, ErrNotFound) || (err == nil && k.RevokedAt != nil) {
		return auth.Principal{}, errs.Unauthorized("invalid_api_key", "API key is invalid or revoked")
	}
	if err != nil {
		return auth.Principal{}, err
	}
	if suspended, err := s.TenantSuspended(ctx, k.TenantID); err != nil {
		return auth.Principal{}, err
	} else if suspended {
		return auth.Principal{}, errs.Forbidden("tenant_suspended", "this account is suspended; contact support")
	}
	if k.LastUsedAt == nil || s.now().Sub(*k.LastUsedAt) > time.Minute {
		if err := s.repo.TouchAPIKey(ctx, k.ID, s.now()); err != nil {
			slog.WarnContext(ctx, "touch api key failed", "err", err)
		}
	}
	return auth.Principal{Kind: auth.KindAPIKey, TenantID: k.TenantID, APIKeyID: k.ID}, nil
}

// IsAPIKey reports whether a bearer token has the API key format.
func IsAPIKey(bearer string) bool { return strings.HasPrefix(bearer, apiKeyPrefix) }

// ---------------------------------------------------------------- API keys

func (s *Service) CreateAPIKey(ctx context.Context, tenantID, name string) (*domain.APIKey, string, error) {
	name, err := domain.ValidateName("name", name)
	if err != nil {
		return nil, "", err
	}
	plain := apiKeyPrefix + ids.SecretBase32(32)
	sum := sha256.Sum256([]byte(plain))
	k := &domain.APIKey{
		ID: ids.New("key"), TenantID: tenantID, Name: name,
		Prefix: plain[:len(apiKeyPrefix)+6], Hash: sum[:], CreatedAt: s.now(),
	}
	if err := s.repo.CreateAPIKey(ctx, k); err != nil {
		return nil, "", err
	}
	return k, plain, nil
}

func (s *Service) ListAPIKeys(ctx context.Context, tenantID string) ([]*domain.APIKey, error) {
	return s.repo.ListAPIKeys(ctx, tenantID)
}

func (s *Service) RevokeAPIKey(ctx context.Context, tenantID, id string) error {
	err := s.repo.RevokeAPIKey(ctx, tenantID, id, s.now())
	if errors.Is(err, ErrNotFound) {
		return errs.NotFound("api_key_not_found", "API key not found")
	}
	return err
}

// ---------------------------------------------------------------- channels

func (s *Service) RegisterChannel(ctx context.Context, tenantID string, p domain.NewChannelParams) (*domain.Channel, error) {
	c, err := domain.NewChannel(ids.New("chn"), tenantID, p, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateChannel(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) Banks() []domain.Bank { return domain.Banks }

func (s *Service) ListChannels(ctx context.Context, tenantID string) ([]*domain.Channel, error) {
	return s.repo.ListChannels(ctx, tenantID)
}

func (s *Service) GetChannel(ctx context.Context, tenantID, id string) (*domain.Channel, error) {
	c, err := s.repo.GetChannel(ctx, tenantID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, errs.NotFound("channel_not_found", "channel not found")
	}
	return c, err
}

func (s *Service) DeactivateChannel(ctx context.Context, tenantID, id string) error {
	err := s.repo.DeactivateChannel(ctx, tenantID, id)
	if errors.Is(err, ErrNotFound) {
		return errs.NotFound("channel_not_found", "channel not found")
	}
	return err
}

// ---------------------------------------------------------------- webhook

type Webhook struct {
	URL    string
	Secret string
}

func (s *Service) GetWebhook(ctx context.Context, tenantID string) (Webhook, error) {
	t, err := s.repo.GetTenant(ctx, tenantID)
	if err != nil {
		return Webhook{}, err
	}
	return Webhook{URL: t.WebhookURL, Secret: t.WebhookSecret}, nil
}

func (s *Service) SetWebhookURL(ctx context.Context, tenantID, rawURL string) (Webhook, error) {
	u, err := domain.ValidateWebhookURL(rawURL)
	if err != nil {
		return Webhook{}, err
	}
	t, err := s.repo.GetTenant(ctx, tenantID)
	if err != nil {
		return Webhook{}, err
	}
	if err := s.repo.UpdateWebhook(ctx, tenantID, u, t.WebhookSecret); err != nil {
		return Webhook{}, err
	}
	return Webhook{URL: u, Secret: t.WebhookSecret}, nil
}

func (s *Service) RotateWebhookSecret(ctx context.Context, tenantID string) (Webhook, error) {
	t, err := s.repo.GetTenant(ctx, tenantID)
	if err != nil {
		return Webhook{}, err
	}
	secret := newWebhookSecret()
	if err := s.repo.UpdateWebhook(ctx, tenantID, t.WebhookURL, secret); err != nil {
		return Webhook{}, err
	}
	return Webhook{URL: t.WebhookURL, Secret: secret}, nil
}

func newWebhookSecret() string { return "whsec_" + ids.Secret(24) }
