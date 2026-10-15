package domain

import (
	"testing"
	"time"
)

func TestSign(t *testing.T) {
	got := Sign("whsec_test", time.Unix(1700000000, 0), []byte(`{"a":1}`))
	// echo -n '1700000000.{"a":1}' | openssl dgst -sha256 -hmac whsec_test
	want := "t=1700000000,v1=38877139021993b830af32feea6e18a8da83eb2f6e49ee50bd9e4cf4ca4d3789"
	if got != want {
		t.Fatalf("Sign = %q, want %q", got, want)
	}
}

func TestRetryScheduleThenFail(t *testing.T) {
	now := time.Unix(0, 0)
	d := NewDelivery("d", "e", "t", "p", "payment.succeeded", nil, now)
	for i := range Backoff {
		d.RecordFailure(500, "boom", now)
		if d.Status != StatusPending || !d.NextAttemptAt.Equal(now.Add(Backoff[i])) {
			t.Fatalf("attempt %d: %+v", i+1, d)
		}
	}
	d.RecordFailure(500, "boom", now)
	if d.Status != StatusFailed || d.NextAttemptAt != nil {
		t.Fatalf("expected failed, got %+v", d)
	}
	d.Redeliver(now)
	if d.Status != StatusPending || d.Attempts != 0 {
		t.Fatalf("redeliver: %+v", d)
	}
	d.RecordSuccess(204, now)
	if d.Status != StatusDelivered || d.DeliveredAt == nil {
		t.Fatalf("success: %+v", d)
	}
}
