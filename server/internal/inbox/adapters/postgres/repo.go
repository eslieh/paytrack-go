// Package postgres implements the inbox Repository.
package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eslieh/paytrack-go/server/internal/inbox/app"
	"github.com/eslieh/paytrack-go/server/internal/inbox/domain"
)

type Repo struct{ pool *pgxpool.Pool }

var _ app.Repository = (*Repo)(nil)

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// visible matches the caller's own notifications and tenant-wide ones.
const visible = `tenant_id = $1 AND (user_id = '' OR user_id = $2)`

func (r *Repo) Insert(ctx context.Context, n *domain.Notification) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO inbox.notifications (id, tenant_id, user_id, kind, title, body, link, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (id) DO NOTHING`,
		n.ID, n.TenantID, n.UserID, n.Kind, n.Title, n.Body, n.Link, n.CreatedAt)
	return err
}

func (r *Repo) List(ctx context.Context, tenantID, userID string, before time.Time, limit int) ([]*domain.Notification, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, user_id, kind, title, body, link, read_at, created_at
		FROM inbox.notifications WHERE `+visible+` AND ($3::timestamptz IS NULL OR created_at < $3)
		ORDER BY created_at DESC LIMIT $4`, tenantID, userID, nullTime(before), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Notification
	for rows.Next() {
		var n domain.Notification
		if err := rows.Scan(&n.ID, &n.TenantID, &n.UserID, &n.Kind, &n.Title, &n.Body, &n.Link, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &n)
	}
	return out, rows.Err()
}

func (r *Repo) CountUnread(ctx context.Context, tenantID, userID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM inbox.notifications WHERE `+visible+` AND read_at IS NULL`,
		tenantID, userID).Scan(&n)
	return n, err
}

func (r *Repo) MarkRead(ctx context.Context, tenantID, userID, id string, at time.Time) error {
	tag, err := r.pool.Exec(ctx, `UPDATE inbox.notifications SET read_at = COALESCE(read_at, $4)
		WHERE `+visible+` AND id = $3`, tenantID, userID, id, at)
	if err == nil && tag.RowsAffected() == 0 {
		return app.ErrNotFound
	}
	return err
}

func (r *Repo) MarkAllRead(ctx context.Context, tenantID, userID string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE inbox.notifications SET read_at = $3 WHERE `+visible+` AND read_at IS NULL`,
		tenantID, userID, at)
	return err
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
