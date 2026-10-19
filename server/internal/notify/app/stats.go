package app

import (
	"context"
	"time"
)

// Stats is the staff overview of webhook deliveries over a window.
type Stats struct {
	Counts         map[string]int `json:"counts"`          // by status
	FailingTenants int            `json:"failing_tenants"` // tenants with an exhausted delivery
	Daily          []DailyStat    `json:"daily"`
}

type DailyStat struct {
	Date      string `json:"date"`
	Delivered int    `json:"delivered"`
	Failed    int    `json:"failed"`
}

func (s *Service) Stats(ctx context.Context, since time.Time) (*Stats, error) {
	return s.repo.DeliveryStats(ctx, since)
}
