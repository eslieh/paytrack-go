// Package postgres implements the identity Repository.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eslieh/paytrack-go/server/internal/identity/app"
	"github.com/eslieh/paytrack-go/server/internal/identity/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/db"
)

type Repo struct{ pool *pgxpool.Pool }

var _ app.Repository = (*Repo)(nil)

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func nf(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	return err
}

func affected(tag interface{ RowsAffected() int64 }, err error) error {
	if err == nil && tag.RowsAffected() == 0 {
		return app.ErrNotFound
	}
	return err
}

// ---------------------------------------------------------------- users

const userCols = `id, tenant_id, email, password_hash, role, email_verified_at, two_step_enabled, last_login_at, created_at, updated_at`

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.Role, &u.EmailVerifiedAt, &u.TwoStepEnabled,
		&u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, nf(err)
	}
	return &u, nil
}

func (r *Repo) CreateUser(ctx context.Context, u *domain.User) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO identity.users (`+userCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		u.ID, u.TenantID, u.Email, u.PasswordHash, u.Role, u.EmailVerifiedAt, u.TwoStepEnabled, u.LastLoginAt, u.CreatedAt, u.UpdatedAt)
	if db.IsUniqueViolation(err, "users_email_key") {
		return app.ErrEmailTaken
	}
	return err
}

func (r *Repo) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM identity.users WHERE id = $1`, id))
}

func (r *Repo) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM identity.users WHERE email = $1`, email))
}

func (r *Repo) GetOwner(ctx context.Context, tenantID string) (*domain.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `SELECT `+userCols+` FROM identity.users WHERE tenant_id = $1 AND role = 'owner'
		ORDER BY created_at LIMIT 1`, tenantID))
}

func (r *Repo) SaveUser(ctx context.Context, u *domain.User) error {
	return affected(r.pool.Exec(ctx, `UPDATE identity.users SET email=$2, password_hash=$3, email_verified_at=$4,
		two_step_enabled=$5, last_login_at=$6, updated_at=$7 WHERE id=$1`,
		u.ID, u.Email, u.PasswordHash, u.EmailVerifiedAt, u.TwoStepEnabled, u.LastLoginAt, u.UpdatedAt))
}

// ---------------------------------------------------------------- sessions

const sessCols = `id, user_id, tenant_id, refresh_hash, prev_refresh_hash, auth_method, user_agent, ip, created_at,
	last_seen_at, expires_at, revoked_at`

func scanSession(row pgx.Row) (*domain.Session, error) {
	var s domain.Session
	err := row.Scan(&s.ID, &s.UserID, &s.TenantID, &s.RefreshHash, &s.PrevRefreshHash, &s.AuthMethod, &s.UserAgent,
		&s.IP, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt, &s.RevokedAt)
	if err != nil {
		return nil, nf(err)
	}
	return &s, nil
}

func (r *Repo) CreateSession(ctx context.Context, s *domain.Session) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO identity.sessions (`+sessCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		s.ID, s.UserID, s.TenantID, s.RefreshHash, s.PrevRefreshHash, s.AuthMethod, s.UserAgent, s.IP, s.CreatedAt,
		s.LastSeenAt, s.ExpiresAt, s.RevokedAt)
	return err
}

func (r *Repo) GetActiveSession(ctx context.Context, id string, now time.Time) (*domain.Session, *domain.User, error) {
	var s domain.Session
	var u domain.User
	err := r.pool.QueryRow(ctx, `SELECT s.id, s.user_id, s.tenant_id, s.expires_at,
			u.id, u.tenant_id, u.email, u.email_verified_at, u.two_step_enabled
		FROM identity.sessions s JOIN identity.users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.revoked_at IS NULL AND s.expires_at > $2`, id, now).
		Scan(&s.ID, &s.UserID, &s.TenantID, &s.ExpiresAt, &u.ID, &u.TenantID, &u.Email, &u.EmailVerifiedAt, &u.TwoStepEnabled)
	if err != nil {
		return nil, nil, nf(err)
	}
	return &s, &u, nil
}

