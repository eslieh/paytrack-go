// Package domain holds the Payment aggregate and its state machine. It has
// no dependencies on adapters, Kafka, Postgres or Daraja.
package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
)

type Status string

const (
	StatusCreated   Status = "created"   // persisted, STK push not yet acknowledged
	StatusPending   Status = "pending"   // Daraja accepted the push; waiting for the customer
	StatusSucceeded Status = "succeeded" // terminal
	StatusFailed    Status = "failed"    // terminal
	StatusCancelled Status = "cancelled" // terminal: customer dismissed the prompt
	// StatusExpired means we never learned the outcome. It is terminal for
	// failures, but a late *successful* result still moves it to succeeded
	// so money received is never reported as lost.
	StatusExpired Status = "expired"
)

func (s Status) Final() bool {
	return s == StatusSucceeded || s == StatusFailed || s == StatusCancelled
}

// Daraja result codes we treat specially; everything non-zero else is a failure.
const (
	ResultSuccess               = 0
	ResultCancelledByUser       = 1032
	MaxAmount             int64 = 250_000 // M-Pesa per-transaction limit (KES)
)

type EventType string

const (
	EventSucceeded EventType = "payment.succeeded"
	EventFailed    EventType = "payment.failed"
	EventCancelled EventType = "payment.cancelled"
	EventExpired   EventType = "payment.expired"
)

// Event records a state transition worth telling the tenant about.
type Event struct {
	Type       EventType
	OccurredAt time.Time
}

// DestinationType mirrors the tenant channel types.
type DestinationType string

const (
	DestinationBank    DestinationType = "bank"
	DestinationPaybill DestinationType = "paybill"
	DestinationTill    DestinationType = "till"
)

// Destination is where a payment's money is routed: the payment module's
// view of a tenant channel, provided by the ChannelProvider port. Every STK
// push is signed with the platform's Daraja app; only PartyB and
// AccountReference vary per destination.
type Destination struct {
	ChannelID     string
	Type          DestinationType
	PartyB        string
	BankName      string
	AccountNumber string // bank account (bank) or default account (paybill)
	Active        bool
}

// TransactionType is the Daraja STK TransactionType for the destination.
func (d DestinationType) TransactionType() string {
	if d == DestinationTill {
		return "CustomerBuyGoodsOnline"
	}
	return "CustomerPayBillOnline"
}

type Payment struct {
	ID        string
	TenantID  string
	ChannelID string
	// Snapshot of the routing at initiation; later channel edits don't
	// change where an existing payment went.
	DestinationType  DestinationType
	PartyB           string
	BankName         string
	IdempotencyKey   string
	RequestHash      string
	Status           Status
	Amount           int64 // whole KES; Daraja does not accept cents
	Phone            string
	AccountReference string
	Description      string
	Metadata         map[string]string

	// CallbackToken is embedded in the callback URL we hand to Daraja, so
	// callbacks are routed by a secret we generated rather than by trusting
	// payload fields.
	CallbackToken     string
	MerchantRequestID string
	CheckoutRequestID string

	MpesaReceipt  string
	ResultCode    *int
	ResultDesc    string
	FailureReason string

	CheckAttempts int
	NextCheckAt   *time.Time
	Version       int

	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
}

type NewParams struct {
	ID, TenantID                string
	Destination                 Destination
	IdempotencyKey, RequestHash string
	CallbackToken               string
	Amount                      int64
	Phone                       string
	AccountReference            string
	Description                 string
	Metadata                    map[string]string
}

var (
	phoneRe      = regexp.MustCompile(`^254(7|1)\d{8}$`)
	nonDigitRe   = regexp.MustCompile(`\D`)
	accountRefRe = regexp.MustCompile(`^[A-Za-z0-9\-_/. ]{1,12}$`)
)

// NormalizePhone accepts 07xx, 7xx, +2547xx, 2547xx (and 01xx variants) and
// returns 2547xxxxxxxx / 2541xxxxxxxx as Daraja requires.
func NormalizePhone(raw string) (string, error) {
	d := nonDigitRe.ReplaceAllString(raw, "")
	switch {
	case len(d) == 9 && (d[0] == '7' || d[0] == '1'):
		d = "254" + d
	case len(d) == 10 && d[0] == '0':
		d = "254" + d[1:]
	}
	if !phoneRe.MatchString(d) {
		return "", errs.Invalid("invalid_phone", "phone must be a Safaricom number, e.g. 0712345678 or 254712345678")
	}
	return d, nil
}

