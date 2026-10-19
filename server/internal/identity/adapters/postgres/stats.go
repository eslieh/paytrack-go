package postgres

import (
	"context"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/identity/app"
)

func (r *Repo) UserStats(ctx context.Context, since time.Time) (*app.Stats, error) {
	var st app.Stats
	err := r.pool.QueryRow(ctx, `SELECT count(*), count(email_verified_at), count(disabled_at),
		count(*) FILTER (WHERE last_login_at >= $1) FROM identity.users`, since).Scan(&st.Total, &st.Verified, &st.Disabled, &st.Active)
	return &st, err
}
