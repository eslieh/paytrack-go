package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/admin/domain"
	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
)

// ---------------------------------------------------------------- fakes

type memRepo struct {
	staff      map[string]*domain.Staff
	challenges map[string]*domain.Challenge
	sessions   map[string]*domain.Session
	audit      []*domain.AuditEntry
	out        []contracts.Notification // published with audit entries
}

func newMemRepo() *memRepo {
	return &memRepo{staff: map[string]*domain.Staff{}, challenges: map[string]*domain.Challenge{}, sessions: map[string]*domain.Session{}}
}

func (r *memRepo) CreateStaff(_ context.Context, s *domain.Staff) error {
	c := *s
	r.staff[s.ID] = &c
	return nil
}
func (r *memRepo) GetStaff(_ context.Context, id string) (*domain.Staff, error) {
	if s, ok := r.staff[id]; ok {
		c := *s
		return &c, nil
	}
	return nil, ErrNotFound
}
func (r *memRepo) GetStaffByEmail(ctx context.Context, email string) (*domain.Staff, error) {
	for _, s := range r.staff {
		if s.Email == email {
			return r.GetStaff(ctx, s.ID)
		}
	}
	return nil, ErrNotFound
}
func (r *memRepo) SaveStaff(_ context.Context, s *domain.Staff) error {
	c := *s
	r.staff[s.ID] = &c
	return nil
}
func (r *memRepo) CreateChallenge(_ context.Context, c *domain.Challenge) error {
	x := *c
	r.challenges[c.ID] = &x
	return nil
}
func (r *memRepo) GetChallenge(_ context.Context, id string) (*domain.Challenge, error) {
	if c, ok := r.challenges[id]; ok {
		x := *c
		return &x, nil
	}
	return nil, ErrNotFound
}
func (r *memRepo) SaveChallenge(_ context.Context, c *domain.Challenge) error {
	x := *c
	r.challenges[c.ID] = &x
	return nil
}
func (r *memRepo) CreateSession(_ context.Context, s *domain.Session) error {
	x := *s
	r.sessions[s.ID] = &x
	return nil
}
func (r *memRepo) GetSessionByToken(_ context.Context, hash []byte) (*domain.Session, error) {
	for _, s := range r.sessions {
		if bytes.Equal(s.TokenHash, hash) {
			x := *s
			return &x, nil
		}
	}
	return nil, ErrNotFound
}
func (r *memRepo) TouchSession(_ context.Context, id string, at time.Time) error {
	r.sessions[id].LastSeenAt = at
	return nil
}
func (r *memRepo) RevokeSession(_ context.Context, id string, at time.Time) error {
	r.sessions[id].RevokedAt = &at
	return nil
}
func (r *memRepo) RevokeStaffSessions(_ context.Context, staffID string, at time.Time) error {
	for _, s := range r.sessions {
		if s.StaffID == staffID {
			s.RevokedAt = &at
		}
	}
	return nil
}
func (r *memRepo) AddAudit(_ context.Context, e *domain.AuditEntry, out []contracts.Notification) error {
	e.ID = int64(len(r.audit) + 1)
	r.audit = append(r.audit, e)
	r.out = append(r.out, out...)
	return nil
}

// notification returns the last published notification with this email template.
func (r *memRepo) notification(t *testing.T, template string) contracts.Notification {
	t.Helper()
	for i := len(r.out) - 1; i >= 0; i-- {
		if r.out[i].Email != nil && r.out[i].Email.Template == template {
			return r.out[i]
		}
	}
	t.Fatalf("no %s notification published", template)
	return contracts.Notification{}
}
func (r *memRepo) ListAudit(context.Context, AuditFilter) ([]*domain.AuditEntry, error) {
	return r.audit, nil
}

type plainHasher struct{}

func (plainHasher) Hash(p string) (string, error) { return "h:" + p, nil }
func (plainHasher) Compare(h, p string) bool      { return h == "h:"+p }

type mailbox struct{ sent []contracts.Email }

func (m *mailbox) Enqueue(_ context.Context, e contracts.Email) error {
	m.sent = append(m.sent, e)
	return nil
}

func (m *mailbox) last(t *testing.T, template string) contracts.Email {
	t.Helper()
	for i := len(m.sent) - 1; i >= 0; i-- {
		if m.sent[i].Template == template {
			return m.sent[i]
		}
	}
	t.Fatalf("no %s email sent", template)
	return contracts.Email{}
}

