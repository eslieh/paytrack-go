// Package app holds the admin module's use cases: staff sign-in (password
// plus an emailed code, every time), cookie sessions, the audit log, and
// account actions on tenants and users. Every account action requires a
// reason, is audited, and notifies the customer by email.
package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/admin/domain"
	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
)

type Config struct {
	AppBaseURL string // dashboard URL used in email links
	CodeKey    []byte // HMAC key for sign-in codes
}

type Service struct {
	repo    Repository
	hasher  PasswordHasher
	mailer  Mailer
	limiter RateLimiter
	tenants Tenants
	users   Users
	cfg     Config
	now     func() time.Time

	dummyHash string // keeps Login timing similar whether or not the email exists
}

func NewService(repo Repository, hasher PasswordHasher, mailer Mailer, limiter RateLimiter, tenants Tenants, users Users,
	cfg Config) (*Service, error) {
	dummy, err := hasher.Hash("not-a-real-password")
	if err != nil {
		return nil, err
	}
	return &Service{repo: repo, hasher: hasher, mailer: mailer, limiter: limiter, tenants: tenants, users: users,
		cfg: cfg, now: time.Now, dummyHash: dummy}, nil
}

// Client describes the caller, for rate limits, sessions and audit.
type Client struct {
	IP        string
	UserAgent string
}

// ---------------------------------------------------------------- staff accounts

// EnsureStaff creates a staff account, or resets the password and re-enables
// an existing one. It is run from the command line (paytrack create-admin),
// never over HTTP.
func (s *Service) EnsureStaff(ctx context.Context, email, name, password string) (created bool, err error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return false, errs.Invalid("invalid_email", "email address is invalid")
	}
	if len(password) < 12 || len(password) > 72 {
		return false, errs.Invalid("weak_password", "staff password must be 12-72 characters")
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return false, err
	}
	now := s.now()
	st, err := s.repo.GetStaffByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		return true, s.repo.CreateStaff(ctx, &domain.Staff{
			ID: ids.New("stf"), Email: email, Name: strings.TrimSpace(name), PasswordHash: hash, CreatedAt: now, UpdatedAt: now,
		})
	}
	if err != nil {
		return false, err
	}
	st.PasswordHash, st.DisabledAt, st.UpdatedAt = hash, nil, now
	if name = strings.TrimSpace(name); name != "" {
		st.Name = name
	}
	if err := s.repo.SaveStaff(ctx, st); err != nil {
		return false, err
	}
	// A password reset from the console ends existing sessions.
	return false, s.repo.RevokeStaffSessions(ctx, st.ID, now)
}

// ---------------------------------------------------------------- sign-in

type Challenge struct {
	ID          string
	MaskedEmail string
	ExpiresAt   time.Time
}

// Login checks the password and emails a sign-in code. Staff sign-in always
// takes both factors.
func (s *Service) Login(ctx context.Context, email, password string, c Client) (*Challenge, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := s.limit(ctx, "admin-login:ip:"+c.IP, 20, 15*time.Minute); err != nil {
		return nil, err
	}
	if err := s.limit(ctx, "admin-login:email:"+email, 5, 15*time.Minute); err != nil {
		return nil, err
	}
	invalid := errs.Unauthorized("invalid_credentials", "email or password is incorrect")
	st, err := s.repo.GetStaffByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		s.hasher.Compare(s.dummyHash, password)
		return nil, invalid
	}
	if err != nil {
		return nil, err
	}
	if !s.hasher.Compare(st.PasswordHash, password) || st.Disabled() {
		return nil, invalid
	}
	now := s.now()
	ch := &domain.Challenge{ID: ids.New("ach"), StaffID: st.ID, CreatedAt: now, ExpiresAt: now.Add(domain.ChallengeTTL)}
	code := newCode()
	ch.CodeHash = s.hashCode(ch.ID, code)
	if err := s.repo.CreateChallenge(ctx, ch); err != nil {
		return nil, err
	}
	s.mail(ctx, st.Email, "admin_login_code", map[string]any{
		"Code": code, "ExpiresMinutes": int(domain.ChallengeTTL / time.Minute),
		"When": s.when(), "Device": describe(c.UserAgent), "IP": c.IP,
	})
	return &Challenge{ID: ch.ID, MaskedEmail: maskEmail(st.Email), ExpiresAt: ch.ExpiresAt}, nil
}

// SignedIn is a new staff session; Token goes into the session cookie.
type SignedIn struct {
	Token     string
	ExpiresAt time.Time
	Staff     *domain.Staff
}

