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
	"github.com/eslieh/paytrack-go/server/internal/platform/db"
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

func (r *Repo) CreateTenantWithOwner(ctx context.Context, t *domain.Tenant, u *domain.User) error {
	secret, err := r.encrypt(t.WebhookSecret)
	if err != nil {
		return err
	}
	err = db.InTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO tenant.tenants (id, name, webhook_secret_enc, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4)`, t.ID, t.Name, secret, t.CreatedAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO tenant.users (id, tenant_id, email, password_hash, role, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)`, u.ID, u.TenantID, u.Email, u.PasswordHash, u.Role, u.CreatedAt)
		return err
	})
	if db.IsUniqueViolation(err, "users_email_key") {
		return app.ErrEmailTaken
	}
	return err
}

func (r *Repo) GetTenant(ctx context.Context, id string) (*domain.Tenant, error) {
	var t domain.Tenant
	var url *string
	var secret []byte
	err := r.pool.QueryRow(ctx, `SELECT id, name, webhook_url, webhook_secret_enc, created_at
		FROM tenant.tenants WHERE id = $1`, id).Scan(&t.ID, &t.Name, &url, &secret, &t.CreatedAt)
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

// ---------------------------------------------------------------- users

const userCols = `id, tenant_id, email, password_hash, role, created_at`

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	if err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

func (r *Repo) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM tenant.users WHERE id = $1`, id))
}

func (r *Repo) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM tenant.users WHERE email = $1`, email))
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
