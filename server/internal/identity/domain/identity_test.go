package domain

import (
	"testing"
	"time"
)

func TestTokenUsable(t *testing.T) {
	now := time.Now()
	tok := &Token{ExpiresAt: now.Add(time.Minute), LastSentAt: now}
	if err := tok.Usable(now); err != nil {
		t.Fatal(err)
	}
	if tok.CanResend(now) == nil {
		t.Fatal("resend inside cooldown must fail")
	}
	if tok.CanResend(now.Add(CodeResendCooldown)) != nil {
		t.Fatal("resend after cooldown must work")
	}
	tok.Attempts = MaxCodeAttempts
	if tok.Usable(now) == nil {
		t.Fatal("exhausted attempts must fail")
	}
	tok.Attempts = 0
	if tok.Usable(now.Add(2*time.Minute)) == nil {
		t.Fatal("expired must fail")
	}
	consumed := now
	tok.ConsumedAt = &consumed
	if tok.Usable(now) == nil {
		t.Fatal("consumed must fail")
	}
}

func TestMaskEmail(t *testing.T) {
	for in, want := range map[string]string{"esliehh@gmail.com": "e••••@gmail.com", "a@b.co": "a•@b.co", "jo@x.io": "j•@x.io"} {
		if got := MaskEmail(in); got != want {
			t.Errorf("MaskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDisableAndEnable(t *testing.T) {
	now := time.Now()
	u := &User{}
	if err := u.Enable(now); err == nil {
		t.Fatal("enabling an enabled user must fail")
	}
	if err := u.Disable("takeover", now); err != nil || !u.Disabled() || u.DisabledReason != "takeover" {
		t.Fatalf("disable: %+v %v", u, err)
	}
	if err := u.Disable("again", now); err == nil {
		t.Fatal("double disable must fail")
	}
	if err := u.Enable(now); err != nil || u.Disabled() || u.DisabledReason != "" {
		t.Fatalf("enable: %+v %v", u, err)
	}
}