func (s *Service) VerifyLogin(ctx context.Context, challengeID, code string, c Client) (*SignedIn, error) {
	if err := s.limit(ctx, "admin-verify:ip:"+c.IP, 30, 15*time.Minute); err != nil {
		return nil, err
	}
	invalid := errs.Invalid("invalid_code", "that code is incorrect")
	ch, err := s.repo.GetChallenge(ctx, challengeID)
	if errors.Is(err, ErrNotFound) {
		return nil, errs.Invalid("invalid_challenge", "sign-in expired; start again")
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	if err := ch.Usable(now); err != nil {
		return nil, err
	}
	if !hmac.Equal(ch.CodeHash, s.hashCode(ch.ID, strings.TrimSpace(code))) {
		ch.Attempts++
		if err := s.repo.SaveChallenge(ctx, ch); err != nil {
			return nil, err
		}
		return nil, invalid
	}
	ch.ConsumedAt = &now
	if err := s.repo.SaveChallenge(ctx, ch); err != nil {
		return nil, err
	}
	st, err := s.repo.GetStaff(ctx, ch.StaffID)
	if err != nil {
		return nil, err
	}
	if st.Disabled() {
		return nil, errs.Unauthorized("invalid_credentials", "email or password is incorrect")
	}

	raw := ids.SecretBase32(32)
	sess := &domain.Session{
		ID: ids.New("ass"), StaffID: st.ID, TokenHash: hashToken(raw), IP: c.IP, UserAgent: truncate(c.UserAgent, 300),
		CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(domain.SessionMaxAge),
	}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return nil, err
	}
	st.LastLoginAt, st.UpdatedAt = &now, now
	if err := s.repo.SaveStaff(ctx, st); err != nil {
		return nil, err
	}
	s.audit(ctx, auth.Principal{Kind: auth.KindStaff, StaffID: st.ID}, domain.ActionSignIn, domain.TargetStaff, st.ID, "", c,
		map[string]string{"session_id": sess.ID})
	return &SignedIn{Token: raw, ExpiresAt: sess.ExpiresAt, Staff: st}, nil
}

// Authenticate resolves a session cookie to a staff principal. The session
// is checked on every request so sign-out and disabling take effect at once.
func (s *Service) Authenticate(ctx context.Context, token string) (auth.Principal, error) {
	invalid := errs.Unauthorized("admin_session_required", "sign in to the admin portal")
	if token == "" {
		return auth.Principal{}, invalid
	}
	sess, err := s.repo.GetSessionByToken(ctx, hashToken(token))
	if errors.Is(err, ErrNotFound) {
		return auth.Principal{}, invalid
	}
	if err != nil {
		return auth.Principal{}, err
	}
	now := s.now()
	if !sess.Active(now) {
		return auth.Principal{}, invalid
	}
	st, err := s.repo.GetStaff(ctx, sess.StaffID)
	if err != nil || st.Disabled() {
		return auth.Principal{}, invalid
	}
	if now.Sub(sess.LastSeenAt) >= domain.SessionTouchEvery {
		if err := s.repo.TouchSession(ctx, sess.ID, now); err != nil {
			slog.WarnContext(ctx, "touch admin session failed", "err", err)
		}
	}
	return auth.Principal{Kind: auth.KindStaff, StaffID: st.ID, SessionID: sess.ID}, nil
}

func (s *Service) Logout(ctx context.Context, p auth.Principal) error {
	return s.repo.RevokeSession(ctx, p.SessionID, s.now())
}

func (s *Service) Me(ctx context.Context, p auth.Principal) (*domain.Staff, error) {
	return s.repo.GetStaff(ctx, p.StaffID)
}

// ---------------------------------------------------------------- audit log

func (s *Service) AuditLog(ctx context.Context, f AuditFilter) ([]*domain.AuditEntry, error) {
	return s.repo.ListAudit(ctx, f)
}

// audit records a completed action. The action has already happened, so a
// failure here is logged loudly rather than reported to the caller.
func (s *Service) audit(ctx context.Context, p auth.Principal, action, targetType, targetID, reason string, c Client,
	data map[string]string) {
	if err := s.repo.AddAudit(ctx, &domain.AuditEntry{
		StaffID: p.StaffID, Action: action, TargetType: targetType, TargetID: targetID, Reason: reason,
		Data: data, IP: c.IP, CreatedAt: s.now(),
	}, nil); err != nil {
		slog.ErrorContext(ctx, "AUDIT WRITE FAILED", "action", action, "target", targetID, "staff", p.StaffID, "err", err)
	}
}

// ---------------------------------------------------------------- account actions

// record writes the audit entry for a completed action and publishes its
// notifications in the same transaction. The action itself already
// happened in another module, so a failure here is reported loudly.
func (s *Service) record(ctx context.Context, p auth.Principal, c Client, action, targetType, targetID, reason string,
	out ...contracts.Notification) error {
	if err := s.repo.AddAudit(ctx, &domain.AuditEntry{
		StaffID: p.StaffID, Action: action, TargetType: targetType, TargetID: targetID, Reason: reason,
		IP: c.IP, CreatedAt: s.now(),
	}, out); err != nil {
		slog.ErrorContext(ctx, "AUDIT WRITE FAILED", "action", action, "target", targetID, "staff", p.StaffID, "err", err)
		return fmt.Errorf("%s was applied but recording it failed: %w", action, err)
	}
	return nil
}

// notice is an email (and optionally a dashboard notification) to a customer.
func notice(tenantID, userID, to, template string, data map[string]any, inApp *contracts.InApp) contracts.Notification {
	return contracts.Notification{ID: ids.New("ntf"), TenantID: tenantID, UserID: userID, InApp: inApp,
		Email: &contracts.Email{Template: template, Category: contracts.EmailSecurity, To: to, Data: data}}
}