type allowAll struct{}

func (allowAll) Allow(context.Context, string, int, time.Duration) (bool, error) { return true, nil }

type fakeTenants struct{ suspended map[string]string }

func (f *fakeTenants) TenantName(context.Context, string) (string, error) { return "Acme Shop", nil }
func (f *fakeTenants) SuspendTenant(_ context.Context, id, reason string) error {
	f.suspended[id] = reason
	return nil
}
func (f *fakeTenants) ReinstateTenant(_ context.Context, id string) error {
	delete(f.suspended, id)
	return nil
}

type fakeUsers struct{ disabled map[string]bool }

func (f *fakeUsers) OwnerContact(context.Context, string) (string, bool, error) {
	return "owner@acme.test", true, nil
}
func (f *fakeUsers) UserContact(_ context.Context, id string) (string, string, error) {
	if id != "usr_1" {
		return "", "", errs.NotFound("user_not_found", "user not found")
	}
	return "user@acme.test", "ten_1", nil
}
func (f *fakeUsers) DisableUser(_ context.Context, id, _ string, _ Client) error {
	f.disabled[id] = true
	return nil
}
func (f *fakeUsers) EnableUser(_ context.Context, id string, _ Client) error {
	delete(f.disabled, id)
	return nil
}

type fixture struct {
	svc     *Service
	repo    *memRepo
	mail    *mailbox
	tenants *fakeTenants
	users   *fakeUsers
	now     time.Time
}

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{repo: newMemRepo(), mail: &mailbox{}, tenants: &fakeTenants{suspended: map[string]string{}},
		users: &fakeUsers{disabled: map[string]bool{}}, now: time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)}
	svc, err := NewService(f.repo, plainHasher{}, f.mail, allowAll{}, f.tenants, f.users,
		Config{AppBaseURL: "https://app.test", CodeKey: []byte("k")})
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return f.now }
	f.svc = svc
	if _, err := svc.EnsureStaff(context.Background(), "Ops@Paytrack.test", "Ops", "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	return f
}

var client = Client{IP: "203.0.113.9", UserAgent: "test"}

// signIn runs both sign-in steps and returns the session token.
func (f *fixture) signIn(t *testing.T) string {
	t.Helper()
	ch, err := f.svc.Login(context.Background(), "ops@paytrack.test", "correct horse battery", client)
	if err != nil {
		t.Fatal(err)
	}
	code := f.mail.last(t, "admin_login_code").Data["Code"].(string)
	in, err := f.svc.VerifyLogin(context.Background(), ch.ID, code, client)
	if err != nil {
		t.Fatal(err)
	}
	return in.Token
}

// ---------------------------------------------------------------- tests

func TestSignInNeedsPasswordAndCode(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.svc.Login(ctx, "ops@paytrack.test", "wrong password!", client); !errs.IsKind(err, errs.KindUnauthorized) {
		t.Fatalf("wrong password: got %v", err)
	}
	if _, err := f.svc.Login(ctx, "nobody@paytrack.test", "correct horse battery", client); !errs.IsKind(err, errs.KindUnauthorized) {
		t.Fatalf("unknown email: got %v", err)
	}
	if len(f.mail.sent) != 0 {
		t.Fatal("no code may be sent without the right password")
	}

	ch, err := f.svc.Login(ctx, "ops@paytrack.test", "correct horse battery", client)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ch.MaskedEmail, "o") || strings.Contains(ch.MaskedEmail, "ops@") {
		t.Fatalf("email not masked: %q", ch.MaskedEmail)
	}
	code := f.mail.last(t, "admin_login_code").Data["Code"].(string)
	if _, err := f.svc.VerifyLogin(ctx, ch.ID, "000000x", client); err == nil {
		t.Fatal("wrong code accepted")
	}
	in, err := f.svc.VerifyLogin(ctx, ch.ID, code, client)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.VerifyLogin(ctx, ch.ID, code, client); err == nil {
		t.Fatal("code reused")
	}
	p, err := f.svc.Authenticate(ctx, in.Token)
	if err != nil || p.Kind != auth.KindStaff || p.StaffID != in.Staff.ID {
		t.Fatalf("authenticate: %+v %v", p, err)
	}
	if f.repo.audit[0].Action != domain.ActionSignIn {
		t.Fatalf("sign-in not audited: %+v", f.repo.audit)
	}
}

