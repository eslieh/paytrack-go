// Package http exposes the admin portal API under /admin/v1. Staff sessions
// use an opaque httpOnly cookie scoped to the admin API path, so tenant
// routes never see it and tenant bearer tokens never reach these routes.
// State-changing requests must also carry the X-Paytrack-Admin header,
// which cross-site forms cannot set.
package http

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/admin/app"
	"github.com/eslieh/paytrack-go/server/internal/admin/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
)

const (
	sessionCookie = "pt_admin"
	csrfHeader    = "X-Paytrack-Admin"
)

type Config struct {
	CookieSecure  bool
	TrustXFF      bool
	APIPublicPath string       // e.g. "/api"; the cookie is scoped to <path>/admin
	AllowedCIDRs  []*net.IPNet // empty allows any address
}

type Handler struct {
	svc *app.Service
	cfg Config
}

func New(svc *app.Service, cfg Config) *Handler { return &Handler{svc: svc, cfg: cfg} }

func (h *Handler) Register(mux *http.ServeMux) {
	pub := func(f http.HandlerFunc) http.Handler { return h.guard(f) }
	staff := func(f http.HandlerFunc) http.Handler { return h.guard(h.authed(f)) }

	mux.Handle("POST /admin/v1/auth/login", pub(h.login))
	mux.Handle("POST /admin/v1/auth/verify", pub(h.verify))
	mux.Handle("POST /admin/v1/auth/logout", staff(h.logout))
	mux.Handle("GET /admin/v1/me", staff(h.me))
	mux.Handle("GET /admin/v1/audit-log", staff(h.auditLog))
	mux.Handle("POST /admin/v1/tenants/{id}/suspend", staff(h.action(h.svc.SuspendTenant)))
	mux.Handle("POST /admin/v1/tenants/{id}/reinstate", staff(h.action(h.svc.ReinstateTenant)))
	mux.Handle("POST /admin/v1/users/{id}/disable", staff(h.action(h.svc.DisableUser)))
	mux.Handle("POST /admin/v1/users/{id}/enable", staff(h.action(h.svc.EnableUser)))
}

// Staff wraps another module's admin route (e.g. the support queue) with
// the same allowlist, CSRF check and staff session as this module's routes.
func (h *Handler) Staff(next http.Handler) http.Handler { return h.guard(h.authed(next.ServeHTTP)) }

// guard applies the IP allowlist and the CSRF header check.
func (h *Handler) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(h.cfg.AllowedCIDRs) > 0 && !h.allowedIP(r) {
			httpx.WriteError(w, r, errs.NotFound("not_found", "not found"))
			return
		}
		if r.Method != http.MethodGet && r.Header.Get(csrfHeader) != "1" {
			httpx.WriteError(w, r, errs.Forbidden("csrf_header_required", csrfHeader+": 1 header required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) allowedIP(r *http.Request) bool {
	ip := net.ParseIP(httpx.ClientIP(r, h.cfg.TrustXFF))
	if ip == nil {
		return false
	}
	for _, n := range h.cfg.AllowedCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func (h *Handler) authed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var token string
		if c, err := r.Cookie(sessionCookie); err == nil {
			token = c.Value
		}
		p, err := h.svc.Authenticate(r.Context(), token)
		if err != nil {
			h.clearCookie(w)
			httpx.WriteError(w, r, err)
			return
		}
		next(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
	}
}

func (h *Handler) client(r *http.Request) app.Client {
	return app.Client{IP: httpx.ClientIP(r, h.cfg.TrustXFF), UserAgent: r.UserAgent()}
}

func (h *Handler) cookiePath() string { return h.cfg.APIPublicPath + "/admin" }

func (h *Handler) setCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: value, Path: h.cookiePath(), HttpOnly: true, Secure: h.cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode, Expires: expires,
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: h.cookiePath(), HttpOnly: true,
		Secure: h.cfg.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}

func principal(r *http.Request) auth.Principal {
	p, _ := auth.FromContext(r.Context())
	return p
}

func decode[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := httpx.Decode(r, &v); err != nil {
		httpx.WriteError(w, r, err)
		return v, false
	}
	return v, true
}

// ---------------------------------------------------------------- views

type staffView struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

func toStaff(s *domain.Staff) staffView {
	return staffView{ID: s.ID, Email: s.Email, Name: s.Name, LastLoginAt: s.LastLoginAt}
}

type auditView struct {
	ID         int64             `json:"id"`
	StaffID    string            `json:"staff_id"`
	Action     string            `json:"action"`
	TargetType string            `json:"target_type"`
	TargetID   string            `json:"target_id"`
	Reason     string            `json:"reason"`
	Data       map[string]string `json:"data"`
	IP         string            `json:"ip"`
	CreatedAt  time.Time         `json:"created_at"`
}

// ---------------------------------------------------------------- auth

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}](w, r)
	if !ok {
		return
	}
	ch, err := h.svc.Login(r.Context(), req.Email, req.Password, h.client(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"challenge": map[string]any{
		"id": ch.ID, "masked_email": ch.MaskedEmail, "expires_at": ch.ExpiresAt,
	}})
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		ChallengeID string `json:"challenge_id"`
		Code        string `json:"code"`
	}](w, r)
	if !ok {
		return
	}
	in, err := h.svc.VerifyLogin(r.Context(), req.ChallengeID, req.Code, h.client(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.setCookie(w, in.Token, in.ExpiresAt)
	httpx.JSON(w, http.StatusOK, map[string]any{"staff": toStaff(in.Staff)})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context(), principal(r)); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Me(r.Context(), principal(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"staff": toStaff(st)})
}

// ---------------------------------------------------------------- audit log

func (h *Handler) auditLog(w http.ResponseWriter, r *http.Request) {
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	entries, err := h.svc.AuditLog(r.Context(), app.AuditFilter{
		TargetType: httpx.TrimmedQuery(r, "target_type"), TargetID: httpx.TrimmedQuery(r, "target_id"),
		BeforeID: before, Limit: httpx.QueryLimit(r, 50, 200),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]auditView, 0, len(entries))
	for _, e := range entries {
		out = append(out, auditView(*e))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// ---------------------------------------------------------------- account actions

type reasonReq struct {
	Reason string `json:"reason"`
}

// action runs one account action that takes a reason and returns 204.
func (h *Handler) action(fn func(context.Context, auth.Principal, string, string, app.Client) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, ok := decode[reasonReq](w, r)
		if !ok {
			return
		}
		if err := fn(r.Context(), principal(r), r.PathValue("id"), req.Reason, h.client(r)); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
