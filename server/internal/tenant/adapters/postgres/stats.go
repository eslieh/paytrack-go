package postgres

import (
	"context"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/tenant/app"
)

func (r *Repo) TenantStats(ctx context.Context, since time.Time) (*app.Stats, error) {
	var st app.Stats
	err := r.pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE status = 'suspended'),
		count(*) FILTER (WHERE created_at >= $1) FROM tenant.tenants`, since).Scan(&st.Total, &st.Suspended, &st.New)
	return &st, err
}
