package app

import (
	"context"
	"time"
)

// Stats counts users for the staff overview.
type Stats struct {
	Total    int `json:"total"`
	Verified int `json:"verified"`
	Disabled int `json:"disabled"`
	Active   int `json:"active"` // signed in during the window
}

func (s *Service) Stats(ctx context.Context, since time.Time) (*Stats, error) {
	return s.repo.UserStats(ctx, since)
}
