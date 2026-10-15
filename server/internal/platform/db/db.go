// Package db wraps the Postgres pool, transactions and schema migrations.
package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parsing DATABASE_URL: %w", err)
	}
	cfg.MaxConnIdleTime = 5 * time.Minute
	// PgBouncer in transaction-pooling mode (conventionally port 6432) cannot
	// keep prepared statements across transactions. Use the simple protocol
	// there, or force it with DB_SIMPLE_PROTOCOL=true.
	if cfg.ConnConfig.Port == 6432 || os.Getenv("DB_SIMPLE_PROTOCOL") == "true" {
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}
	return pool, nil
}

// InTx runs fn inside a transaction, committing on nil error.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// IsUniqueViolation reports a unique-constraint error, optionally for a
// specific constraint name.
func IsUniqueViolation(err error, constraint string) bool {
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23505" {
		return false
	}
	return constraint == "" || pg.ConstraintName == constraint
}

// Migrate applies every *.sql file in fsys not yet recorded in
// schema_migrations, in filename order. Each migration runs in its own
// transaction holding a transaction-scoped advisory lock, so concurrent runs
// are safe, including through PgBouncer's transaction pooling.
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		sql, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		applied := false
		err = InTx(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(727274)`); err != nil {
				return err
			}
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists); err != nil || exists {
				return err
			}
			if _, err := tx.Exec(ctx, string(sql)); err != nil {
				return fmt.Errorf("migration %s: %w", name, err)
			}
			applied = true
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version)
			return err
		})
		if err != nil {
			return err
		}
		if applied {
			slog.Info("migration applied", "version", version)
		}
	}
	return nil
}
