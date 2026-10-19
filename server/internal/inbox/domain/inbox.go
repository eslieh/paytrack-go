// Package domain holds the inbox module's entity: an in-app notification
// shown under the dashboard's bell.
package domain

import "time"

type Notification struct {
	ID        string
	TenantID  string
	UserID    string // empty: everyone in the tenant
	Kind      string
	Title     string
	Body      string
	Link      string
	ReadAt    *time.Time
	CreatedAt time.Time
}

// Lengths are capped so a bad publisher can't bloat the bell.
const (
	MaxTitle = 200
	MaxBody  = 500
)

func Clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
