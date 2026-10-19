package domain

import (
	"testing"
	"time"
)

func TestNewChannel(t *testing.T) {
	now := time.Now()
	c, err := NewChannel("c", "t", NewChannelParams{Name: "Savings", Type: ChannelBank, BankCode: "247247", AccountNumber: "0123456789"}, now)
	if err != nil || c.PartyB != "247247" || c.BankName != "Equity Bank" || c.AccountNumber != "0123456789" {
		t.Fatalf("bank: %+v %v", c, err)
	}
	c, err = NewChannel("c", "t", NewChannelParams{Name: "Till", Type: ChannelTill, BusinessNumber: "123456", AccountNumber: "ignored"}, now)
	if err != nil || c.PartyB != "123456" || c.AccountNumber != "" {
		t.Fatalf("till: %+v %v", c, err)
	}
	for name, p := range map[string]NewChannelParams{
		"unknown bank":     {Name: "x", Type: ChannelBank, BankCode: "999999", AccountNumber: "1"},
		"bank no account":  {Name: "x", Type: ChannelBank, BankCode: "247247"},
		"paybill bad num":  {Name: "x", Type: ChannelPaybill, BusinessNumber: "12"},
		"account too long": {Name: "x", Type: ChannelPaybill, BusinessNumber: "400200", AccountNumber: "123456789012345678901"},
		"unknown type":     {Name: "x", Type: "wallet"},
	} {
		if _, err := NewChannel("c", "t", p, now); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
