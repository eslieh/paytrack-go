package domain

import (
	"testing"
	"time"
)

func TestSuspendAndReinstate(t *testing.T) {
	now := time.Now()
	tn := &Tenant{ID: "t", Status: StatusActive}
	if err := tn.Reinstate(); err == nil {
		t.Fatal("reinstating an active tenant must fail")
	}
	if err := tn.Suspend("fraud review", now); err != nil || !tn.Suspended() || tn.SuspendedReason != "fraud review" {
		t.Fatalf("suspend: %+v %v", tn, err)
	}
	if err := tn.Suspend("again", now); err == nil {
		t.Fatal("double suspend must fail")
	}
	if err := tn.Reinstate(); err != nil || tn.Suspended() || tn.SuspendedAt != nil || tn.SuspendedReason != "" {
		t.Fatalf("reinstate: %+v %v", tn, err)
	}
}
