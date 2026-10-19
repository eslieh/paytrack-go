package app

import (
	"context"
	"errors"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/support/domain"
)

var ErrNotFound = errors.New("not found")

// Repository writes are transactional: the notifications passed in are
// published through the outbox in the same transaction as the change.
type Repository interface {
	SupportStats(ctx context.Context, since time.Time) (*Stats, error)
	// CreateTicket stores the ticket and its first message and sets t.Number.
	// The notifications need that number, so they are built by build.
	CreateTicket(ctx context.Context, t *domain.Ticket, first *domain.Message, build func() []contracts.Notification) error
	GetTicket(ctx context.Context, id string) (*domain.Ticket, error)
	ListTickets(ctx context.Context, f TicketFilter) ([]*domain.Ticket, error)
	SaveTicket(ctx context.Context, t *domain.Ticket, out []contracts.Notification) error
	// AddMessage stores the message and the updated ticket together.
	AddMessage(ctx context.Context, t *domain.Ticket, m *domain.Message, out []contracts.Notification) error
	ListMessages(ctx context.Context, ticketID string, includeInternal bool) ([]*domain.Message, error)
	CountByStatus(ctx context.Context) (map[domain.Status]int, error)

	CreateFeedback(ctx context.Context, f *domain.Feedback, out []contracts.Notification) error
	ListFeedback(ctx context.Context, before string, limit int) ([]*domain.Feedback, error)
}

// TicketFilter narrows ticket lists; zero fields match everything.
type TicketFilter struct {
	TenantID string
	Status   domain.Status
	Before   string // id cursor
	Limit    int
}

// Accounts is the identity and tenant modules, seen from support.
type Accounts interface {
	UserContact(ctx context.Context, userID string) (email, tenantID string, err error)
	TenantName(ctx context.Context, tenantID string) (string, error)
}

// Staff is the admin module, seen from support: who to alert.
type Staff interface {
	StaffEmails(ctx context.Context) ([]string, error)
}

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}
