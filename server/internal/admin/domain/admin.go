// Package domain holds the admin module's entities: platform staff, their
// sign-in challenges and sessions, and the audit log of staff actions.
// Staff are not tenant users and never authenticate on tenant routes.
package domain

import (
	"strings"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
)

type Staff struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	LastLoginAt  *time.Time
	DisabledAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (s *Staff) Disabled() bool { return s.DisabledAt != nil }

// Lifetimes and limits.
const (
	ChallengeTTL       = 10 * time.Minute
	MaxCodeAttempts    = 5
	SessionIdleTimeout = 30 * time.Minute
	SessionMaxAge      = 12 * time.Hour
	// SessionTouchEvery limits last_seen_at writes to one per minute.
	SessionTouchEvery = time.Minute
)

// Challenge is the emailed-code step every staff sign-in goes through.
type Challenge struct {
	ID         string
	StaffID    string
	CodeHash   []byte
	Attempts   int
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}

func (c *Challenge) Usable(now time.Time) error {
	switch {
	case c.ConsumedAt != nil:
		return errs.Invalid("code_used", "this code has already been used; sign in again")
	case !now.Before(c.ExpiresAt):
		return errs.Invalid("code_expired", "this code has expired; sign in again")
	case c.Attempts >= MaxCodeAttempts:
		return errs.Invalid("too_many_attempts", "too many incorrect codes; sign in again")
	}
	return nil
}

// Session is an opaque cookie session. It ends after SessionIdleTimeout
// without activity or SessionMaxAge in total, whichever comes first.
type Session struct {
	ID         string
	StaffID    string
	TokenHash  []byte
	IP         string
	UserAgent  string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
}

func (s *Session) Active(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAt) && now.Sub(s.LastSeenAt) < SessionIdleTimeout
}

// AuditEntry records one staff action. Every action that changes a tenant
// or user is audited with the reason the staff member gave.
type AuditEntry struct {
	ID         int64
	StaffID    string
	Action     string
	TargetType string
	TargetID   string
	Reason     string
	Data       map[string]string
	IP         string
	CreatedAt  time.Time
}

const (
	TargetTenant = "tenant"
	TargetUser   = "user"
	TargetStaff  = "staff"
)

const (
	ActionSignIn          = "staff.sign_in"
	ActionTenantSuspend   = "tenant.suspend"
	ActionTenantReinstate = "tenant.reinstate"
	ActionUserDisable     = "user.disable"
	ActionUserEnable      = "user.enable"
)

// ValidateReason requires a short explanation for every action that changes
// a customer's account. It is shown in the audit log and, for suspensions,
// to the customer.
func ValidateReason(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) < 5 || len(s) > 500 {
		return "", errs.Invalid("invalid_reason", "reason must be 5-500 characters")
	}
	return s, nil
}
