package domain

import (
	"testing"
	"time"
)

var (
	t0      = time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	paybill = Destination{ChannelID: "chn_1", Type: DestinationPaybill, PartyB: "400200", Active: true}
	bank    = Destination{ChannelID: "chn_2", Type: DestinationBank, PartyB: "247247", BankName: "Equity Bank", AccountNumber: "0123456789", Active: true}
	till    = Destination{ChannelID: "chn_3", Type: DestinationTill, PartyB: "123456", Active: true}
)

func newPayment(t *testing.T) *Payment {
	t.Helper()
	p, err := New(NewParams{ID: "pay_1", TenantID: "ten_1", Destination: paybill, Amount: 100,
		Phone: "0712345678", AccountReference: "INV-1", CallbackToken: "tok"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNormalizePhone(t *testing.T) {
	for in, want := range map[string]string{
		"0712345678": "254712345678", "712345678": "254712345678", "+254 712 345 678": "254712345678",
		"254112345678": "254112345678", "0112345678": "254112345678",
	} {
		if got, err := NormalizePhone(in); err != nil || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "12345", "0812345678", "25471234567"} {
		if _, err := NormalizePhone(bad); err == nil {
			t.Errorf("NormalizePhone(%q) succeeded, want error", bad)
		}
	}
}

func TestNewValidates(t *testing.T) {
	base := NewParams{Destination: paybill, Amount: 10, Phone: "0712345678", AccountReference: "A"}
	cases := map[string]func(*NewParams){
		"zero amount": func(p *NewParams) { p.Amount = 0 },
		"too much":    func(p *NewParams) { p.Amount = MaxAmount + 1 },
		"long ref":    func(p *NewParams) { p.AccountReference = "123456789012345678901" },
		"long desc":   func(p *NewParams) { p.Description = "12345678901234" },
		"bad phone":   func(p *NewParams) { p.Phone = "123" },
	}
	for name, mut := range cases {
		p := base
		mut(&p)
		if _, err := New(p, t0); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestHappyPath(t *testing.T) {
	p := newPayment(t)
	p.MarkPending("m1", "ws_CO_1", t0)
	if p.Status != StatusPending || p.CheckoutRequestID != "ws_CO_1" {
		t.Fatalf("got %+v", p)
	}
	ev := p.ApplyResult(Result{Code: 0, Desc: "ok", Receipt: "R1"}, t0.Add(time.Minute))
	if ev == nil || ev.Type != EventSucceeded || p.Status != StatusSucceeded || p.MpesaReceipt != "R1" || p.NextCheckAt != nil {
		t.Fatalf("got %+v, %+v", p, ev)
	}
	if again := p.ApplyResult(Result{Code: 1, Desc: "late"}, t0.Add(2*time.Minute)); again != nil || p.Status != StatusSucceeded {
		t.Fatal("final state must not change on duplicate callback")
	}
}

func TestCallbackBeforePendingSaved(t *testing.T) {
	// Daraja can call back before we persist the push acknowledgement.
	p := newPayment(t)
	ev := p.ApplyResult(Result{Code: 1032, Desc: "cancelled", CheckoutRequestID: "ws_CO_2"}, t0)
	if ev == nil || ev.Type != EventCancelled || p.CheckoutRequestID != "ws_CO_2" {
		t.Fatalf("got %+v", p)
	}
	p.MarkPending("m", "ws_CO_2", t0)
	if p.Status != StatusCancelled {
		t.Fatal("MarkPending must not reopen a final payment")
	}
}

func TestExpiryAndLateSuccess(t *testing.T) {
	p := newPayment(t)
	p.MarkPending("m", "c", t0)
	var ev *Event
	for i := 0; i < len(checkBackoff); i++ {
		ev = p.RecordUnresolvedCheck(t0)
	}
	if ev == nil || ev.Type != EventExpired || p.Status != StatusExpired {
		t.Fatalf("expected expiry, got %+v", p)
	}
	if p.ApplyResult(Result{Code: 2001, Desc: "wrong pin"}, t0) != nil {
		t.Fatal("late failure must not change expired payment")
	}
	if ev := p.ApplyResult(Result{Code: 0, Receipt: "R"}, t0); ev == nil || p.Status != StatusSucceeded {
		t.Fatal("late success must win over expired")
	}
}

func TestCreatedTimeout(t *testing.T) {
	p := newPayment(t)
	if ev := p.RecordUnresolvedCheck(t0.Add(time.Minute)); ev != nil || p.Status != StatusCreated {
		t.Fatal("should still wait")
	}
	if ev := p.RecordUnresolvedCheck(t0.Add(createdTimeout)); ev == nil || p.Status != StatusExpired {
		t.Fatal("should expire")
	}
}

func TestInitiationFailed(t *testing.T) {
	p := newPayment(t)
	p.MarkInitiationFailed("Invalid PartyB", t0)
	if p.Status != StatusFailed || p.FailureReason != "Invalid PartyB" || p.NextCheckAt != nil {
		t.Fatalf("got %+v", p)
	}
}

func TestAccountReferenceRouting(t *testing.T) {
	cases := []struct {
		dest      Destination
		requested string
		want      string
	}{
		{bank, "", "0123456789"},
		{bank, "0123456789", "0123456789"},
		{Destination{Type: DestinationPaybill, PartyB: "400200", AccountNumber: "DEFAULT"}, "", "DEFAULT"},
		{Destination{Type: DestinationPaybill, PartyB: "400200", AccountNumber: "DEFAULT"}, "INV-9", "INV-9"},
		{till, "", "Till Payment"},
	}
	for _, c := range cases {
		p, err := New(NewParams{Destination: c.dest, Amount: 1, Phone: "0712345678", AccountReference: c.requested}, t0)
		if err != nil || p.AccountReference != c.want || p.PartyB != c.dest.PartyB || p.DestinationType != c.dest.Type {
			t.Errorf("%s %q: got %+v, %v; want ref %q", c.dest.Type, c.requested, p, err, c.want)
		}
	}
	if DestinationTill.TransactionType() != "CustomerBuyGoodsOnline" || DestinationBank.TransactionType() != "CustomerPayBillOnline" {
		t.Error("transaction types")
	}
}
