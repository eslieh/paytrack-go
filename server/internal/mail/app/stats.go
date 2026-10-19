package app

import (
	"context"
	"time"
)

// Stats is the staff overview of transactional email over a window.
type Stats struct {
	Counts     map[string]int  `json:"counts"` // by status
	ByTemplate []TemplateCount `json:"by_template"`
}

type TemplateCount struct {
	Template string `json:"template"`
	Sent     int    `json:"sent"`
	Failed   int    `json:"failed"`
}

func (s *Service) Stats(ctx context.Context, since time.Time) (*Stats, error) {
	return s.repo.MailStats(ctx, since)
}