func TestCodeLocksAfterTooManyAttempts(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	ch, _ := f.svc.Login(ctx, "ops@paytrack.test", "correct horse battery", client)
	code := f.mail.last(t, "admin_login_code").Data["Code"].(string)
	for range domain.MaxCodeAttempts {
		_, _ = f.svc.VerifyLogin(ctx, ch.ID, "wrong", client)
	}
	if _, err := f.svc.VerifyLogin(ctx, ch.ID, code, client); err == nil {
		t.Fatal("right code accepted after too many attempts")
	}
}

func TestSessionIdleTimeoutAndLogout(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	token := f.signIn(t)

	f.now = f.now.Add(domain.SessionIdleTimeout - time.Minute)
	p, err := f.svc.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("active session rejected: %v", err)
	}
	f.now = f.now.Add(domain.SessionIdleTimeout - time.Minute) // activity above kept it alive
	if _, err := f.svc.Authenticate(ctx, token); err != nil {
		t.Fatalf("touched session rejected: %v", err)
	}
	if err := f.svc.Logout(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(ctx, token); err == nil {
		t.Fatal("session usable after logout")
	}

	token = f.signIn(t)
	f.now = f.now.Add(domain.SessionIdleTimeout)
	if _, err := f.svc.Authenticate(ctx, token); err == nil {
		t.Fatal("idle session still usable")
	}
}

func TestPasswordResetFromConsoleEndsSessions(t *testing.T) {
	f := setup(t)
	token := f.signIn(t)
	if created, err := f.svc.EnsureStaff(context.Background(), "ops@paytrack.test", "", "another long password"); err != nil || created {
		t.Fatalf("reset: created=%v err=%v", created, err)
	}
	if _, err := f.svc.Authenticate(context.Background(), token); err == nil {
		t.Fatal("session survived password reset")
	}
}

func TestSuspendTenantRequiresReasonAuditsAndEmailsOwner(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p, _ := f.svc.Authenticate(ctx, f.signIn(t))

	if err := f.svc.SuspendTenant(ctx, p, "ten_1", "  ", client); !errs.IsKind(err, errs.KindInvalid) {
		t.Fatalf("empty reason: got %v", err)
	}
	if err := f.svc.SuspendTenant(ctx, p, "ten_1", "Chargebacks under review", client); err != nil {
		t.Fatal(err)
	}
	if f.tenants.suspended["ten_1"] != "Chargebacks under review" {
		t.Fatal("tenant not suspended")
	}
	last := f.repo.audit[len(f.repo.audit)-1]
	if last.Action != domain.ActionTenantSuspend || last.TargetID != "ten_1" || last.StaffID != p.StaffID || last.Reason == "" {
		t.Fatalf("audit entry: %+v", last)
	}
	n := f.repo.notification(t, "tenant_suspended")
	if e := n.Email; e.To != "owner@acme.test" || e.Data["Reason"] != "Chargebacks under review" || e.Data["BusinessName"] != "Acme Shop" {
		t.Fatalf("suspension email: %+v", e)
	}
	if n.TenantID != "ten_1" || n.UserID != "" || n.InApp == nil || n.InApp.Kind != "account.suspended" {
		t.Fatalf("suspension must reach every user's dashboard: %+v", n)
	}

	if err := f.svc.ReinstateTenant(ctx, p, "ten_1", "Review cleared", client); err != nil {
		t.Fatal(err)
	}
	if _, still := f.tenants.suspended["ten_1"]; still {
		t.Fatal("tenant still suspended")
	}
	f.repo.notification(t, "tenant_reinstated")
}

func TestDisableUserEmailsUserAndAudits(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p, _ := f.svc.Authenticate(ctx, f.signIn(t))

	if err := f.svc.DisableUser(ctx, p, "usr_missing", "Suspected takeover", client); !errs.IsKind(err, errs.KindNotFound) {
		t.Fatalf("missing user: got %v", err)
	}
	if err := f.svc.DisableUser(ctx, p, "usr_1", "Suspected takeover", client); err != nil {
		t.Fatal(err)
	}
	if !f.users.disabled["usr_1"] {
		t.Fatal("user not disabled")
	}
	if n := f.repo.notification(t, "account_disabled"); n.Email.To != "user@acme.test" || n.InApp != nil {
		t.Fatalf("disable notification: %+v", n)
	}
	if last := f.repo.audit[len(f.repo.audit)-1]; last.Action != domain.ActionUserDisable || last.TargetID != "usr_1" {
		t.Fatalf("audit entry: %+v", last)
	}
}
