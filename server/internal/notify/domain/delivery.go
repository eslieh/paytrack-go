// Package domain models webhook deliveries to tenant backends: retry
// schedule, terminal states and payload signing.
package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusDelivering Status = "delivering" // claimed by a dispatcher (lease)
	StatusDelivered  Status = "delivered"
	StatusFailed     Status = "failed" // retries exhausted; can be redelivered manually
)

// Backoff is the delay after each failed attempt; its length is the number
// of retries (~16h in total).
var Backoff = []time.Duration{
	10 * time.Second, 30 * time.Second, time.Minute, 5 * time.Minute, 15 * time.Minute,
	30 * time.Minute, time.Hour, 2 * time.Hour, 4 * time.Hour, 8 * time.Hour,
}

type Delivery struct {
	ID             string
	EventID        string
	TenantID       string
	PaymentID      string
	EventType      string
	Payload        []byte
	Status         Status
	Attempts       int
	NextAttemptAt  *time.Time
	LastError      string
	LastStatusCode int
	DeliveredAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewDelivery(id, eventID, tenantID, paymentID, eventType string, payload []byte, now time.Time) *Delivery {
	return &Delivery{
		ID: id, EventID: eventID, TenantID: tenantID, PaymentID: paymentID, EventType: eventType,
		Payload: payload, Status: StatusPending, NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now,
	}
}

func (d *Delivery) RecordSuccess(statusCode int, now time.Time) {
	d.Attempts++
	d.Status = StatusDelivered
	d.LastStatusCode, d.LastError = statusCode, ""
	d.DeliveredAt, d.NextAttemptAt = &now, nil
	d.UpdatedAt = now
}

func (d *Delivery) RecordFailure(statusCode int, errMsg string, now time.Time) {
	d.Attempts++
	d.LastStatusCode = statusCode
	if len(errMsg) > 500 {
		errMsg = errMsg[:500]
	}
	d.LastError = errMsg
	d.UpdatedAt = now
	if d.Attempts > len(Backoff) {
		d.Status, d.NextAttemptAt = StatusFailed, nil
		return
	}
	next := now.Add(Backoff[d.Attempts-1])
	d.Status, d.NextAttemptAt = StatusPending, &next
}

// Redeliver queues the delivery again with a fresh retry budget.
func (d *Delivery) Redeliver(now time.Time) {
	d.Status = StatusPending
	d.Attempts = 0
	d.NextAttemptAt = &now
	d.UpdatedAt = now
}

const SignatureHeader = "PayTrack-Signature"

// Sign produces the PayTrack-Signature header value:
//
//	t=<unix seconds>,v1=<hex HMAC-SHA256(secret, "<t>.<body>")>
//
// Receivers recompute v1 and reject timestamps older than a few minutes to
// prevent replay.
func Sign(secret string, ts time.Time, body []byte) string {
	t := strconv.FormatInt(ts.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(t))
	mac.Write([]byte("."))
	mac.Write(body)
	return fmt.Sprintf("t=%s,v1=%s", t, hex.EncodeToString(mac.Sum(nil)))
}
