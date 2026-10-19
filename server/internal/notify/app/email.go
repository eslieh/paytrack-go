package app

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/notify/domain"
)

// EmailOnPaymentEvent sends the merchant's payment emails: a receipt for
// every successful payment when receipts are on, and a one-time "first
// payment" email. Dedupe keys make redelivered events harmless.
func (s *Service) EmailOnPaymentEvent(ctx context.Context, ev contracts.PaymentEvent) error {
	if ev.Type != contracts.EventPaymentSucceeded {
		return nil
	}
	pr, err := s.prefs.Prefs(ctx, ev.TenantID)
	if err != nil {
		return err
	}
	slog.DebugContext(ctx, "payment email", "tenant_id", ev.TenantID, "payment_id", ev.Data.ID,
		"recipient_set", pr.Recipient != "", "receipts", pr.PaymentReceipts)
	if pr.Recipient == "" {
		return nil
	}
	p := ev.Data
	receiptURL := s.appURL + "/receipt?id=" + p.ID
	if err := s.mailer.Enqueue(ctx, contracts.Email{
		Template: "first_payment", Category: contracts.EmailLifecycle, To: pr.Recipient,
		Data:      map[string]any{"BusinessName": pr.BusinessName, "Amount": FormatKES(p.Amount), "ReceiptURL": receiptURL},
		DedupeKey: "first_payment:" + ev.TenantID,
	}); err != nil {
		return err
	}
	if !pr.PaymentReceipts {
		return nil
	}
	paidAt := p.UpdatedAt
	if p.CompletedAt != nil {
		paidAt = *p.CompletedAt
	}
	accountRef := p.AccountReference
	if p.DestinationType == "till" {
		accountRef = ""
	}
	receipt := p.MpesaReceiptNumber
	if receipt == "" {
		receipt = "Confirmed by M-Pesa status check"
	}
	return s.mailer.Enqueue(ctx, contracts.Email{
		Template: "payment_receipt", Category: contracts.EmailPayments, To: pr.Recipient,
		Data: map[string]any{
			"BusinessName": pr.BusinessName, "Amount": FormatKES(p.Amount), "Receipt": receipt,
			"Phone": MaskPhone(p.Phone), "Destination": Destination(p), "AccountReference": accountRef,
			"Description": p.Description, "PaidAt": paidAt.In(eat).Format("2 Jan 2006, 15:04 MST"),
			"PaymentID": p.ID, "ReceiptURL": receiptURL,
		},
		DedupeKey: "receipt:" + p.ID,
	})
}

var eat = time.FixedZone("EAT", 3*60*60)

func (s *Service) alertWebhookFailed(ctx context.Context, d *domain.Delivery, url string) {
	// One alert per exhausted retry cycle (a manual redeliver starts a new one).
	key := "webhook_failing:" + d.ID + ":" + strconv.FormatInt(d.UpdatedAt.Unix(), 10)
	s.notifyDashboard(ctx, key, d.TenantID, contracts.InApp{
		Kind: "webhook.failing", Title: "A webhook couldn't be delivered",
		Body: d.EventType + " for " + d.PaymentID + ": " + d.LastError, Link: "/dashboard/webhooks",
	})
	pr, err := s.prefs.Prefs(ctx, d.TenantID)
	if err != nil || !pr.WebhookFailureAlerts || pr.Recipient == "" {
		return
	}
	if url == "" {
		url = "(no webhook URL configured)"
	}
	if err := s.mailer.Enqueue(ctx, contracts.Email{
		Template: "webhook_failing", Category: contracts.EmailOperations, To: pr.Recipient,
		Data: map[string]any{
			"BusinessName": pr.BusinessName, "WebhookURL": url, "EventType": d.EventType, "PaymentID": d.PaymentID,
			"Attempts": d.Attempts, "LastError": d.LastError, "DeliveriesURL": s.appURL + "/dashboard/webhooks",
		},
		DedupeKey: key,
	}); err != nil {
		slog.ErrorContext(ctx, "webhook alert email failed", "delivery_id", d.ID, "err", err)
	}
}

// FormatKES renders 1500 as "KES 1,500".
func FormatKES(amount int64) string {
	s := strconv.FormatInt(amount, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return "KES " + b.String()
}

// MaskPhone renders 254712345678 as "+254 7•• ••• 678".
func MaskPhone(p string) string {
	if len(p) != 12 {
		return p
	}
	return fmt.Sprintf("+%s %s•• ••• %s", p[:3], p[3:4], p[9:])
}

func Destination(p contracts.PaymentView) string {
	switch p.DestinationType {
	case "bank":
		return p.BankName + " · " + p.AccountReference
	case "till":
		return "Till " + p.PartyB
	default:
		return "PayBill " + p.PartyB
	}
}
