// Package ratelimit is a fixed-window rate limiter stored in Postgres, so
// limits hold across every API replica.
package ratelimit

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Limiter struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Limiter { return &Limiter{pool: pool} }

// Allow counts one hit against key and reports whether it is within limit
// for the current window.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	start := time.Now().Truncate(window)
	var n int
	err := l.pool.QueryRow(ctx, `INSERT INTO rate_limits (key, window_start, count) VALUES ($1, $2, 1)
		ON CONFLICT (key, window_start) DO UPDATE SET count = rate_limits.count + 1
		RETURNING count`, key, start).Scan(&n)
	if err != nil {
		return false, err
	}
	return n <= limit, nil
}

// Prune deletes windows older than a day.
func (l *Limiter) Prune(ctx context.Context) error {
	_, err := l.pool.Exec(ctx, `DELETE FROM rate_limits WHERE window_start < now() - interval '1 day'`)
	return err
}

// Reset clears every window of the given keys (support unlocking an account).
func (l *Limiter) Reset(ctx context.Context, keys ...string) error {
	_, err := l.pool.Exec(ctx, `DELETE FROM rate_limits WHERE key = ANY($1)`, keys)
	return err
}
