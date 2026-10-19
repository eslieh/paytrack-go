package app

import (
	"context"
	"time"
)

// Stats is the staff overview of payment volume over a window.
type Stats struct {
	RoutedAmount  int64          `json:"routed_amount"`   // succeeded payments in the window, KES
	RoutedAllTime int64          `json:"routed_all_time"` // KES
	ActiveTenants int            `json:"active_tenants"`  // tenants with a succeeded payment in the window
	Counts        map[string]int `json:"counts"`          // by status
	ByDestination []DestStat     `json:"by_destination"`
	Daily         []DailyStat    `json:"daily"` // only days with payments; dates in Nairobi time
	TopFailures   []ReasonCount  `json:"top_failures"`
}

type DestStat struct {
	Type   string `json:"type"`
	Amount int64  `json:"amount"`
	Count  int    `json:"count"`
}

type DailyStat struct {
	Date      string `json:"date"` // YYYY-MM-DD
	Amount    int64  `json:"amount"`
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"` // failed, cancelled or expired
}

type ReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

func (s *Service) Stats(ctx context.Context, since time.Time) (*Stats, error) {
	return s.repo.PaymentStats(ctx, since)
}
