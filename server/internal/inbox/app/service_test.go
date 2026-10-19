package app

import (
	"context"
	"testing"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/inbox/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
)

type memRepo struct {
	rows map[string]*domain.Notification
}

func (r *memRepo) Insert(_ context.Context, n *domain.Notification) error {
	if _, ok := r.rows[n.ID]; !ok {
		r.rows[n.ID] = n
	}
	return nil
}
func (r *memRepo) List(_ context.Context, tenantID, userID string, _ time.Time, _ int) ([]*domain.Notification, error) {
	var out []*domain.Notification
	for _, n := range r.rows {
		if n.TenantID == tenantID && (n.UserID == "" || n.UserID == userID) {
			out = append(out, n)
		}
	}
	return out, nil
}
func (r *memRepo) CountUnread(ctx context.Context, tenantID, userID string) (int, error) {
	ns, _ := r.List(ctx, tenantID, userID, time.Time{}, 0)
	c := 0
	for _, n := range ns {
		if n.ReadAt == nil {
			c++
		}
	}
	return c, nil
}
func (r *memRepo) MarkRead(context.Context, string, string, string, time.Time) error { return nil }
func (r *memRepo) MarkAllRead(context.Context, string, string, time.Time) error      { return nil }

type mailbox struct{ sent []contracts.Email }

func (m *mailbox) Enqueue(_ context.Context, e contracts.Email) error {
	m.sent = append(m.sent, e)
	return nil
}

func TestDeliver(t *testing.T) {
	repo, mail := &memRepo{rows: map[string]*domain.Notification{}}, &mailbox{}
	svc := NewService(repo, mail)
	ctx := context.Background()

	n := contracts.Notification{ID: "ntf_1", TenantID: "ten_1", UserID: "usr_1",
		Email: &contracts.Email{Template: "ticket_reply", To: "o@acme.test"},
		InApp: &contracts.InApp{Kind: "support.reply", Title: "New reply on #4"}}
	for range 2 { // redelivery
		if err := svc.Deliver(ctx, n); err != nil {
			t.Fatal(err)
		}
	}
	if len(repo.rows) != 1 {
		t.Fatalf("redelivery stored %d rows", len(repo.rows))
	}
	if len(mail.sent) != 2 || mail.sent[0].DedupeKey != "ntf:ntf_1" || mail.sent[1].DedupeKey != "ntf:ntf_1" {
		t.Fatalf("emails must carry the notification dedupe key: %+v", mail.sent)
	}

	// Staff-only alert: email, no dashboard row.
	_ = svc.Deliver(ctx, contracts.Notification{ID: "ntf_2", Email: &contracts.Email{Template: "support_staff_alert"}})
	if len(repo.rows) != 1 {
		t.Fatal("staff alert reached a dashboard")
	}

	// Tenant-wide notifications are visible to every user; personal ones only to their user.
	_ = svc.Deliver(ctx, contracts.Notification{ID: "ntf_3", TenantID: "ten_1", InApp: &contracts.InApp{Title: "Suspended"}})
	mine := auth.Principal{TenantID: "ten_1", UserID: "usr_1"}
	teammate := auth.Principal{TenantID: "ten_1", UserID: "usr_9"}
	if c, _ := svc.UnreadCount(ctx, mine); c != 2 {
		t.Fatalf("owner unread = %d", c)
	}
	if c, _ := svc.UnreadCount(ctx, teammate); c != 1 {
		t.Fatalf("teammate unread = %d", c)
	}

	if err := svc.Deliver(ctx, contracts.Notification{}); err == nil {
		t.Fatal("notification without id accepted")
	}
}
