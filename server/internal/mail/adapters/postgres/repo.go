// Package postgres implements the mail Repository.
package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eslieh/paytrack-go/server/internal/mail/app"
	"github.com/eslieh/paytrack-go/server/internal/mail/domain"
)

type Repo struct{ pool *pgxpool.Pool }

var _ app.Repository = (*Repo)(nil)

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

const cols = `id, COALESCE(dedupe_key, ''), template, category, to_email, to_name, data::text, status, attempts,
	next_attempt_at, provider_message_id, last_error, created_at, updated_at, sent_at`

func (r *Repo) Create(ctx context.Context, m *domain.Message) error {
	data, err := json.Marshal(m.Data)
	if err != nil {
		return err
	}
	var dedupe *string
	if m.DedupeKey != "" {
		dedupe = &m.DedupeKey
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO mail.messages (id, dedupe_key, template, category, to_email, to_name, data,
		status, attempts, next_attempt_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, 0, $9, $10, $10)
		ON CONFLICT ON CONSTRAINT messages_dedupe_key DO NOTHING`,
		m.ID, dedupe, m.Template, m.Category, m.ToEmail, m.ToName, string(data), m.Status, m.NextAttemptAt, m.CreatedAt)
	return err
}

func (r *Repo) Save(ctx context.Context, m *domain.Message) error {
	_, err := r.pool.Exec(ctx, `UPDATE mail.messages SET status=$2, attempts=$3, next_attempt_at=$4,
		provider_message_id=$5, last_error=$6, updated_at=$7, sent_at=$8 WHERE id=$1`,
		m.ID, m.Status, m.Attempts, m.NextAttemptAt, m.ProviderMessageID, m.LastError, m.UpdatedAt, m.SentAt)
	return err
}

func (r *Repo) ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]*domain.Message, error) {
	rows, err := r.pool.Query(ctx, `UPDATE mail.messages SET status = 'sending', next_attempt_at = $2
		WHERE id IN (SELECT id FROM mail.messages WHERE status IN ('queued', 'sending') AND next_attempt_at <= $1
			ORDER BY next_attempt_at LIMIT $3 FOR UPDATE SKIP LOCKED)
		RETURNING `+cols, now, now.Add(lease), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Message
	for rows.Next() {
		var m domain.Message
		var data string
		if err := rows.Scan(&m.ID, &m.DedupeKey, &m.Template, &m.Category, &m.ToEmail, &m.ToName, &data, &m.Status,
			&m.Attempts, &m.NextAttemptAt, &m.ProviderMessageID, &m.LastError, &m.CreatedAt, &m.UpdatedAt, &m.SentAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(data), &m.Data); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

var _ = pgx.ErrNoRows
