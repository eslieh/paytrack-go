package app

import (
	"context"
	"testing"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
)

type fakePrefs struct{ p Prefs }

func (f fakePrefs) Prefs(context.Context, string) (Prefs, error) { return f.p, nil }

type fakeMailer struct{ sent []contracts.Email }

func (f *fakeMailer) Enqueue(_ context.Context, e contracts.Email) error {
	f.sent = append(f.sent, e)
	return nil
}

func event(id string) contracts.PaymentEvent {
	done := time.Date(2026, 10, 16, 7, 8, 0, 0, time.UTC)
	return contracts.PaymentEvent{ID: "evt_" + id, Type: contracts.EventPaymentSucceeded, TenantID: "ten_1",
		Data: contracts.PaymentView{ID: "pay_" + id, Amount: 1500, Phone: "254712345678", DestinationType: "bank",
			BankName: "Equity Bank", AccountReference: "0123456789", MpesaReceiptNumber: "TJF1", Description: "Order", CompletedAt: &done}}
}

func TestEmailOnPaymentEvent(t *testing.T) {
	m := &fakeMailer{}
	s := NewService(nil, nil, nil, fakePrefs{Prefs{BusinessName: "Shop", Recipient: "o@shop.co.ke"}}, m, "https://app")
	if err := s.EmailOnPaymentEvent(context.Background(), event("1")); err != nil {
		t.Fatal(err)
	}
	if len(m.sent) != 1 || m.sent[0].Template != "first_payment" || m.sent[0].DedupeKey != "first_payment:ten_1" {
		t.Fatalf("receipts off: want only first_payment, got %+v", m.sent)
	}

	m.sent = nil
	s.prefs = fakePrefs{Prefs{BusinessName: "Shop", Recipient: "o@shop.co.ke", PaymentReceipts: true}}
	if err := s.EmailOnPaymentEvent(context.Background(), event("2")); err != nil {
		t.Fatal(err)
	}
	if len(m.sent) != 2 || m.sent[1].Template != "payment_receipt" || m.sent[1].DedupeKey != "receipt:pay_2" {
		t.Fatalf("receipts on: got %+v", m.sent)
	}
	d := m.sent[1].Data
	if d["Destination"] != "Equity Bank · 0123456789" || d["PaidAt"] != "16 Oct 2026, 10:08 EAT" || d["Amount"] != "KES 1,500" {
		t.Fatalf("receipt data %+v", d)
	}

	m.sent = nil
	ev := event("3")
	ev.Type = contracts.EventPaymentFailed
	_ = s.EmailOnPaymentEvent(context.Background(), ev)
	if len(m.sent) != 0 {
		t.Fatal("failed payments must not email")
	}
}
