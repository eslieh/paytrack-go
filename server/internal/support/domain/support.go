// Package domain holds the support module's entities and rules: tickets,
// their message threads (with staff-only internal notes), satisfaction
// ratings and product feedback.
package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
)

type Status string

const (
	StatusAwaitingStaff Status = "awaiting_staff" // new, or the customer replied
	StatusAwaitingUser  Status = "awaiting_user"  // staff replied
	StatusResolved      Status = "resolved"       // a customer reply reopens it
	StatusClosed        Status = "closed"         // final; the customer opens a new ticket
)

func (s Status) Valid() bool {
	switch s {
	case StatusAwaitingStaff, StatusAwaitingUser, StatusResolved, StatusClosed:
		return true
	}
	return false
}

type Category string

var Categories = []Category{"payments", "channels", "webhooks", "api", "account", "billing", "other"}

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}

type Ticket struct {
	ID              string
	Number          int64
	TenantID        string
	UserID          string
	RequesterEmail  string
	TenantName      string
	Subject         string
	Category        Category
	Priority        Priority
	Status          Status
	FirstResponseAt *time.Time
	LastMessageAt   time.Time
	ResolvedAt      *time.Time
	Rating          *int
	RatingComment   string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AuthorKind string

const (
	AuthorUser  AuthorKind = "user"
	AuthorStaff AuthorKind = "staff"
)

// StaffDisplayName is how staff replies are signed to customers.
const StaffDisplayName = "Paytrack Support"

type Message struct {
	ID         string
	TicketID   string
	AuthorKind AuthorKind
	AuthorID   string
	AuthorName string
	Body       string
	Internal   bool
	CreatedAt  time.Time
}

type NewTicket struct {
	Subject  string
	Category string
	Body     string
}

func (n NewTicket) Validate() (subject string, cat Category, body string, err error) {
	subject = strings.TrimSpace(n.Subject)
	if l := utf8.RuneCountInString(subject); l < 3 || l > 150 {
		return "", "", "", errs.Invalid("invalid_subject", "subject must be 3-150 characters")
	}
	cat = Category(strings.TrimSpace(n.Category))
	if cat == "" {
		cat = "other"
	}
	known := false
	for _, c := range Categories {
		known = known || c == cat
	}
	if !known {
		return "", "", "", errs.Invalid("invalid_category", "unknown category")
	}
	body, err = ValidateBody(n.Body)
	return subject, cat, body, err
}

func ValidateBody(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > 10_000 {
		return "", errs.Invalid("invalid_body", "message must be 1-10000 characters")
	}
	return s, nil
}

// UserReplied applies a customer reply: it reopens a resolved ticket and
// puts it back in the staff queue. Closed tickets take no more replies.
func (t *Ticket) UserReplied(at time.Time) error {
	if t.Status == StatusClosed {
		return errs.Conflict("ticket_closed", "this ticket is closed; open a new one")
	}
	t.Status, t.ResolvedAt, t.LastMessageAt, t.UpdatedAt = StatusAwaitingStaff, nil, at, at
	return nil
}

// StaffReplied applies a public staff reply. Internal notes change nothing
// the customer can see, so they leave the status alone.
func (t *Ticket) StaffReplied(internal bool, at time.Time) {
	t.UpdatedAt = at
	if internal {
		return
	}
	if t.FirstResponseAt == nil {
		t.FirstResponseAt = &at
	}
	t.LastMessageAt = at
	if t.Status != StatusClosed {
		t.Status, t.ResolvedAt = StatusAwaitingUser, nil
	}
}

// SetStatus is a staff status change.
func (t *Ticket) SetStatus(s Status, at time.Time) error {
	if !s.Valid() {
		return errs.Invalid("invalid_status", "unknown status")
	}
	if s == t.Status {
		return nil
	}
	t.Status, t.UpdatedAt = s, at
	switch s {
	case StatusResolved:
		t.ResolvedAt = &at
	case StatusAwaitingStaff, StatusAwaitingUser:
		t.ResolvedAt = nil
	}
	return nil
}

func (t *Ticket) SetPriority(p Priority, at time.Time) error {
	if !p.Valid() {
		return errs.Invalid("invalid_priority", "unknown priority")
	}
	t.Priority, t.UpdatedAt = p, at
	return nil
}

// Rate records the customer's satisfaction rating on a resolved ticket.
func (t *Ticket) Rate(rating int, comment string, at time.Time) error {
	if t.Status != StatusResolved && t.Status != StatusClosed {
		return errs.Conflict("ticket_not_resolved", "you can rate a ticket once it is resolved")
	}
	if rating < 1 || rating > 5 {
		return errs.Invalid("invalid_rating", "rating must be 1-5")
	}
	comment = strings.TrimSpace(comment)
	if utf8.RuneCountInString(comment) > 1000 {
		return errs.Invalid("invalid_comment", "comment must be at most 1000 characters")
	}
	t.Rating, t.RatingComment, t.UpdatedAt = &rating, comment, at
	return nil
}

// Excerpt is the email preview of a message: whitespace collapsed and cut
// at n characters. Emails carry an excerpt and a link, never the full text.
func Excerpt(body string, n int) string {
	s := strings.Join(strings.Fields(body), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)[:n]
	if i := strings.LastIndexByte(string(r), ' '); i > n/2 {
		return string(r)[:i] + "…"
	}
	return string(r) + "…"
}

// Feedback is free-form product feedback from a dashboard user.
type Feedback struct {
	ID        string
	TenantID  string
	UserID    string
	UserEmail string
	Rating    *int
	Message   string
	Page      string
	CreatedAt time.Time
}

func (f *Feedback) Validate() error {
	f.Message = strings.TrimSpace(f.Message)
	if f.Message == "" || utf8.RuneCountInString(f.Message) > 5000 {
		return errs.Invalid("invalid_message", "feedback must be 1-5000 characters")
	}
	if f.Rating != nil && (*f.Rating < 1 || *f.Rating > 5) {
		return errs.Invalid("invalid_rating", "rating must be 1-5")
	}
	if len(f.Page) > 200 {
		f.Page = f.Page[:200]
	}
	return nil
}
