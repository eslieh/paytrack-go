package postgres

import (
	"context"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/payment/app"
)

// Dates are bucketed in Nairobi time.
const day = `to_char(created_at AT TIME ZONE 'Africa/Nairobi', 'YYYY-MM-DD')`

func (r *Repo) PaymentStats(ctx context.Context, since time.Time) (*app.Stats, error) {
	st := &app.Stats{Counts: map[string]int{}, ByDestination: []app.DestStat{}, Daily: []app.DailyStat{}, TopFailures: []app.ReasonCount{}}
	err := r.pool.QueryRow(ctx, `SELECT
			coalesce(sum(amount) FILTER (WHERE created_at >= $1), 0),
			coalesce(sum(amount), 0),
			count(DISTINCT tenant_id) FILTER (WHERE created_at >= $1)
		FROM payment.payments WHERE status = 'succeeded'`, since).Scan(&st.RoutedAmount, &st.RoutedAllTime, &st.ActiveTenants)
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `SELECT status, count(*) FROM payment.payments WHERE created_at >= $1 GROUP BY 1`, since)
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

	rows, err = r.pool.Query(ctx, `SELECT destination_type, sum(amount), count(*) FROM payment.payments
		WHERE status = 'succeeded' AND created_at >= $1 GROUP BY 1 ORDER BY 2 DESC`, since)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var d app.DestStat
		if err := rows.Scan(&d.Type, &d.Amount, &d.Count); err != nil {
			rows.Close()
			return nil, err
		}
		st.ByDestination = append(st.ByDestination, d)
	}
	rows.Close()

	rows, err = r.pool.Query(ctx, `SELECT `+day+`,
			coalesce(sum(amount) FILTER (WHERE status = 'succeeded'), 0),
			count(*) FILTER (WHERE status = 'succeeded'),
			count(*) FILTER (WHERE status IN ('failed', 'cancelled', 'expired'))
		FROM payment.payments WHERE created_at >= $1 GROUP BY 1 ORDER BY 1`, since)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var d app.DailyStat
		if err := rows.Scan(&d.Date, &d.Amount, &d.Succeeded, &d.Failed); err != nil {
			rows.Close()
			return nil, err
		}
		st.Daily = append(st.Daily, d)
	}
	rows.Close()

	rows, err = r.pool.Query(ctx, `SELECT coalesce(nullif(failure_reason, ''), nullif(result_desc, ''), status), count(*)
		FROM payment.payments WHERE status IN ('failed', 'cancelled', 'expired') AND created_at >= $1
		GROUP BY 1 ORDER BY 2 DESC LIMIT 5`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rc app.ReasonCount
		if err := rows.Scan(&rc.Reason, &rc.Count); err != nil {
			return nil, err
		}
		st.TopFailures = append(st.TopFailures, rc)
	}
	return st, rows.Err()
}
