// Package postgres implements the callback Inbox.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/eslieh/paytrack-go/server/internal/callback/app"
	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/platform/db"
	"github.com/eslieh/paytrack-go/server/internal/platform/outbox"
)

type Inbox struct{ pool *pgxpool.Pool }

var _ app.Inbox = (*Inbox)(nil)

func New(pool *pgxpool.Pool) *Inbox { return &Inbox{pool: pool} }

func (i *Inbox) Store(ctx context.Context, rec app.Record, msg *contracts.STKCallback) error {
	return db.InTx(ctx, i.pool, func(tx pgx.Tx) error {
		var id int64
		if err := tx.QueryRow(ctx, `INSERT INTO callback.inbox
			(callback_token, checkout_request_id, raw, parse_error, remote_addr, received_at)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			rec.CallbackToken, rec.CheckoutRequestID, rec.Raw, rec.ParseError, rec.RemoteAddr, rec.ReceivedAt).Scan(&id); err != nil {
			return err
		}
		if msg == nil {
			return nil
		}
		msg.InboxID = id
		return outbox.Insert(ctx, tx, outbox.Message{
			Topic: contracts.TopicSTKCallbacks, Key: msg.CheckoutRequestID, Payload: msg,
		})
	})
}
