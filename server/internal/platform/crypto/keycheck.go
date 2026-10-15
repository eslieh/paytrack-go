package crypto

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const canaryPlain = "paytrack-encryption-key-check"

var ErrKeyMismatch = errors.New("ENCRYPTION_KEY does not match the key this database was initialised with; " +
	"use the original key (it cannot be changed once data exists)")

// VerifyKey makes sure c is the key the database's encrypted data was
// written with. On first run it records a canary, but only after confirming
// any pre-existing encrypted rows decrypt with c.
func VerifyKey(ctx context.Context, pool *pgxpool.Pool, c *Cipher) error {
	var canary []byte
	err := pool.QueryRow(ctx, `SELECT canary FROM encryption_key_check WHERE id = 1`).Scan(&canary)
	if errors.Is(err, pgx.ErrNoRows) {
		var sample []byte
		err := pool.QueryRow(ctx, `SELECT webhook_secret_enc FROM tenant.tenants
			WHERE webhook_secret_enc IS NOT NULL LIMIT 1`).Scan(&sample)
		if err == nil {
			if _, err := c.Decrypt(sample); err != nil {
				return ErrKeyMismatch
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("encryption key check: %w", err)
		}
		enc, err := c.Encrypt([]byte(canaryPlain))
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, `INSERT INTO encryption_key_check (id, canary) VALUES (1, $1)
			ON CONFLICT (id) DO NOTHING`, enc); err != nil {
			return fmt.Errorf("encryption key check: %w", err)
		}
		return VerifyKey(ctx, pool, c) // re-read: another process may have won the insert
	}
	if err != nil {
		return fmt.Errorf("encryption key check: %w", err)
	}
	if p, err := c.Decrypt(canary); err != nil || string(p) != canaryPlain {
		return ErrKeyMismatch
	}
	return nil
}
