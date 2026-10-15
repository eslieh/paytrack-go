// Package outbox implements the transactional outbox: modules write
// integration events into the outbox table in the same transaction as their
// state change, and the Relay publishes them to Kafka afterwards. This keeps
// the API and callback roles independent of Kafka availability.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Message struct {
	Topic   string
	Key     string
	Payload any
}

// Insert enqueues messages within tx.
func Insert(ctx context.Context, tx pgx.Tx, msgs ...Message) error {
	for _, m := range msgs {
		b, err := json.Marshal(m.Payload)
		if err != nil {
			return fmt.Errorf("outbox: marshal %s: %w", m.Topic, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO outbox (topic, key, payload) VALUES ($1, $2, $3::jsonb)`, m.Topic, m.Key, string(b)); err != nil {
			return fmt.Errorf("outbox: insert: %w", err)
		}
	}
	return nil
}

type Record struct {
	Topic string
	Key   []byte
	Value []byte
}

type Publisher interface {
	Publish(ctx context.Context, recs []Record) error
}

type Relay struct {
	Pool      *pgxpool.Pool
	Publisher Publisher
	BatchSize int
}

// RelayOnce publishes one batch. Rows are locked with SKIP LOCKED so many
// worker replicas can relay concurrently. Delivery is at-least-once: if the
// commit fails after publishing, the batch is published again.
func (r *Relay) RelayOnce(ctx context.Context) (more bool, err error) {
	batch := r.BatchSize
	if batch == 0 {
		batch = 200
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	rows, err := tx.Query(ctx, `SELECT id, topic, key, payload::text FROM outbox
		WHERE published_at IS NULL ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED`, batch)
	if err != nil {
		return false, err
	}
	var ids []int64
	var recs []Record
	for rows.Next() {
		var id int64
		var rec Record
		var key, value string
		if err := rows.Scan(&id, &rec.Topic, &key, &value); err != nil {
			rows.Close()
			return false, err
		}
		rec.Key, rec.Value = []byte(key), []byte(value)
		ids = append(ids, id)
		recs = append(recs, rec)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, err
	}
	if len(recs) == 0 {
		return false, nil
	}

	pubCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := r.Publisher.Publish(pubCtx, recs); err != nil {
		return false, fmt.Errorf("outbox: publish: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at = now() WHERE id = ANY($1)`, ids); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return len(recs) == batch, nil
}

// Prune deletes published rows older than keep.
func (r *Relay) Prune(ctx context.Context, keep time.Duration) error {
	_, err := r.Pool.Exec(ctx, `DELETE FROM outbox WHERE published_at < $1`, time.Now().Add(-keep))
	return err
}
