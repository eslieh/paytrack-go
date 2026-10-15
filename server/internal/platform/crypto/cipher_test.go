package crypto

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	c, err := NewCipher(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := c.Encrypt([]byte("passkey"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := c.Decrypt(ct)
	if err != nil || string(pt) != "passkey" {
		t.Fatalf("got %q, %v", pt, err)
	}
	ct[len(ct)-1] ^= 1
	if _, err := c.Decrypt(ct); err == nil {
		t.Fatal("expected tamper detection")
	}
}
