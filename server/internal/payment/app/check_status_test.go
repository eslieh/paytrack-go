package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/payment/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
)

// memRepo is an in-memory Repository recording emitted events.
type memRepo struct {
	Repository
	p      *domain.Payment
	events []domain.Event
}

func (m *memRepo) Get(_ context.Context, tenantID, id string) (*domain.Payment, error) {
	if m.p == nil || m.p.ID != id || m.p.TenantID != tenantID {
		return nil, ErrNotFound
	}
	cp := *m.p
	return &cp, nil
}

func (m *memRepo) Save(_ context.Context, p *domain.Payment, evs []domain.Event) error {
	if p.Version != m.p.Version {
		return ErrVersionConflict
	}
	cp := *p
	cp.Version++
	m.p = &cp
	p.Version++
	m.events = append(m.events, evs...)
	return nil
}

type fakeGateway struct {
	Gateway
	res   STKQueryResult
	err   error
	calls int
}

func (g *fakeGateway) STKQuery(context.Context, string) (STKQueryResult, error) {
	g.calls++
	return g.res, g.err
}

type allowAll struct{ n, limit int }

func (a *allowAll) Allow(context.Context, string, int, time.Duration) (bool, error) {
	a.n++
	return a.limit == 0 || a.n <= a.limit, nil
}

func pending(t *testing.T, status domain.Status, checkoutID string) *domain.Payment {
	t.Helper()
	p, err := domain.New(domain.NewParams{ID: "pay_1", TenantID: "ten_1", Amount: 100, Phone: "0712345678",
		AccountReference: "INV1", CallbackToken: "tok",
		Destination: domain.Destination{ChannelID: "chn_1", Type: domain.DestinationPaybill, PartyB: "400200", Active: true}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p.Status, p.CheckoutRequestID = status, checkoutID
	return p
}

func TestCheckStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("result settles the payment and emits the webhook event", func(t *testing.T) {
		repo := &memRepo{p: pending(t, domain.StatusPending, "ws_CO_1")}
		gw := &fakeGateway{res: STKQueryResult{Result: domain.Result{Code: 0, Desc: "ok"}}}
		svc := NewService(repo, nil, gw, &allowAll{}, "https://cb")
		p, outcome, err := svc.CheckStatus(ctx, "ten_1", "pay_1")
		if err != nil || outcome != CheckUpdated || p.Status != domain.StatusSucceeded {
			t.Fatalf("got %v %v %v", p, outcome, err)
		}
		if len(repo.events) != 1 || repo.events[0].Type != domain.EventSucceeded {
			t.Fatalf("want one payment.succeeded event (-> webhook), got %+v", repo.events)
		}
	})

	t.Run("still processing leaves it pending", func(t *testing.T) {
		repo := &memRepo{p: pending(t, domain.StatusPending, "ws_CO_1")}
		svc := NewService(repo, nil, &fakeGateway{res: STKQueryResult{Pending: true}}, &allowAll{}, "")
		_, outcome, err := svc.CheckStatus(ctx, "ten_1", "pay_1")
		if err != nil || outcome != CheckStillProcessing || len(repo.events) != 0 {
			t.Fatalf("got %v %v events=%d", outcome, err, len(repo.events))
		}
	})

	t.Run("expired payment recovers a late success", func(t *testing.T) {
		repo := &memRepo{p: pending(t, domain.StatusExpired, "ws_CO_1")}
		svc := NewService(repo, nil, &fakeGateway{res: STKQueryResult{Result: domain.Result{Code: 0}}}, &allowAll{}, "")
		p, outcome, _ := svc.CheckStatus(ctx, "ten_1", "pay_1")
		if outcome != CheckUpdated || p.Status != domain.StatusSucceeded {
			t.Fatalf("got %v %v", outcome, p.Status)
		}
	})

	t.Run("expired payment ignores a failure answer", func(t *testing.T) {
		repo := &memRepo{p: pending(t, domain.StatusExpired, "ws_CO_1")}
		svc := NewService(repo, nil, &fakeGateway{res: STKQueryResult{Result: domain.Result{Code: 1037}}}, &allowAll{}, "")
		p, outcome, _ := svc.CheckStatus(ctx, "ten_1", "pay_1")
		if outcome != CheckUnchanged || p.Status != domain.StatusExpired || len(repo.events) != 0 {
			t.Fatalf("got %v %v", outcome, p.Status)
		}
	})

	t.Run("final and unqueryable payments don't call Daraja", func(t *testing.T) {
		for status, want := range map[domain.Status]CheckOutcome{domain.StatusSucceeded: CheckFinal, domain.StatusCreated: CheckNotQueryable} {
			gw := &fakeGateway{}
			svc := NewService(&memRepo{p: pending(t, status, "")}, nil, gw, &allowAll{}, "")
			_, outcome, _ := svc.CheckStatus(ctx, "ten_1", "pay_1")
			if outcome != want || gw.calls != 0 {
				t.Fatalf("%s: got %v, daraja calls %d", status, outcome, gw.calls)
			}
		}
	})

	t.Run("Daraja error is 503, rate limit is 429, other tenant is 404", func(t *testing.T) {
		svc := NewService(&memRepo{p: pending(t, domain.StatusPending, "ws")}, nil, &fakeGateway{err: errors.New("boom")}, &allowAll{}, "")
		if _, _, err := svc.CheckStatus(ctx, "ten_1", "pay_1"); !errs.IsKind(err, errs.KindUnavailable) {
			t.Fatalf("want unavailable, got %v", err)
		}
		svc = NewService(&memRepo{p: pending(t, domain.StatusPending, "ws")}, nil, &fakeGateway{res: STKQueryResult{Pending: true}}, &allowAll{limit: 1}, "")
		svc.CheckStatus(ctx, "ten_1", "pay_1")
		if _, _, err := svc.CheckStatus(ctx, "ten_1", "pay_1"); !errs.IsKind(err, errs.KindRateLimited) {
			t.Fatalf("want rate limited, got %v", err)
		}
		if _, _, err := svc.CheckStatus(ctx, "ten_2", "pay_1"); !errs.IsKind(err, errs.KindNotFound) {
			t.Fatalf("want not found for another tenant, got %v", err)
		}
	})
}
