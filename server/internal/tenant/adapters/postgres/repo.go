// Package postgres implements the tenant Repository on Postgres. Webhook
// signing secrets are encrypted before they are written.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eslieh/paytrack-go/server/internal/platform/crypto"
	"github.com/eslieh/paytrack-go/server/internal/tenant/app"
	"github.com/eslieh/paytrack-go/server/internal/tenant/domain"
)

type Repo struct {
	pool   *pgxpool.Pool
	cipher *crypto.Cipher
}

var _ app.Repository = (*Repo)(nil)

func New(pool *pgxpool.Pool, cipher *crypto.Cipher) *Repo { return &Repo{pool: pool, cipher: cipher} }

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	return err
}

func (r *Repo) encrypt(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	return r.cipher.Encrypt([]byte(s))
}

func (r *Repo) decrypt(b []byte) (string, error) {
	if len(b) == 0 {
		return "", nil
	}
	p, err := r.cipher.Decrypt(b)
	return string(p), err
}

// ---------------------------------------------------------------- tenants

func (r *Repo) CreateTenant(ctx context.Context, t *domain.Tenant) error {
	secret, err := r.encrypt(t.WebhookSecret)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO tenant.tenants (id, name, webhook_secret_enc, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)`, t.ID, t.Name, secret, t.CreatedAt)
	return err
}

func (r *Repo) ListTenantsCreatedBetween(ctx context.Context, from, to time.Time) ([]*domain.Tenant, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, created_at FROM tenant.tenants
		WHERE created_at >= $1 AND created_at < $2 ORDER BY created_at`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Tenant
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

func (r *Repo) GetNotificationSettings(ctx context.Context, tenantID string) (domain.NotificationSettings, error) {
	s := domain.DefaultNotificationSettings(tenantID)
	err := r.pool.QueryRow(ctx, `SELECT payment_receipts, receipt_email, webhook_failure_alerts, updated_at
		FROM tenant.notification_settings WHERE tenant_id = $1`, tenantID).
		Scan(&s.PaymentReceipts, &s.ReceiptEmail, &s.WebhookFailureAlerts, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, nil
	}
	return s, err
}

func (r *Repo) SaveNotificationSettings(ctx context.Context, s domain.NotificationSettings) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO tenant.notification_settings (tenant_id, payment_receipts, receipt_email,
		webhook_failure_alerts, updated_at) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id) DO UPDATE SET payment_receipts = $2, receipt_email = $3, webhook_failure_alerts = $4, updated_at = $5`,
		s.TenantID, s.PaymentReceipts, s.ReceiptEmail, s.WebhookFailureAlerts, s.UpdatedAt)
	return err
}

func (r *Repo) HasActiveChannel(ctx context.Context, tenantID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tenant.channels WHERE tenant_id = $1 AND active)`, tenantID).Scan(&ok)
	return ok, err
}