func (r *Repo) GetSessionByRefresh(ctx context.Context, hash []byte) (*domain.Session, error) {
	return scanSession(r.pool.QueryRow(ctx, `SELECT `+sessCols+` FROM identity.sessions WHERE refresh_hash = $1`, hash))
}

func (r *Repo) GetSessionByPrevRefresh(ctx context.Context, hash []byte) (*domain.Session, error) {
	return scanSession(r.pool.QueryRow(ctx, `SELECT `+sessCols+` FROM identity.sessions WHERE prev_refresh_hash = $1`, hash))
}

func (r *Repo) RotateRefresh(ctx context.Context, id string, oldHash, newHash []byte, now time.Time, ip, ua string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE identity.sessions SET refresh_hash = $3, prev_refresh_hash = $2,
		last_seen_at = $4, ip = $5, user_agent = $6 WHERE id = $1 AND refresh_hash = $2 AND revoked_at IS NULL`,
		id, oldHash, newHash, now, ip, ua)
	if err == nil && tag.RowsAffected() == 0 {
		return app.ErrStaleRefresh
	}
	return err
}

func (r *Repo) ListSessions(ctx context.Context, userID string, now time.Time) ([]*domain.Session, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+sessCols+` FROM identity.sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $2 ORDER BY last_seen_at DESC LIMIT 50`, userID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repo) RevokeSession(ctx context.Context, userID, id string, now time.Time) error {
	return affected(r.pool.Exec(ctx, `UPDATE identity.sessions SET revoked_at = $3
		WHERE user_id = $1 AND id = $2 AND revoked_at IS NULL`, userID, id, now))
}

func (r *Repo) RevokeUserSessions(ctx context.Context, userID, exceptID string, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE identity.sessions SET revoked_at = $3
		WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL`, userID, exceptID, now)
	return err
}

// ---------------------------------------------------------------- tokens

const tokCols = `id, user_id, purpose, token_hash, code_hash, attempts, sends, last_sent_at, data::text, created_at, expires_at, consumed_at`

func scanToken(row pgx.Row) (*domain.Token, error) {
	var t domain.Token
	var data string
	err := row.Scan(&t.ID, &t.UserID, &t.Purpose, &t.TokenHash, &t.CodeHash, &t.Attempts, &t.Sends, &t.LastSentAt,
		&data, &t.CreatedAt, &t.ExpiresAt, &t.ConsumedAt)
	if err != nil {
		return nil, nf(err)
	}
	if err := json.Unmarshal([]byte(data), &t.Data); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repo) CreateToken(ctx context.Context, t *domain.Token) error {
	data, err := json.Marshal(nonNil(t.Data))
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO identity.tokens (`+"id, user_id, purpose, token_hash, code_hash, attempts, sends, last_sent_at, data, created_at, expires_at, consumed_at"+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11,$12)`,
		t.ID, t.UserID, t.Purpose, t.TokenHash, t.CodeHash, t.Attempts, t.Sends, t.LastSentAt, string(data), t.CreatedAt,
		t.ExpiresAt, t.ConsumedAt)
	return err
}

