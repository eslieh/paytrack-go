package domain

import (
	"strings"
	"testing"
	"time"
)

func TestTicketLifecycle(t *testing.T) {
	now := time.Now()
	tk := &Ticket{Status: StatusAwaitingStaff}

	tk.StaffReplied(true, now)
	if tk.Status != StatusAwaitingStaff || tk.FirstResponseAt != nil {
		t.Fatalf("internal note changed visible state: %+v", tk)
	}
	tk.StaffReplied(false, now)
	if tk.Status != StatusAwaitingUser || tk.FirstResponseAt == nil {
		t.Fatalf("staff reply: %+v", tk)
	}
	if err := tk.SetStatus(StatusResolved, now); err != nil || tk.ResolvedAt == nil {
		t.Fatalf("resolve: %+v %v", tk, err)
	}
	if err := tk.UserReplied(now); err != nil || tk.Status != StatusAwaitingStaff || tk.ResolvedAt != nil {
		t.Fatalf("user reply must reopen a resolved ticket: %+v %v", tk, err)
	}
	_ = tk.SetStatus(StatusClosed, now)
	if err := tk.UserReplied(now); err == nil {
		t.Fatal("closed ticket accepted a reply")
	}
	if err := tk.SetStatus("bogus", now); err == nil {
		t.Fatal("unknown status accepted")
	}
}

func TestRate(t *testing.T) {
	now := time.Now()
	tk := &Ticket{Status: StatusAwaitingUser}
	if err := tk.Rate(5, "", now); err == nil {
		t.Fatal("rated an open ticket")
	}
	tk.Status = StatusResolved
	if err := tk.Rate(6, "", now); err == nil {
		t.Fatal("rating out of range accepted")
	}
	if err := tk.Rate(4, " quick ", now); err != nil || *tk.Rating != 4 || tk.RatingComment != "quick" {
		t.Fatalf("rate: %+v %v", tk, err)
	}
}

func TestNewTicketValidate(t *testing.T) {
	if _, _, _, err := (NewTicket{Subject: "Hi", Body: "x"}).Validate(); err == nil {
		t.Fatal("short subject accepted")
	}
	if _, _, _, err := (NewTicket{Subject: "Help", Category: "nope", Body: "x"}).Validate(); err == nil {
		t.Fatal("unknown category accepted")
	}
	if _, _, _, err := (NewTicket{Subject: "Help", Body: "   "}).Validate(); err == nil {
		t.Fatal("blank body accepted")
	}
	s, c, b, err := NewTicket{Subject: " Payment stuck ", Body: " details "}.Validate()
	if err != nil || s != "Payment stuck" || c != "other" || b != "details" {
		t.Fatalf("got %q %q %q %v", s, c, b, err)
	}
}

func TestExcerpt(t *testing.T) {
	if got := Excerpt("short\n\n  text", 50); got != "short text" {
		t.Fatalf("got %q", got)
	}
	got := Excerpt(strings.Repeat("word ", 100), 40)
	if !strings.HasSuffix(got, "…") || len([]rune(got)) > 41 || strings.HasSuffix(got, " …") {
		t.Fatalf("got %q", got)
	}
}
