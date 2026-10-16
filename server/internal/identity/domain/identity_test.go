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
