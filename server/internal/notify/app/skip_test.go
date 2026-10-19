package app

import (
	"context"
	"testing"

	"github.com/eslieh/paytrack-go/server/internal/notify/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
)

type memDeliveries struct {
	Repository
	saved map[string]*domain.Delivery
}

func (m *memDeliveries) Create(_ context.Context, d *domain.Delivery) error {
	m.saved[d.ID] = d
	return nil
}

func (m *memDeliveries) Get(_ context.Context, _, id string) (*domain.Delivery, error) {
	if d, ok := m.saved[id]; ok {
		return d, nil
	}
	return nil, ErrNotFound
}

func (m *memDeliveries) Save(_ context.Context, d *domain.Delivery) error {
	m.saved[d.ID] = d
	return nil
}

type fixedTarget struct{ url string }

func (f *fixedTarget) Target(context.Context, string) (Target, error) {
	return Target{URL: f.url, Secret: "whsec_x"}, nil
}

func TestNoWebhookURLSkipsDelivery(t *testing.T) {
	ctx := context.Background()
	repo := &memDeliveries{saved: map[string]*domain.Delivery{}}
	target := &fixedTarget{}
	s := NewService(repo, target, nil, fakePrefs{}, &fakeMailer{}, "https://app")

	if err := s.Enqueue(ctx, event("1")); err != nil {
		t.Fatal(err)
	}
	var d *domain.Delivery
	for _, v := range repo.saved {
		d = v
	}
	if d == nil || d.Status != domain.StatusSkipped || d.NextAttemptAt != nil || d.LastError != domain.NoEndpoint {
		t.Fatalf("want a skipped delivery with nothing scheduled, got %+v", d)
	}

	if _, err := s.Redeliver(ctx, "ten_1", d.ID); !errs.IsKind(err, errs.KindConflict) {
		t.Fatalf("redeliver without a URL: want conflict, got %v", err)
	}

	target.url = "https://shop.example/hooks"
	got, err := s.Redeliver(ctx, "ten_1", d.ID)
	if err != nil || got.Status != domain.StatusPending || got.NextAttemptAt == nil {
		t.Fatalf("redeliver after setting a URL: got %+v, %v", got, err)
	}

	// With a URL configured, new events are queued normally.
	repo.saved = map[string]*domain.Delivery{}
	_ = s.Enqueue(ctx, event("2"))
	for _, v := range repo.saved {
		if v.Status != domain.StatusPending {
			t.Fatalf("want pending, got %s", v.Status)
		}
	}
}
