// Package domain models a transactional email in the send queue.
package domain

import "time"

type Status string

const (
	StatusQueued  Status = "queued"
	StatusSending Status = "sending" // claimed by a dispatcher (lease)
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed" // permanent provider error or retries exhausted
)

// Backoff is the delay after each failed attempt.
var Backoff = []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, 2 * time.Hour}

type Message struct {
	ID                string
	DedupeKey         string
	Template          string
	Category          string
	ToEmail           string
	ToName            string
	Data              map[string]any
	Status            Status
	Attempts          int
	NextAttemptAt     *time.Time
	ProviderMessageID string
	LastError         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	SentAt            *time.Time
}

func (m *Message) RecordSent(providerID string, now time.Time) {
	m.Attempts++
	m.Status, m.ProviderMessageID, m.LastError = StatusSent, providerID, ""
	m.SentAt, m.NextAttemptAt, m.UpdatedAt = &now, nil, now
}

// RecordFailure schedules a retry, or fails the message when the error is
// permanent or retries are exhausted.
func (m *Message) RecordFailure(errMsg string, permanent bool, now time.Time) {
	m.Attempts++
	if len(errMsg) > 500 {
		errMsg = errMsg[:500]
	}
	m.LastError, m.UpdatedAt = errMsg, now
	if permanent || m.Attempts > len(Backoff) {
		m.Status, m.NextAttemptAt = StatusFailed, nil
		return
	}
	next := now.Add(Backoff[m.Attempts-1])
	m.Status, m.NextAttemptAt = StatusQueued, &next
}
