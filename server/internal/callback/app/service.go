// Package app holds the callback module's single use case: durably accept
// a Daraja callback and hand it on.
package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/callback/domain"
	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
)

type Record struct {
	CallbackToken     string
	CheckoutRequestID string
	Raw               []byte
	ParseError        string
	RemoteAddr        string
	ReceivedAt        time.Time
}

// Inbox stores the raw callback and, if msg is non-nil, enqueues it on the
// outbox in the same transaction. It sets msg.InboxID.
type Inbox interface {
	Store(ctx context.Context, rec Record, msg *contracts.STKCallback) error
}

type Service struct {
	inbox Inbox
	now   func() time.Time
}

func NewService(inbox Inbox) *Service { return &Service{inbox: inbox, now: time.Now} }

// Ingest persists the callback before it is acknowledged. Unparseable
// bodies are still stored (for investigation) but not forwarded.
func (s *Service) Ingest(ctx context.Context, token string, raw []byte, remoteAddr string) error {
	if !domain.ValidToken(token) {
		return errs.NotFound("not_found", "not found")
	}
	rec := Record{CallbackToken: token, Raw: raw, RemoteAddr: remoteAddr, ReceivedAt: s.now()}
	res, err := domain.Parse(raw)
	if err != nil {
		rec.ParseError = err.Error()
		slog.WarnContext(ctx, "unparseable stk callback stored", "err", err, "remote_addr", remoteAddr)
		return s.inbox.Store(ctx, rec, nil)
	}
	rec.CheckoutRequestID = res.CheckoutRequestID
	return s.inbox.Store(ctx, rec, &contracts.STKCallback{
		CallbackToken: token, MerchantRequestID: res.MerchantRequestID, CheckoutRequestID: res.CheckoutRequestID,
		ResultCode: res.ResultCode, ResultDesc: res.ResultDesc, Amount: res.Amount,
		MpesaReceiptNumber: res.MpesaReceiptNumber, TransactionDate: res.TransactionDate,
		PhoneNumber: res.PhoneNumber, ReceivedAt: rec.ReceivedAt,
	})
}
