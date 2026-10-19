// Package postgres implements the admin Repository.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eslieh/paytrack-go/server/internal/admin/app"
	"github.com/eslieh/paytrack-go/server/internal/admin/domain"
	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/platform/db"
	"github.com/eslieh/paytrack-go/server/internal/platform/outbox"
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

// ---------------------------------------------------------------- staff

const staffCols = `id, email, name, password_hash, last_login_at, disabled_at, created_at, updated_at`

func scanStaff(row pgx.Row) (*domain.Staff, error) {
	var s domain.Staff
	if err := row.Scan(&s.ID, &s.Email, &s.Name, &s.PasswordHash, &s.LastLoginAt, &s.DisabledAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, nf(err)
	}
	return &s, nil
}

func (r *Repo) CreateStaff(ctx context.Context, s *domain.Staff) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO admin.staff (`+staffCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		s.ID, s.Email, s.Name, s.PasswordHash, s.LastLoginAt, s.DisabledAt, s.CreatedAt, s.UpdatedAt)
	if db.IsUniqueViolation(err, "staff_email_key") {
		return app.ErrEmailTaken
	}
	return err
}

func (r *Repo) GetStaff(ctx context.Context, id string) (*domain.Staff, error) {
	return scanStaff(r.pool.QueryRow(ctx, `SELECT `+staffCols+` FROM admin.staff WHERE id = $1`, id))
}

func (r *Repo) GetStaffByEmail(ctx context.Context, email string) (*domain.Staff, error) {
	return scanStaff(r.pool.QueryRow(ctx, `SELECT `+staffCols+` FROM admin.staff WHERE email = $1`, email))
}

func (r *Repo) SaveStaff(ctx context.Context, s *domain.Staff) error {
	return affected(r.pool.Exec(ctx, `UPDATE admin.staff SET name=$2, password_hash=$3, last_login_at=$4, disabled_at=$5,
		updated_at=$6 WHERE id=$1`, s.ID, s.Name, s.PasswordHash, s.LastLoginAt, s.DisabledAt, s.UpdatedAt))
}

func (r *Repo) ListActiveStaffEmails(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT email FROM admin.staff WHERE disabled_at IS NULL ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// ---------------------------------------------------------------- challenges

func (r *Repo) CreateChallenge(ctx context.Context, c *domain.Challenge) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO admin.login_challenges (id, staff_id, code_hash, attempts, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)`, c.ID, c.StaffID, c.CodeHash, c.Attempts, c.CreatedAt, c.ExpiresAt)
	return err
}

func (r *Repo) GetChallenge(ctx context.Context, id string) (*domain.Challenge, error) {
	var c domain.Challenge
	err := r.pool.QueryRow(ctx, `SELECT id, staff_id, code_hash, attempts, created_at, expires_at, consumed_at
		FROM admin.login_challenges WHERE id = $1`, id).
		Scan(&c.ID, &c.StaffID, &c.CodeHash, &c.Attempts, &c.CreatedAt, &c.ExpiresAt, &c.ConsumedAt)
	if err != nil {
		return nil, nf(err)
	}
	return &c, nil
}

func (r *Repo) SaveChallenge(ctx context.Context, c *domain.Challenge) error {
	return affected(r.pool.Exec(ctx, `UPDATE admin.login_challenges SET attempts=$2, consumed_at=$3 WHERE id=$1`,
		c.ID, c.Attempts, c.ConsumedAt))
}

// ---------------------------------------------------------------- sessions

func (r *Repo) CreateSession(ctx context.Context, s *domain.Session) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO admin.sessions (id, staff_id, token_hash, ip, user_agent, created_at, last_seen_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, s.ID, s.StaffID, s.TokenHash, s.IP, s.UserAgent, s.CreatedAt, s.LastSeenAt, s.ExpiresAt)
	return err
}

func (r *Repo) GetSessionByToken(ctx context.Context, hash []byte) (*domain.Session, error) {
	var s domain.Session
	err := r.pool.QueryRow(ctx, `SELECT id, staff_id, token_hash, ip, user_agent, created_at, last_seen_at, expires_at, revoked_at
		FROM admin.sessions WHERE token_hash = $1`, hash).
		Scan(&s.ID, &s.StaffID, &s.TokenHash, &s.IP, &s.UserAgent, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt, &s.RevokedAt)
	if err != nil {
		return nil, nf(err)
	}
	return &s, nil
}

func (r *Repo) TouchSession(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE admin.sessions SET last_seen_at = $2 WHERE id = $1`, id, at)
	return err
}

func (r *Repo) RevokeSession(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE admin.sessions SET revoked_at = COALESCE(revoked_at, $2) WHERE id = $1`, id, at)
	return err
}

func (r *Repo) RevokeStaffSessions(ctx context.Context, staffID string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE admin.sessions SET revoked_at = $2 WHERE staff_id = $1 AND revoked_at IS NULL`, staffID, at)
	return err
}

// ---------------------------------------------------------------- audit log

func (r *Repo) AddAudit(ctx context.Context, e *domain.AuditEntry, out []contracts.Notification) error {
	data := []byte("{}")
	if e.Data != nil {
		var err error
		if data, err = json.Marshal(e.Data); err != nil {
			return err
		}
	}
	return db.InTx(ctx, r.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `INSERT INTO admin.audit_log (staff_id, action, target_type, target_id, reason, data, ip, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
			e.StaffID, e.Action, e.TargetType, e.TargetID, e.Reason, data, e.IP, e.CreatedAt).Scan(&e.ID)
		if err != nil {
			return err
		}
		msgs := make([]outbox.Message, len(out))
		for i, n := range out {
			msgs[i] = outbox.Message{Topic: contracts.TopicNotifications, Key: n.TenantID, Payload: n}
		}
		return outbox.Insert(ctx, tx, msgs...)
	})
}

func (r *Repo) ListAudit(ctx context.Context, f app.AuditFilter) ([]*domain.AuditEntry, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, staff_id, action, target_type, target_id, reason, data, ip, created_at
		FROM admin.audit_log
		WHERE ($1 = '' OR target_type = $1) AND ($2 = '' OR target_id = $2) AND ($3 = 0 OR id < $3)
		ORDER BY id DESC LIMIT $4`, f.TargetType, f.TargetID, f.BeforeID, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.AuditEntry
	for rows.Next() {
		var e domain.AuditEntry
		var data []byte
		if err := rows.Scan(&e.ID, &e.StaffID, &e.Action, &e.TargetType, &e.TargetID, &e.Reason, &data, &e.IP, &e.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &e.Data); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}