func nonNil(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func (r *Repo) GetToken(ctx context.Context, id string) (*domain.Token, error) {
	return scanToken(r.pool.QueryRow(ctx, `SELECT `+tokCols+` FROM identity.tokens WHERE id = $1`, id))
}

func (r *Repo) GetTokenByHash(ctx context.Context, purpose domain.TokenPurpose, hash []byte) (*domain.Token, error) {
	return scanToken(r.pool.QueryRow(ctx, `SELECT `+tokCols+` FROM identity.tokens WHERE token_hash = $1 AND purpose = $2`, hash, purpose))
}

func (r *Repo) SaveToken(ctx context.Context, t *domain.Token) error {
	return affected(r.pool.Exec(ctx, `UPDATE identity.tokens SET code_hash=$2, attempts=$3, sends=$4, last_sent_at=$5,
		consumed_at=$6 WHERE id=$1`, t.ID, t.CodeHash, t.Attempts, t.Sends, t.LastSentAt, t.ConsumedAt))
}

func (r *Repo) InvalidateTokens(ctx context.Context, userID string, purpose domain.TokenPurpose, now time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE identity.tokens SET consumed_at = $3
		WHERE user_id = $1 AND purpose = $2 AND consumed_at IS NULL`, userID, purpose, now)
	return err
}

// ---------------------------------------------------------------- trusted devices

func (r *Repo) CreateTrustedDevice(ctx context.Context, d *domain.TrustedDevice) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO identity.trusted_devices (id, user_id, token_hash, user_agent, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)`, d.ID, d.UserID, d.TokenHash, d.UserAgent, d.CreatedAt, d.ExpiresAt)
	return err
}

func (r *Repo) IsTrustedDevice(ctx context.Context, userID string, hash []byte, now time.Time) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM identity.trusted_devices
		WHERE user_id = $1 AND token_hash = $2 AND expires_at > $3)`, userID, hash, now).Scan(&ok)
	return ok, err
}

func (r *Repo) DeleteTrustedDevices(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM identity.trusted_devices WHERE user_id = $1`, userID)
	return err
}

// ---------------------------------------------------------------- OAuth identities

const identCols = `provider, subject, user_id, email, created_at`

func scanIdent(row pgx.Row) (*domain.OAuthIdentity, error) {
	var i domain.OAuthIdentity
	if err := row.Scan(&i.Provider, &i.Subject, &i.UserID, &i.Email, &i.CreatedAt); err != nil {
		return nil, nf(err)
	}
	return &i, nil
}

func (r *Repo) GetIdentity(ctx context.Context, provider, subject string) (*domain.OAuthIdentity, error) {
	return scanIdent(r.pool.QueryRow(ctx, `SELECT `+identCols+` FROM identity.oauth_identities WHERE provider = $1 AND subject = $2`, provider, subject))
}

func (r *Repo) ListIdentities(ctx context.Context, userID string) ([]*domain.OAuthIdentity, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+identCols+` FROM identity.oauth_identities WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.OAuthIdentity
	for rows.Next() {
		i, err := scanIdent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (r *Repo) CreateIdentity(ctx context.Context, i *domain.OAuthIdentity) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO identity.oauth_identities (`+identCols+`) VALUES ($1,$2,$3,$4,$5)`,
		i.Provider, i.Subject, i.UserID, i.Email, i.CreatedAt)
	if db.IsUniqueViolation(err, "") {
		return app.ErrIdentityTaken
	}
	return err
}

func (r *Repo) DeleteIdentity(ctx context.Context, userID, provider string) error {
	return affected(r.pool.Exec(ctx, `DELETE FROM identity.oauth_identities WHERE user_id = $1 AND provider = $2`, userID, provider))
}

// ---------------------------------------------------------------- audit

func (r *Repo) AddEvent(ctx context.Context, e *domain.SecurityEvent) error {
	data, err := json.Marshal(nonNil(e.Data))
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO identity.security_events (user_id, tenant_id, type, ip, user_agent, data, created_at)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7)`, e.UserID, e.TenantID, e.Type, e.IP, e.UserAgent, string(data), e.CreatedAt)
	return err
}

func (r *Repo) ListEvents(ctx context.Context, userID string, limit int) ([]*domain.SecurityEvent, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, user_id, tenant_id, type, ip, user_agent, data::text, created_at
		FROM identity.security_events WHERE user_id = $1 ORDER BY id DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.SecurityEvent
	for rows.Next() {
		var e domain.SecurityEvent
		var data string
		if err := rows.Scan(&e.ID, &e.UserID, &e.TenantID, &e.Type, &e.IP, &e.UserAgent, &data, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(data), &e.Data)
		out = append(out, &e)
	}
	return out, rows.Err()
}
