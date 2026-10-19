package app

import (
	"context"
	"testing"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/support/domain"
)

type memRepo struct {
	tickets  map[string]*domain.Ticket
	messages []*domain.Message
	feedback []*domain.Feedback
	out      []contracts.Notification // published through the "outbox"
}

func (r *memRepo) CreateTicket(_ context.Context, t *domain.Ticket, m *domain.Message, build func() []contracts.Notification) error {
	t.Number = int64(1000 + len(r.tickets))
	c := *t
	r.tickets[t.ID] = &c
	r.messages = append(r.messages, m)
	r.out = append(r.out, build()...)
	return nil
}
func (r *memRepo) GetTicket(_ context.Context, id string) (*domain.Ticket, error) {
	if t, ok := r.tickets[id]; ok {
		c := *t
		return &c, nil
	}
	return nil, ErrNotFound
}
func (r *memRepo) ListTickets(_ context.Context, f TicketFilter) ([]*domain.Ticket, error) {
	var out []*domain.Ticket
	for _, t := range r.tickets {
		if f.TenantID == "" || t.TenantID == f.TenantID {
			out = append(out, t)
		}
	}
	return out, nil
}
func (r *memRepo) SaveTicket(_ context.Context, t *domain.Ticket, out []contracts.Notification) error {
	c := *t
	r.tickets[t.ID] = &c
	r.out = append(r.out, out...)
	return nil
}
func (r *memRepo) AddMessage(ctx context.Context, t *domain.Ticket, m *domain.Message, out []contracts.Notification) error {
	r.messages = append(r.messages, m)
	return r.SaveTicket(ctx, t, out)
}
func (r *memRepo) ListMessages(_ context.Context, ticketID string, internal bool) ([]*domain.Message, error) {
	var out []*domain.Message
	for _, m := range r.messages {
		if m.TicketID == ticketID && (internal || !m.Internal) {
			out = append(out, m)
		}
	}
	return out, nil
}
func (r *memRepo) CountByStatus(context.Context) (map[domain.Status]int, error) { return nil, nil }
func (r *memRepo) CreateFeedback(_ context.Context, f *domain.Feedback) error {
	r.feedback = append(r.feedback, f)
	return nil
}
func (r *memRepo) ListFeedback(context.Context, string, int) ([]*domain.Feedback, error) {
	return r.feedback, nil
}

// emails lists published emails as "template→to".
func (r *memRepo) emails() []string {
	var out []string
	for _, n := range r.out {
		if n.Email != nil {
			out = append(out, n.Email.Template+"→"+n.Email.To)
		}
	}
	return out
}

type accounts struct{}

func (accounts) UserContact(_ context.Context, id string) (string, string, error) {
	return id + "@acme.test", "ten_1", nil
}
func (accounts) TenantName(context.Context, string) (string, error) { return "Acme Shop", nil }

type allowAll struct{}

func (allowAll) Allow(context.Context, string, int, time.Duration) (bool, error) { return true, nil }

var (
	owner = auth.Principal{Kind: auth.KindUser, TenantID: "ten_1", UserID: "usr_1"}
	other = auth.Principal{Kind: auth.KindUser, TenantID: "ten_2", UserID: "usr_2"}
	staff = auth.Principal{Kind: auth.KindStaff, StaffID: "stf_1"}
)

func setup(staffEmail string) (*Service, *memRepo) {
	repo := &memRepo{tickets: map[string]*domain.Ticket{}}
	return NewService(repo, accounts{}, allowAll{}, Config{AppBaseURL: "https://app.test", StaffEmail: staffEmail}), repo
}

func TestConversationAndNotifications(t *testing.T) {
	svc, repo := setup("help@paytrack.test")
	ctx := context.Background()

	tk, err := svc.CreateTicket(ctx, owner, domain.NewTicket{Subject: "Payment stuck", Category: "payments", Body: "It is pending."})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ticket_received→usr_1@acme.test", "support_staff_alert→help@paytrack.test"}
	if got := repo.emails(); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("emails after create: %v", got)
	}
	if repo.out[1].TenantID != "" || repo.out[1].InApp != nil {
		t.Fatalf("staff alert must not reach the tenant's dashboard: %+v", repo.out[1])
	}

	repo.out = nil
	if _, err := svc.StaffReply(ctx, staff, tk.ID, "Checking with Safaricom", true); err != nil {
		t.Fatal(err)
	}
	if len(repo.out) != 0 {
		t.Fatal("internal note notified the customer")
	}
	if _, err := svc.StaffReply(ctx, staff, tk.ID, "Reconciled, it's paid now.", false); err != nil {
		t.Fatal(err)
	}
	n := repo.out[0]
	if e := n.Email; e.Template != "ticket_reply" || e.To != "usr_1@acme.test" || e.Data["Excerpt"] != "Reconciled, it's paid now." ||
		e.Data["TicketURL"] != "https://app.test/dashboard/support/"+tk.ID {
		t.Fatalf("reply email: %+v", e)
	}
	if n.TenantID != "ten_1" || n.UserID != "usr_1" || n.InApp == nil || n.InApp.Link != "/dashboard/support/"+tk.ID {
		t.Fatalf("reply in-app notification: %+v %+v", n, n.InApp)
	}

	_, msgs, err := svc.GetTicket(ctx, owner, tk.ID)
	if err != nil || len(msgs) != 2 {
		t.Fatalf("customer must see 2 messages (no internal note), got %d: %v", len(msgs), err)
	}
	_, msgs, _ = svc.StaffGetTicket(ctx, tk.ID)
	if len(msgs) != 3 {
		t.Fatalf("staff must see 3 messages, got %d", len(msgs))
	}

	repo.out = nil
	resolved := domain.StatusResolved
	if _, err := svc.StaffUpdate(ctx, tk.ID, TicketUpdate{Status: &resolved}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StaffUpdate(ctx, tk.ID, TicketUpdate{Status: &resolved}); err != nil {
		t.Fatal(err)
	}
	if got := repo.emails(); len(got) != 1 || got[0] != "ticket_resolved→usr_1@acme.test" {
		t.Fatalf("resolve must notify once: %v", got)
	}
	if _, err := svc.Rate(ctx, owner, tk.ID, 5, "great"); err != nil {
		t.Fatal(err)
	}
}

func TestTenantIsolation(t *testing.T) {
	svc, _ := setup("")
	ctx := context.Background()
	tk, _ := svc.CreateTicket(ctx, owner, domain.NewTicket{Subject: "Help me", Body: "x"})

	if _, _, err := svc.GetTicket(ctx, other, tk.ID); !errs.IsKind(err, errs.KindNotFound) {
		t.Fatalf("other tenant read: %v", err)
	}
	if _, err := svc.Reply(ctx, other, tk.ID, "hi"); !errs.IsKind(err, errs.KindNotFound) {
		t.Fatalf("other tenant reply: %v", err)
	}
	if ts, _ := svc.ListTickets(ctx, other, "", 10); len(ts) != 0 {
		t.Fatalf("other tenant listed %d tickets", len(ts))
	}
}

func TestNoStaffAlertWithoutAddress(t *testing.T) {
	svc, repo := setup("")
	tk, _ := svc.CreateTicket(context.Background(), owner, domain.NewTicket{Subject: "Help me", Body: "x"})
	_, _ = svc.Reply(context.Background(), owner, tk.ID, "more")
	if got := repo.emails(); len(got) != 1 || got[0] != "ticket_received→usr_1@acme.test" {
		t.Fatalf("emails: %v", got)
	}
}
