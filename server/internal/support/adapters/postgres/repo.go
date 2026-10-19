// Package postgres implements the support Repository.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/platform/db"
	"github.com/eslieh/paytrack-go/server/internal/platform/outbox"
	"github.com/eslieh/paytrack-go/server/internal/support/app"
	"github.com/eslieh/paytrack-go/server/internal/support/domain"
)

type Repo struct{ pool *pgxpool.Pool }

var _ app.Repository = (*Repo)(nil)

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// ---------------------------------------------------------------- tickets

const ticketCols = `id, number, tenant_id, user_id, requester_email, tenant_name, subject, category, priority, status,
	first_response_at, last_message_at, resolved_at, rating, rating_comment, created_at, updated_at`

func scanTicket(row pgx.Row) (*domain.Ticket, error) {
	var t domain.Ticket
	err := row.Scan(&t.ID, &t.Number, &t.TenantID, &t.UserID, &t.RequesterEmail, &t.TenantName, &t.Subject, &t.Category,
		&t.Priority, &t.Status, &t.FirstResponseAt, &t.LastMessageAt, &t.ResolvedAt, &t.Rating, &t.RatingComment,
		&t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, app.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func publish(ctx context.Context, tx pgx.Tx, ns []contracts.Notification) error {
	msgs := make([]outbox.Message, len(ns))
	for i, n := range ns {
		msgs[i] = outbox.Message{Topic: contracts.TopicNotifications, Key: n.TenantID, Payload: n}
	}
	return outbox.Insert(ctx, tx, msgs...)
}

func (r *Repo) CreateTicket(ctx context.Context, t *domain.Ticket, first *domain.Message, build func() []contracts.Notification) error {
	return db.InTx(ctx, r.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `INSERT INTO support.tickets (id, tenant_id, user_id, requester_email, tenant_name, subject,
				category, priority, status, last_message_at, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING number`,
			t.ID, t.TenantID, t.UserID, t.RequesterEmail, t.TenantName, t.Subject, t.Category, t.Priority, t.Status,
			t.LastMessageAt, t.CreatedAt, t.UpdatedAt).Scan(&t.Number)
		if err != nil {
			return err
		}
		if err := insertMessage(ctx, tx, first); err != nil {
			return err
		}
		return publish(ctx, tx, build())
	})
}

func (r *Repo) GetTicket(ctx context.Context, id string) (*domain.Ticket, error) {
	return scanTicket(r.pool.QueryRow(ctx, `SELECT `+ticketCols+` FROM support.tickets WHERE id = $1`, id))
}

func (r *Repo) ListTickets(ctx context.Context, f app.TicketFilter) ([]*domain.Ticket, error) {
	var where []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.TenantID != "" {
		add("tenant_id = $%d", f.TenantID)
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.Before != "" {
		add("id < $%d", f.Before)
	}
	cond := "true"
	if len(where) > 0 {
		cond = strings.Join(where, " AND ")
	}
	args = append(args, f.Limit)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT %s FROM support.tickets WHERE %s ORDER BY id DESC LIMIT $%d`,
		ticketCols, cond, len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Ticket
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

const updateTicket = `UPDATE support.tickets SET priority=$2, status=$3, first_response_at=$4, last_message_at=$5,
	resolved_at=$6, rating=$7, rating_comment=$8, updated_at=$9 WHERE id=$1`

func ticketArgs(t *domain.Ticket) []any {
	return []any{t.ID, t.Priority, t.Status, t.FirstResponseAt, t.LastMessageAt, t.ResolvedAt, t.Rating, t.RatingComment, t.UpdatedAt}
}

func saveTicket(ctx context.Context, tx pgx.Tx, t *domain.Ticket, out []contracts.Notification) error {
	tag, err := tx.Exec(ctx, updateTicket, ticketArgs(t)...)
	if err == nil && tag.RowsAffected() == 0 {
		return app.ErrNotFound
	}
	if err != nil {
		return err
	}
	return publish(ctx, tx, out)
}

func (r *Repo) SaveTicket(ctx context.Context, t *domain.Ticket, out []contracts.Notification) error {
	return db.InTx(ctx, r.pool, func(tx pgx.Tx) error { return saveTicket(ctx, tx, t, out) })
}

func (r *Repo) AddMessage(ctx context.Context, t *domain.Ticket, m *domain.Message, out []contracts.Notification) error {
	return db.InTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := insertMessage(ctx, tx, m); err != nil {
			return err
		}
		return saveTicket(ctx, tx, t, out)
	})
}

func (r *Repo) CountByStatus(ctx context.Context) (map[domain.Status]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT status, count(*) FROM support.tickets GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[domain.Status]int{}
	for rows.Next() {
		var s domain.Status
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[s] = n
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- messages

func insertMessage(ctx context.Context, tx pgx.Tx, m *domain.Message) error {
	_, err := tx.Exec(ctx, `INSERT INTO support.messages (id, ticket_id, author_kind, author_id, author_name, body, internal, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, m.ID, m.TicketID, m.AuthorKind, m.AuthorID, m.AuthorName, m.Body, m.Internal, m.CreatedAt)
	return err
}

func (r *Repo) ListMessages(ctx context.Context, ticketID string, includeInternal bool) ([]*domain.Message, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, ticket_id, author_kind, author_id, author_name, body, internal, created_at
		FROM support.messages WHERE ticket_id = $1 AND ($2 OR NOT internal) ORDER BY id`, ticketID, includeInternal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Message
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.ID, &m.TicketID, &m.AuthorKind, &m.AuthorID, &m.AuthorName, &m.Body, &m.Internal, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- feedback

func (r *Repo) CreateFeedback(ctx context.Context, f *domain.Feedback) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO support.feedback (id, tenant_id, user_id, user_email, rating, message, page, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, f.ID, f.TenantID, f.UserID, f.UserEmail, f.Rating, f.Message, f.Page, f.CreatedAt)
	return err
}

func (r *Repo) ListFeedback(ctx context.Context, before string, limit int) ([]*domain.Feedback, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, tenant_id, user_id, user_email, rating, message, page, created_at
		FROM support.feedback WHERE ($1 = '' OR id < $1) ORDER BY id DESC LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Feedback
	for rows.Next() {
		var f domain.Feedback
		if err := rows.Scan(&f.ID, &f.TenantID, &f.UserID, &f.UserEmail, &f.Rating, &f.Message, &f.Page, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &f)
	}
	return out, rows.Err()
}