func (r *Repo) HasActiveAPIKey(ctx context.Context, tenantID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tenant.api_keys WHERE tenant_id = $1 AND revoked_at IS NULL)`, tenantID).Scan(&ok)
	return ok, err
}

func (r *Repo) GetTenant(ctx context.Context, id string) (*domain.Tenant, error) {
	var t domain.Tenant
	var url *string
	var secret []byte
	err := r.pool.QueryRow(ctx, `SELECT id, name, webhook_url, webhook_secret_enc, status, suspended_at, suspended_reason, created_at
		FROM tenant.tenants WHERE id = $1`, id).
		Scan(&t.ID, &t.Name, &url, &secret, &t.Status, &t.SuspendedAt, &t.SuspendedReason, &t.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	if url != nil {
		t.WebhookURL = *url
	}
	if t.WebhookSecret, err = r.decrypt(secret); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repo) GetTenantStatus(ctx context.Context, id string) (domain.Status, error) {
	var st domain.Status
	err := r.pool.QueryRow(ctx, `SELECT status FROM tenant.tenants WHERE id = $1`, id).Scan(&st)
	return st, notFound(err)
}

func (r *Repo) SaveTenantStatus(ctx context.Context, t *domain.Tenant) error {
	tag, err := r.pool.Exec(ctx, `UPDATE tenant.tenants SET status = $2, suspended_at = $3, suspended_reason = $4,
		updated_at = now() WHERE id = $1`, t.ID, t.Status, t.SuspendedAt, t.SuspendedReason)
	if err == nil && tag.RowsAffected() == 0 {
		return app.ErrNotFound
	}
	return err
}

func (r *Repo) UpdateWebhook(ctx context.Context, tenantID, url, secret string) error {
	enc, err := r.encrypt(secret)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, `UPDATE tenant.tenants SET webhook_url = NULLIF($2, ''), webhook_secret_enc = $3,
		updated_at = now() WHERE id = $1`, tenantID, url, enc)
	if err == nil && tag.RowsAffected() == 0 {
		return app.ErrNotFound
	}
	return err
}

// ---------------------------------------------------------------- API keys

const keyCols = `id, tenant_id, name, prefix, hash, created_at, last_used_at, revoked_at`

func scanKey(row pgx.Row) (*domain.APIKey, error) {
	var k domain.APIKey
	if err := row.Scan(&k.ID, &k.TenantID, &k.Name, &k.Prefix, &k.Hash, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt); err != nil {
		return nil, notFound(err)
	}
	return &k, nil
}

func (r *Repo) CreateAPIKey(ctx context.Context, k *domain.APIKey) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO tenant.api_keys (id, tenant_id, name, prefix, hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, k.ID, k.TenantID, k.Name, k.Prefix, k.Hash, k.CreatedAt)
	return err
}

func (r *Repo) ListAPIKeys(ctx context.Context, tenantID string) ([]*domain.APIKey, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+keyCols+` FROM tenant.api_keys WHERE tenant_id = $1 ORDER BY id DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.APIKey
	for rows.Next() {
		k, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *Repo) GetAPIKeyByHash(ctx context.Context, hash []byte) (*domain.APIKey, error) {
	return scanKey(r.pool.QueryRow(ctx, `SELECT `+keyCols+` FROM tenant.api_keys WHERE hash = $1`, hash))
}

func (r *Repo) RevokeAPIKey(ctx context.Context, tenantID, id string, at time.Time) error {
	tag, err := r.pool.Exec(ctx, `UPDATE tenant.api_keys SET revoked_at = COALESCE(revoked_at, $3)
		WHERE tenant_id = $1 AND id = $2`, tenantID, id, at)
	if err == nil && tag.RowsAffected() == 0 {
		return app.ErrNotFound
	}
	return err
}

func (r *Repo) TouchAPIKey(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE tenant.api_keys SET last_used_at = $2 WHERE id = $1`, id, at)
	return err
}

// ---------------------------------------------------------------- channels

const chanCols = `id, tenant_id, name, type, party_b, bank_name, account_number, active, created_at`

func scanChannel(row pgx.Row) (*domain.Channel, error) {
	var c domain.Channel
	if err := row.Scan(&c.ID, &c.TenantID, &c.Name, &c.Type, &c.PartyB, &c.BankName, &c.AccountNumber,
		&c.Active, &c.CreatedAt); err != nil {
		return nil, notFound(err)
	}
	return &c, nil
}

func (r *Repo) CreateChannel(ctx context.Context, c *domain.Channel) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO tenant.channels (`+chanCols+`, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`,
		c.ID, c.TenantID, c.Name, c.Type, c.PartyB, c.BankName, c.AccountNumber, c.Active, c.CreatedAt)
	return err
}

func (r *Repo) ListChannels(ctx context.Context, tenantID string) ([]*domain.Channel, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+chanCols+` FROM tenant.channels WHERE tenant_id = $1 ORDER BY id DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Channel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repo) GetChannel(ctx context.Context, tenantID, id string) (*domain.Channel, error) {
	return scanChannel(r.pool.QueryRow(ctx, `SELECT `+chanCols+` FROM tenant.channels WHERE tenant_id = $1 AND id = $2`, tenantID, id))
}

func (r *Repo) DeactivateChannel(ctx context.Context, tenantID, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE tenant.channels SET active = false, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return app.ErrNotFound
	}
	return err
}