// owner returns the tenant owner's email and the business name.
func (s *Service) owner(ctx context.Context, tenantID string) (email, name string, err error) {
	if email, _, err = s.users.OwnerContact(ctx, tenantID); err != nil {
		return "", "", err
	}
	name, err = s.tenants.TenantName(ctx, tenantID)
	return email, name, err
}

// SuspendTenant blocks a tenant's API keys and makes its dashboard read-only.
// Payments already in flight still complete and their webhooks still go out.
func (s *Service) SuspendTenant(ctx context.Context, p auth.Principal, tenantID, reason string, c Client) error {
	reason, err := domain.ValidateReason(reason)
	if err != nil {
		return err
	}
	email, name, err := s.owner(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := s.tenants.SuspendTenant(ctx, tenantID, reason); err != nil {
		return err
	}
	return s.record(ctx, p, c, domain.ActionTenantSuspend, domain.TargetTenant, tenantID, reason,
		notice(tenantID, "", email, "tenant_suspended", map[string]any{
			"BusinessName": name, "Reason": reason, "DashboardURL": s.cfg.AppBaseURL + "/dashboard/support",
		}, &contracts.InApp{Kind: "account.suspended", Title: "Your account has been suspended", Body: reason, Link: "/dashboard/support"}))
}

func (s *Service) ReinstateTenant(ctx context.Context, p auth.Principal, tenantID, reason string, c Client) error {
	reason, err := domain.ValidateReason(reason)
	if err != nil {
		return err
	}
	email, name, err := s.owner(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := s.tenants.ReinstateTenant(ctx, tenantID); err != nil {
		return err
	}
	return s.record(ctx, p, c, domain.ActionTenantReinstate, domain.TargetTenant, tenantID, reason,
		notice(tenantID, "", email, "tenant_reinstated", map[string]any{
			"BusinessName": name, "DashboardURL": s.cfg.AppBaseURL + "/dashboard",
		}, &contracts.InApp{Kind: "account.reinstated", Title: "Your account is active again",
			Body: "Your API keys work again and you can start payments.", Link: "/dashboard"}))
}

// DisableUser blocks sign-in for one user and ends their sessions. They
// can't see the dashboard, so they are only emailed.
func (s *Service) DisableUser(ctx context.Context, p auth.Principal, userID, reason string, c Client) error {
	reason, err := domain.ValidateReason(reason)
	if err != nil {
		return err
	}
	email, tenantID, err := s.users.UserContact(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.users.DisableUser(ctx, userID, reason, c); err != nil {
		return err
	}
	return s.record(ctx, p, c, domain.ActionUserDisable, domain.TargetUser, userID, reason,
		notice(tenantID, userID, email, "account_disabled", map[string]any{"Reason": reason}, nil))
}

func (s *Service) EnableUser(ctx context.Context, p auth.Principal, userID, reason string, c Client) error {
	reason, err := domain.ValidateReason(reason)
	if err != nil {
		return err
	}
	email, tenantID, err := s.users.UserContact(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.users.EnableUser(ctx, userID, c); err != nil {
		return err
	}
	return s.record(ctx, p, c, domain.ActionUserEnable, domain.TargetUser, userID, reason,
		notice(tenantID, userID, email, "account_enabled", map[string]any{"SignInURL": s.cfg.AppBaseURL + "/auth"}, nil))
}

// ---------------------------------------------------------------- helpers

// mail sends directly (not through the outbox) for sign-in codes, which
// must not wait on Kafka.
func (s *Service) mail(ctx context.Context, to, template string, data map[string]any) {
	if err := s.mailer.Enqueue(ctx, contracts.Email{
		Template: template, Category: contracts.EmailSecurity, To: to, Data: data,
	}); err != nil {
		slog.ErrorContext(ctx, "enqueue email failed", "template", template, "err", err)
	}
}

func (s *Service) limit(ctx context.Context, key string, n int, window time.Duration) error {
	ok, err := s.limiter.Allow(ctx, key, n, window)
	if err != nil {
		slog.WarnContext(ctx, "rate limiter unavailable", "err", err)
		return nil
	}
	if !ok {
		return errs.RateLimited("too many attempts; please wait a few minutes and try again")
	}
	return nil
}

func (s *Service) hashCode(challengeID, code string) []byte {
	m := hmac.New(sha256.New, s.cfg.CodeKey)
	m.Write([]byte(challengeID + ":" + code))
	return m.Sum(nil)
}

var eat = time.FixedZone("EAT", 3*60*60)

func (s *Service) when() string { return s.now().In(eat).Format("2 Jan 2006, 15:04 MST") }

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func newCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func maskEmail(email string) string {
	local, dom, ok := strings.Cut(email, "@")
	if !ok || local == "" {
		return email
	}
	return local[:1] + strings.Repeat("•", max(1, min(len(local)-1, 4))) + "@" + dom
}

func describe(userAgent string) string {
	if userAgent == "" {
		return "Unknown device"
	}
	return truncate(userAgent, 80)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
