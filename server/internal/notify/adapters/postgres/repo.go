// Package postgres implements the notify Repository.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eslieh/paytrack-go/server/internal/notify/app"
	"github.com/eslieh/paytrack-go/server/internal/notify/domain"
)

type Repo struct{ pool *pgxpool.Pool }

var _ app.Repository = (*Repo)(nil)

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

const cols = `id, event_id, tenant_id, payment_id, event_type, payload, status, attempts, next_attempt_at,
	last_error, last_status_code, delivered_at, created_at, updated_at`

func scan(row pgx.Row) (*domain.Delivery, error) {
	var d domain.Delivery
	err := row.Scan(&d.ID, &d.EventID, &d.TenantID, &d.PaymentID, &d.EventType, &d.Payload, &d.Status, &d.Attempts,
		&d.NextAttemptAt, &d.LastError, &d.LastStatusCode, &d.DeliveredAt, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, app.ErrNotFound
	}
	return &d, err
}

func collect(rows pgx.Rows, err error) ([]*domain.Delivery, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Delivery
	for rows.Next() {
		d, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repo) Create(ctx context.Context, d *domain.Delivery) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO notify.deliveries (`+cols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT ON CONSTRAINT deliveries_event_key DO NOTHING`,
		d.ID, d.EventID, d.TenantID, d.PaymentID, d.EventType, d.Payload, d.Status, d.Attempts, d.NextAttemptAt,
		d.LastError, d.LastStatusCode, d.DeliveredAt, d.CreatedAt, d.UpdatedAt)
	return err
}

func (r *Repo) Get(ctx context.Context, tenantID, id string) (*domain.Delivery, error) {
	return scan(r.pool.QueryRow(ctx, `SELECT `+cols+` FROM notify.deliveries WHERE tenant_id = $1 AND id = $2`, tenantID, id))
}

func (r *Repo) Save(ctx context.Context, d *domain.Delivery) error {
	_, err := r.pool.Exec(ctx, `UPDATE notify.deliveries SET status=$2, attempts=$3, next_attempt_at=$4,
		last_error=$5, last_status_code=$6, delivered_at=$7, updated_at=$8 WHERE id=$1`,
		d.ID, d.Status, d.Attempts, d.NextAttemptAt, d.LastError, d.LastStatusCode, d.DeliveredAt, d.UpdatedAt)
	return err
}

func (r *Repo) List(ctx context.Context, f app.ListFilter) ([]*domain.Delivery, error) {
	where := []string{"tenant_id = $1"}
	args := []any{f.TenantID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.PaymentID != "" {
		add("payment_id = $%d", f.PaymentID)
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.Before != "" {
		add("id < $%d", f.Before)
	}
	args = append(args, f.Limit)
	return collect(r.pool.Query(ctx, fmt.Sprintf(`SELECT %s FROM notify.deliveries WHERE %s ORDER BY id DESC LIMIT $%d`,
		cols, strings.Join(where, " AND "), len(args)), args...))
}

func (r *Repo) ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]*domain.Delivery, error) {
	return collect(r.pool.Query(ctx, `UPDATE notify.deliveries SET status = 'delivering', next_attempt_at = $2
		WHERE id IN (
			SELECT id FROM notify.deliveries
			WHERE status IN ('pending', 'delivering') AND next_attempt_at <= $1
			ORDER BY next_attempt_at LIMIT $3 FOR UPDATE SKIP LOCKED)
		RETURNING `+cols, now, now.Add(lease), limit))
}
