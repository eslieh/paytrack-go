package postgres

import (
	"context"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/notify/app"
)

func (r *Repo) DeliveryStats(ctx context.Context, since time.Time) (*app.Stats, error) {
	st := &app.Stats{Counts: map[string]int{}, Daily: []app.DailyStat{}}
	rows, err := r.pool.Query(ctx, `SELECT status, count(*) FROM notify.deliveries WHERE created_at >= $1 GROUP BY 1`, since)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			rows.Close()
			return nil, err
		}
		st.Counts[s] = n
	}
	rows.Close()
	if err := r.pool.QueryRow(ctx, `SELECT count(DISTINCT tenant_id) FROM notify.deliveries
		WHERE status = 'failed' AND created_at >= $1`, since).Scan(&st.FailingTenants); err != nil {
		return nil, err
	}
	rows, err = r.pool.Query(ctx, `SELECT to_char(created_at AT TIME ZONE 'Africa/Nairobi', 'YYYY-MM-DD'),
			count(*) FILTER (WHERE status = 'delivered'), count(*) FILTER (WHERE status = 'failed')
		FROM notify.deliveries WHERE created_at >= $1 GROUP BY 1 ORDER BY 1`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d app.DailyStat
		if err := rows.Scan(&d.Date, &d.Delivered, &d.Failed); err != nil {
			return nil, err
		}
		st.Daily = append(st.Daily, d)
	}
	return st, rows.Err()
}
