package postgres

import (
	"context"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/mail/app"
)

func (r *Repo) MailStats(ctx context.Context, since time.Time) (*app.Stats, error) {
	st := &app.Stats{Counts: map[string]int{}, ByTemplate: []app.TemplateCount{}}
	rows, err := r.pool.Query(ctx, `SELECT status, count(*) FROM mail.messages WHERE created_at >= $1 GROUP BY 1`, since)
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
	rows, err = r.pool.Query(ctx, `SELECT template, count(*) FILTER (WHERE status = 'sent'), count(*) FILTER (WHERE status = 'failed')
		FROM mail.messages WHERE created_at >= $1 GROUP BY 1 ORDER BY 2 DESC, 1`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t app.TemplateCount
		if err := rows.Scan(&t.Template, &t.Sent, &t.Failed); err != nil {
			return nil, err
		}
		st.ByTemplate = append(st.ByTemplate, t)
	}
	return st, rows.Err()
}
