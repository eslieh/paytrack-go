package postgres

import (
	"context"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/support/app"
)

func (r *Repo) SupportStats(ctx context.Context, since time.Time) (*app.Stats, error) {
	var st app.Stats
	err := r.pool.QueryRow(ctx, `SELECT
			count(*) FILTER (WHERE created_at >= $1),
			count(*) FILTER (WHERE status = 'awaiting_staff'),
			avg(extract(epoch FROM first_response_at - created_at)) FILTER (WHERE created_at >= $1 AND first_response_at IS NOT NULL),
			avg(rating) FILTER (WHERE created_at >= $1),
			count(rating) FILTER (WHERE created_at >= $1),
			(SELECT count(*) FROM support.feedback WHERE created_at >= $1)
		FROM support.tickets`, since).
		Scan(&st.Opened, &st.AwaitingStaff, &st.AvgFirstResponseSeconds, &st.AvgRating, &st.Ratings, &st.Feedback)
	return &st, err
}
