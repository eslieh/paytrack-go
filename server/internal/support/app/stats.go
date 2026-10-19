package app

import (
	"context"
	"time"
)

// Stats is the staff overview of support over a window.
type Stats struct {
	Opened                  int      `json:"opened"`         // tickets created in the window
	AwaitingStaff           int      `json:"awaiting_staff"` // right now
	AvgFirstResponseSeconds *float64 `json:"avg_first_response_seconds"`
	AvgRating               *float64 `json:"avg_rating"`
	Ratings                 int      `json:"ratings"`
	Feedback                int      `json:"feedback"` // received in the window
}

func (s *Service) Stats(ctx context.Context, since time.Time) (*Stats, error) {
	return s.repo.SupportStats(ctx, since)
}
