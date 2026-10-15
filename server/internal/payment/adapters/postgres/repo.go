// Package postgres implements the payment Repository, writing payment
// events to the transactional outbox alongside state changes.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/payment/app"
	"github.com/eslieh/paytrack-go/server/internal/payment/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/db"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
	"github.com/eslieh/paytrack-go/server/internal/platform/outbox"
)

type Repo struct{ pool *pgxpool.Pool }

var _ app.Repository = (*Repo)(nil)

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

const cols = `id, tenant_id, channel_id, destination_type, party_b, bank_name, idempotency_key, request_hash, status, amount, phone,
	account_reference, description, metadata, callback_token, merchant_request_id, checkout_request_id,
	mpesa_receipt, result_code, result_desc, failure_reason, check_attempts, next_check_at, version,
	created_at, updated_at, completed_at`

func scan(row pgx.Row) (*domain.Payment, error) {
	var p domain.Payment
	err := row.Scan(&p.ID, &p.TenantID, &p.ChannelID, &p.DestinationType, &p.PartyB, &p.BankName, &p.IdempotencyKey, &p.RequestHash, &p.Status, &p.Amount,
		&p.Phone, &p.AccountReference, &p.Description, &p.Metadata, &p.CallbackToken, &p.MerchantRequestID,
		&p.CheckoutRequestID, &p.MpesaReceipt, &p.ResultCode, &p.ResultDesc, &p.FailureReason, &p.CheckAttempts,
		&p.NextCheckAt, &p.Version, &p.CreatedAt, &p.UpdatedAt, &p.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, app.ErrNotFound
	}
	return &p, err
}

func (r *Repo) Create(ctx context.Context, p *domain.Payment) error {
	md := p.Metadata
	if md == nil {
		md = map[string]string{}
	}
	// Sent as JSON text with an explicit cast so it also works over
	// PgBouncer's simple protocol, where pgx cannot infer jsonb.
	mdJSON, err := json.Marshal(md)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `INSERT INTO payment.payments (`+cols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14::jsonb,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27)`,
		p.ID, p.TenantID, p.ChannelID, p.DestinationType, p.PartyB, p.BankName, p.IdempotencyKey, p.RequestHash, p.Status, p.Amount, p.Phone,
		p.AccountReference, p.Description, string(mdJSON), p.CallbackToken, p.MerchantRequestID, p.CheckoutRequestID,
		p.MpesaReceipt, p.ResultCode, p.ResultDesc, p.FailureReason, p.CheckAttempts, p.NextCheckAt, p.Version,
		p.CreatedAt, p.UpdatedAt, p.CompletedAt)
	if db.IsUniqueViolation(err, "payments_idempotency_key") {
		return app.ErrDuplicateKey
	}
	return err
}

func (r *Repo) Get(ctx context.Context, tenantID, id string) (*domain.Payment, error) {
	return scan(r.pool.QueryRow(ctx, `SELECT `+cols+` FROM payment.payments WHERE tenant_id = $1 AND id = $2`, tenantID, id))
}

func (r *Repo) GetByIdempotencyKey(ctx context.Context, tenantID, key string) (*domain.Payment, error) {
	return scan(r.pool.QueryRow(ctx, `SELECT `+cols+` FROM payment.payments WHERE tenant_id = $1 AND idempotency_key = $2`, tenantID, key))
}

func (r *Repo) GetByCallbackToken(ctx context.Context, token string) (*domain.Payment, error) {
	return scan(r.pool.QueryRow(ctx, `SELECT `+cols+` FROM payment.payments WHERE callback_token = $1`, token))
}

// Save writes p if its version is unchanged, bumping the version, and
// enqueues events in the same transaction.
func (r *Repo) Save(ctx context.Context, p *domain.Payment, events []domain.Event) error {
	err := db.InTx(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE payment.payments SET status=$3, merchant_request_id=$4, checkout_request_id=$5,
			mpesa_receipt=$6, result_code=$7, result_desc=$8, failure_reason=$9, check_attempts=$10, next_check_at=$11,
			updated_at=$12, completed_at=$13, version = version + 1
			WHERE id = $1 AND version = $2`,
			p.ID, p.Version, p.Status, p.MerchantRequestID, p.CheckoutRequestID, p.MpesaReceipt, p.ResultCode,
			p.ResultDesc, p.FailureReason, p.CheckAttempts, p.NextCheckAt, p.UpdatedAt, p.CompletedAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return app.ErrVersionConflict
		}
		msgs := make([]outbox.Message, len(events))
		for i, e := range events {
			msgs[i] = outbox.Message{
				Topic: contracts.TopicPaymentEvents,
				Key:   p.ID,
				Payload: contracts.PaymentEvent{
					ID: ids.New("evt"), Type: string(e.Type), TenantID: p.TenantID, CreatedAt: e.OccurredAt, Data: app.View(p),
				},
			}
		}
		return outbox.Insert(ctx, tx, msgs...)
	})
	if err == nil {
		p.Version++
	}
	return err
}

func (r *Repo) List(ctx context.Context, f app.ListFilter) ([]*domain.Payment, error) {
	where := []string{"tenant_id = $1"}
	args := []any{f.TenantID}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.ChannelID != "" {
		add("channel_id = $%d", f.ChannelID)
	}
	if f.Phone != "" {
		add("phone = $%d", f.Phone)
	}
	if f.Before != "" {
		add("id < $%d", f.Before)
	}
	args = append(args, f.Limit)
	q := fmt.Sprintf(`SELECT %s FROM payment.payments WHERE %s ORDER BY id DESC LIMIT $%d`, cols, strings.Join(where, " AND "), len(args))
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Payment
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) ClaimDueChecks(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]*domain.Payment, error) {
	rows, err := r.pool.Query(ctx, `UPDATE payment.payments SET next_check_at = $2
		WHERE id IN (
			SELECT id FROM payment.payments
			WHERE status IN ('created', 'pending') AND next_check_at <= $1
			ORDER BY next_check_at LIMIT $3 FOR UPDATE SKIP LOCKED)
		RETURNING `+cols, now, now.Add(lease), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Payment
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