func New(p NewParams, now time.Time) (*Payment, error) {
	if p.Amount < 1 || p.Amount > MaxAmount {
		return nil, errs.Invalid("invalid_amount", "amount must be a whole number of KES between 1 and 250000")
	}
	phone, err := NormalizePhone(p.Phone)
	if err != nil {
		return nil, err
	}
	ref, err := accountReference(p.Destination, strings.TrimSpace(p.AccountReference))
	if err != nil {
		return nil, err
	}
	desc := strings.TrimSpace(p.Description)
	if desc == "" {
		desc = "Payment"
	}
	if len(desc) > 13 { // Daraja TransactionDesc limit
		return nil, errs.Invalid("invalid_description", "description must be at most 13 characters")
	}
	if len(p.Metadata) > 20 {
		return nil, errs.Invalid("invalid_metadata", "metadata may have at most 20 keys")
	}
	for k, v := range p.Metadata {
		if len(k) > 40 || len(v) > 500 {
			return nil, errs.Invalid("invalid_metadata", "metadata keys must be <= 40 chars and values <= 500 chars")
		}
	}
	next := now.Add(createdCheckDelay)
	return &Payment{
		ID: p.ID, TenantID: p.TenantID, ChannelID: p.Destination.ChannelID,
		DestinationType: p.Destination.Type, PartyB: p.Destination.PartyB, BankName: p.Destination.BankName,
		IdempotencyKey: p.IdempotencyKey, RequestHash: p.RequestHash,
		Status: StatusCreated, Amount: p.Amount, Phone: phone, AccountReference: ref,
		Description: desc, Metadata: p.Metadata, CallbackToken: p.CallbackToken,
		NextCheckAt: &next, Version: 1, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// accountReference resolves the STK AccountReference:
//   - bank: always the channel's bank account number; a different value is
//     rejected so a payment can't be redirected to another account.
//   - paybill: the requested value, else the channel default; one is required.
//   - till: the requested value, else "Till Payment" (Buy Goods ignores it).
func accountReference(d Destination, requested string) (string, error) {
	switch d.Type {
	case DestinationBank:
		if requested != "" && requested != d.AccountNumber {
			return "", errs.Invalid("account_reference_mismatch", "bank channels always credit the channel's account_number; omit account_reference")
		}
		requested = d.AccountNumber
	case DestinationPaybill:
		if requested == "" {
			requested = d.AccountNumber
		}
		if requested == "" {
			return "", errs.Invalid("account_reference_required", "account_reference is required for this PayBill channel")
		}
	case DestinationTill:
		if requested == "" {
			requested = "Till Payment"
		}
	default:
		return "", errs.Invalid("invalid_destination", "unknown channel type")
	}
	if !accountRefRe.MatchString(requested) {
		return "", errs.Invalid("invalid_account_reference", "account_reference must be 1-12 characters: letters, digits, space and -_/.")
	}
	return requested, nil
}

// ---------------------------------------------------------------- transitions

// MarkPending records Daraja's acceptance of the STK push.
func (p *Payment) MarkPending(merchantRequestID, checkoutRequestID string, now time.Time) {
	if p.Status != StatusCreated {
		return
	}
	p.Status = StatusPending
	p.MerchantRequestID, p.CheckoutRequestID = merchantRequestID, checkoutRequestID
	p.CheckAttempts = 0
	next := now.Add(checkBackoff[0])
	p.NextCheckAt = &next
	p.UpdatedAt = now
}

// MarkInitiationFailed records a definite rejection of the STK push by
// Daraja. The caller learns this synchronously, so no event is emitted.
func (p *Payment) MarkInitiationFailed(reason string, now time.Time) {
	if p.Status != StatusCreated {
		return
	}
	p.Status = StatusFailed
	p.FailureReason = reason
	p.finish(now)
}

// Result is an STK outcome, from a callback or a status query.
type Result struct {
	Code              int
	Desc              string
	MerchantRequestID string
	CheckoutRequestID string
	Receipt           string // empty when learned via status query
}

// ApplyResult transitions on an STK outcome. It returns nil when the result
// changes nothing (duplicate or late callback).
func (p *Payment) ApplyResult(r Result, now time.Time) *Event {
	if p.Status.Final() {
		return nil
	}
	if p.Status == StatusExpired && r.Code != ResultSuccess {
		return nil
	}
	if p.CheckoutRequestID == "" {
		p.MerchantRequestID, p.CheckoutRequestID = r.MerchantRequestID, r.CheckoutRequestID
	}
	code := r.Code
	p.ResultCode, p.ResultDesc = &code, r.Desc
	var ev EventType
	switch code {
	case ResultSuccess:
		p.Status, ev = StatusSucceeded, EventSucceeded
		p.MpesaReceipt = r.Receipt
		p.FailureReason = ""
	case ResultCancelledByUser:
		p.Status, ev = StatusCancelled, EventCancelled
	default:
		p.Status, ev = StatusFailed, EventFailed
		p.FailureReason = r.Desc
	}
	p.finish(now)
	return &Event{Type: ev, OccurredAt: now}
}

// ---------------------------------------------------------------- reconciliation

const (
	// A payment stuck in created (push outcome unknown, e.g. a timeout
	// talking to Daraja) is expired after this long without a callback.
	createdTimeout    = 3 * time.Minute
	createdCheckDelay = 45 * time.Second
)

// checkBackoff is the delay before each STK status query for a pending
// payment. After the last one we give up and expire it.
var checkBackoff = []time.Duration{
	45 * time.Second, time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 20 * time.Minute,
}

// CanQueryStatus reports whether a status query is possible (we need
// Daraja's CheckoutRequestID).
func (p *Payment) CanQueryStatus() bool {
	return p.Status == StatusPending && p.CheckoutRequestID != ""
}

// RecordUnresolvedCheck is called when a reconciliation pass could not
// determine the outcome. It schedules the next check or expires the
// payment, returning an event if it expired.
func (p *Payment) RecordUnresolvedCheck(now time.Time) *Event {
	switch p.Status {
	case StatusCreated:
		if now.Sub(p.CreatedAt) >= createdTimeout {
			return p.expire("no response from M-Pesa for STK push", now)
		}
		next := now.Add(createdCheckDelay)
		p.NextCheckAt = &next
	case StatusPending:
		p.CheckAttempts++
		if p.CheckAttempts >= len(checkBackoff) {
			return p.expire("payment outcome could not be determined", now)
		}
		next := now.Add(checkBackoff[p.CheckAttempts])
		p.NextCheckAt = &next
	default:
		p.NextCheckAt = nil
	}
	p.UpdatedAt = now
	return nil
}

func (p *Payment) expire(reason string, now time.Time) *Event {
	p.Status = StatusExpired
	p.FailureReason = reason
	p.finish(now)
	return &Event{Type: EventExpired, OccurredAt: now}
}

func (p *Payment) finish(now time.Time) {
	p.NextCheckAt = nil
	p.UpdatedAt = now
	p.CompletedAt = &now
}
