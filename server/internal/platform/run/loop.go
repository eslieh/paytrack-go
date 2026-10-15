// Package run provides helpers for background workers.
package run

import (
	"context"
	"log/slog"
	"time"
)

// Poll calls fn every interval until ctx is cancelled. When fn reports that
// it did a full batch of work, it is called again immediately to drain the
// backlog. Every replica can run the same loop: callers claim work with
// SELECT ... FOR UPDATE SKIP LOCKED, so no leader election is needed.
func Poll(ctx context.Context, name string, interval time.Duration, fn func(context.Context) (more bool, err error)) error {
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
		more, err := fn(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("poll loop error", "loop", name, "err", err)
		}
		if more && err == nil {
			t.Reset(0)
		} else {
			t.Reset(interval)
		}
	}
}
