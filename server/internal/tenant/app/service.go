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
	repo   Repository
	hasher PasswordHasher
	tokens TokenIssuer
	now    func() time.Time
	// dummyHash keeps Login's timing similar whether or not the email exists.
	dummyHash string
}

func NewService(repo Repository, hasher PasswordHasher, tokens TokenIssuer) (*Service, error) {
	dummy, err := hasher.Hash("not-a-real-password")
	if err != nil {
		return nil, err
	}
	return &Service{repo: repo, hasher: hasher, tokens: tokens, now: time.Now, dummyHash: dummy}, nil
}

type Session struct {
	Token     string
	ExpiresAt time.Time
	User      *domain.User
	Tenant    *domain.Tenant
}

type RegisterCmd struct {
	BusinessName string
	Email        string
	Password     string
}

// Register creates a tenant with its owner user and signs them in.
func (s *Service) Register(ctx context.Context, cmd RegisterCmd) (*Session, error) {
	name, err := domain.ValidateName("business_name", cmd.BusinessName)
	if err != nil {
		return nil, err
	}
	email, err := domain.NormalizeEmail(cmd.Email)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePassword(cmd.Password); err != nil {
		return nil, err
	}
	hash, err := s.hasher.Hash(cmd.Password)
	if err != nil {
		return nil, err
	}
	now := s.now()
	t := &domain.Tenant{ID: ids.New("ten"), Name: name, WebhookSecret: newWebhookSecret(), CreatedAt: now}
	u := &domain.User{ID: ids.New("usr"), TenantID: t.ID, Email: email, PasswordHash: hash, Role: domain.RoleOwner, CreatedAt: now}
	if err := s.repo.CreateTenantWithOwner(ctx, t, u); err != nil {
		if errors.Is(err, ErrEmailTaken) {
			return nil, errs.Conflict("email_taken", "an account with this email already exists")
		}
		return nil, err
	}
	return s.session(u, t)
}

func (s *Service) Login(ctx context.Context, email, password string) (*Session, error) {
	invalid := errs.Unauthorized("invalid_credentials", "email or password is incorrect")
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		s.hasher.Compare(s.dummyHash, password)
		return nil, invalid
	}
	if err != nil {
		return nil, err
	}
	if !s.hasher.Compare(u.PasswordHash, password) {
		return nil, invalid
	}
	t, err := s.repo.GetTenant(ctx, u.TenantID)
	if err != nil {
		return nil, err
	}
	return s.session(u, t)
}

func (s *Service) session(u *domain.User, t *domain.Tenant) (*Session, error) {
	tok, exp, err := s.tokens.Issue(u.ID, t.ID)
	if err != nil {
		return nil, err
	}
	return &Session{Token: tok, ExpiresAt: exp, User: u, Tenant: t}, nil
}

// Authenticate implements auth.Authenticator for both API keys and
// dashboard session tokens.
func (s *Service) Authenticate(ctx context.Context, bearer string) (auth.Principal, error) {
	if strings.HasPrefix(bearer, apiKeyPrefix) {
		sum := sha256.Sum256([]byte(bearer))
		k, err := s.repo.GetAPIKeyByHash(ctx, sum[:])
		if errors.Is(err, ErrNotFound) || (err == nil && k.RevokedAt != nil) {
			return auth.Principal{}, errs.Unauthorized("invalid_api_key", "API key is invalid or revoked")
		}
		if err != nil {
			return auth.Principal{}, err
		}
		if k.LastUsedAt == nil || s.now().Sub(*k.LastUsedAt) > time.Minute {
			if err := s.repo.TouchAPIKey(ctx, k.ID, s.now()); err != nil {
				slog.WarnContext(ctx, "touch api key failed", "err", err)
			}
		}
		return auth.Principal{Kind: auth.KindAPIKey, TenantID: k.TenantID, APIKeyID: k.ID}, nil
	}
	userID, tenantID, err := s.tokens.Verify(bearer)
	if err != nil {
		return auth.Principal{}, errs.Unauthorized("invalid_token", "session token is invalid or expired")
	}
	return auth.Principal{Kind: auth.KindUser, TenantID: tenantID, UserID: userID}, nil
}

func (s *Service) Me(ctx context.Context, p auth.Principal) (*domain.Tenant, *domain.User, error) {
	t, err := s.repo.GetTenant(ctx, p.TenantID)
	if err != nil {
		return nil, nil, err
	}
	if p.UserID == "" {
		return t, nil, nil
	}
	u, err := s.repo.GetUser(ctx, p.UserID)
	if err != nil {
		return nil, nil, err
	}
	return t, u, nil
}

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

func (s *Service) ListChannels(ctx context.Context, tenantID string) ([]*domain.Channel, error) {
	return s.repo.ListChannels(ctx, tenantID)
}

// GetChannel returns a tenant's channel including decrypted credentials.
// Inbound adapters must not expose Credentials.
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
