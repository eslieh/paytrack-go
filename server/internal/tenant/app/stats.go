package app

import (
	"context"
	"time"
)

// Stats counts tenants for the staff overview.
type Stats struct {
	Total     int `json:"total"`
	Suspended int `json:"suspended"`
	New       int `json:"new"` // created in the window
}

func (s *Service) Stats(ctx context.Context, since time.Time) (*Stats, error) {
	return s.repo.TenantStats(ctx, since)
}
