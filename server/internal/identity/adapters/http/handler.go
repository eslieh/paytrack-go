// Package http exposes the identity endpoints. Dashboard sessions use a
// short-lived access token (JSON) plus an httpOnly refresh cookie; the
// dashboard reaches the API through its own /api proxy so cookies are
// first-party.
package http

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/identity/app"
	"github.com/eslieh/paytrack-go/server/internal/identity/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
)

const (
	refreshCookie = "pt_refresh"
	deviceCookie  = "pt_device"
	stateCookie   = "pt_oauth_state"
)

type Config struct {
	CookieSecure   bool
	TrustXFF       bool
	AllowedOrigins []string // dashboard origins allowed as OAuth return targets
	AppBaseURL     string
	APIPublicPath  string // e.g. "/api"
}

// TenantNamer resolves tenant names for API-key principals on /v1/me.
type TenantNamer interface {
	TenantName(ctx context.Context, id string) (string, error)
}

type Handler struct {
	svc     *app.Service
	tenants TenantNamer
	cfg     Config
}

func New(svc *app.Service, tenants TenantNamer, cfg Config) *Handler {
	return &Handler{svc: svc, tenants: tenants, cfg: cfg}
}

func (h *Handler) Register(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	mux.HandleFunc("GET /v1/auth/providers", h.providers)
	mux.HandleFunc("POST /v1/auth/register", h.register)
	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.HandleFunc("POST /v1/auth/login/verify", h.loginVerify)
	mux.HandleFunc("POST /v1/auth/login/resend", h.loginResend)
	mux.HandleFunc("POST /v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /v1/auth/logout", h.logout)
	mux.HandleFunc("POST /v1/auth/verify-email", h.verifyEmail)
	mux.HandleFunc("POST /v1/auth/password/forgot", h.forgot)
	mux.HandleFunc("POST /v1/auth/password/reset", h.reset)
	mux.HandleFunc("GET /v1/auth/google/start", h.googleStart)
	mux.HandleFunc("GET /v1/auth/google/callback", h.googleCallback)
	mux.HandleFunc("GET /v1/auth/google/signup", h.googleSignupInfo)
	mux.HandleFunc("POST /v1/auth/google/complete", h.googleComplete)

	mux.Handle("GET /v1/me", authed(http.HandlerFunc(h.me)))
	user := func(f http.HandlerFunc) http.Handler { return authed(auth.RequireUser(f)) }
	mux.Handle("POST /v1/auth/verify-email/resend", user(h.resendVerification))
	mux.Handle("POST /v1/me/password", user(h.changePassword))
	mux.Handle("PUT /v1/me/two-step", user(h.setTwoStep))
	mux.Handle("GET /v1/me/sessions", user(h.sessions))
	mux.Handle("DELETE /v1/me/sessions/{id}", user(h.revokeSession))
	mux.Handle("POST /v1/me/sessions/revoke-others", user(h.revokeOthers))
	mux.Handle("GET /v1/me/security-events", user(h.securityEvents))
	mux.Handle("POST /v1/me/identities/google", user(h.linkGoogle))
	mux.Handle("DELETE /v1/me/identities/google", user(h.unlinkGoogle))
}

// ---------------------------------------------------------------- helpers

func (h *Handler) client(r *http.Request) app.Client {
	return app.Client{IP: httpx.ClientIP(r, h.cfg.TrustXFF), UserAgent: r.UserAgent()}
}

func (h *Handler) setCookie(w http.ResponseWriter, name, value string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", HttpOnly: true, Secure: h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode, MaxAge: int(maxAge.Seconds()),
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, Secure: h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

func cookie(r *http.Request, name string) string {
	if c, err := r.Cookie(name); err == nil {
		return c.Value
	}
	return ""
}

type userView struct {
	ID             string     `json:"id"`
	Email          string     `json:"email"`
	Role           string     `json:"role"`
	EmailVerified  bool       `json:"email_verified"`
	TwoStepEnabled bool       `json:"two_step_enabled"`
	HasPassword    bool       `json:"has_password"`
	CreatedAt      time.Time  `json:"created_at"`
	LastLoginAt    *time.Time `json:"last_login_at"`
}

func toUser(u *domain.User) userView {
	return userView{ID: u.ID, Email: u.Email, Role: string(u.Role), EmailVerified: u.Verified(),
		TwoStepEnabled: u.TwoStepEnabled, HasPassword: u.HasPassword(), CreatedAt: u.CreatedAt, LastLoginAt: u.LastLoginAt}
}

type tenantView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Suspended bool   `json:"suspended"` // dashboard is read-only
}

type sessionView struct {
	Token     string     `json:"token"`
	ExpiresAt time.Time  `json:"expires_at"`
	User      userView   `json:"user"`
	Tenant    tenantView `json:"tenant"`
}

// signedIn writes the session: access token in JSON, refresh (and device)
// tokens as httpOnly cookies.
func (h *Handler) signedIn(w http.ResponseWriter, a *app.Auth, status int) {
	h.setCookies(w, a)
	httpx.JSON(w, status, sessionView{a.AccessToken, a.AccessExpiresAt, toUser(a.User), tenantView{a.User.TenantID, a.TenantName, a.TenantSuspended}})
}

func (h *Handler) setCookies(w http.ResponseWriter, a *app.Auth) {
	h.setCookie(w, refreshCookie, a.RefreshToken, time.Until(a.RefreshExpiresAt))
	if a.DeviceToken != "" {
		h.setCookie(w, deviceCookie, a.DeviceToken, domain.TrustedDeviceTTL)
	}
}

func decode[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := httpx.Decode(r, &v); err != nil {
		httpx.WriteError(w, r, err)
		return v, false
	}
	return v, true
}

// ---------------------------------------------------------------- sign-up / sign-in

func (h *Handler) providers(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]bool{"password": true, "google": h.svc.GoogleEnabled()})
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		BusinessName string `json:"business_name"`
		Email        string `json:"email"`
		Password     string `json:"password"`
	}](w, r)
	if !ok {
		return
	}
	a, err := h.svc.Register(r.Context(), app.RegisterCmd{BusinessName: req.BusinessName, Email: req.Email, Password: req.Password}, h.client(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.signedIn(w, a, http.StatusCreated)
}

type challengeView struct {
	ID          string    `json:"id"`
	MaskedEmail string    `json:"masked_email"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}](w, r)
	if !ok {
		return
	}
	res, err := h.svc.Login(r.Context(), req.Email, req.Password, h.client(r), cookie(r, deviceCookie))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if res.Challenge != nil {
		httpx.JSON(w, http.StatusOK, map[string]any{"mfa_required": true,
			"challenge": challengeView{res.Challenge.ID, res.Challenge.MaskedEmail, res.Challenge.ExpiresAt}})
		return
	}
	h.signedIn(w, res.Auth, http.StatusOK)
}

func (h *Handler) loginVerify(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		ChallengeID    string `json:"challenge_id"`
		Code           string `json:"code"`
		RememberDevice bool   `json:"remember_device"`
	}](w, r)
	if !ok {
		return
	}
	a, err := h.svc.VerifyLogin(r.Context(), req.ChallengeID, req.Code, req.RememberDevice, h.client(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.signedIn(w, a, http.StatusOK)
}

func (h *Handler) loginResend(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		ChallengeID string `json:"challenge_id"`
	}](w, r)
	if !ok {
		return
	}
	ch, err := h.svc.ResendLoginCode(r.Context(), req.ChallengeID, h.client(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"challenge": challengeView{ch.ID, ch.MaskedEmail, ch.ExpiresAt}})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	a, err := h.svc.Refresh(r.Context(), cookie(r, refreshCookie), h.client(r))
	if err != nil {
		if errs.IsKind(err, errs.KindUnauthorized) {
			h.clearCookie(w, refreshCookie)
		}
		httpx.WriteError(w, r, err)
		return
	}
	h.signedIn(w, a, http.StatusOK)
}

// logout revokes the session on the server, then clears the cookie. It
// accepts the access token or just the refresh cookie (expired access token).
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var p *auth.Principal
	if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && tok != "" {
		if got, err := h.svc.Authenticate(r.Context(), tok); err == nil {
			p = &got
		}
	}
	if err := h.svc.Logout(r.Context(), p, cookie(r, refreshCookie), h.client(r)); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.clearCookie(w, refreshCookie)
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------- verification & recovery

func (h *Handler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Token string `json:"token"`
	}](w, r)
	if !ok {
		return
	}
	u, err := h.svc.VerifyEmail(r.Context(), req.Token, h.client(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"email_verified": true, "email": u.Email})
}

func (h *Handler) resendVerification(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	if err := h.svc.ResendVerification(r.Context(), p); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) forgot(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Email string `json:"email"`
	}](w, r)
	if !ok {
		return
	}
	if err := h.svc.ForgotPassword(r.Context(), req.Email, h.client(r)); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]string{"status": "If an account exists for that email, we've sent a reset link."})
}

func (h *Handler) reset(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}](w, r)
	if !ok {
		return
	}
	a, err := h.svc.ResetPassword(r.Context(), req.Token, req.Password, h.client(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.clearCookie(w, deviceCookie)
	h.signedIn(w, a, http.StatusOK)
}

// ---------------------------------------------------------------- account

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	if p.Kind == auth.KindAPIKey {
		name, err := h.tenants.TenantName(r.Context(), p.TenantID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"tenant": tenantView{p.TenantID, name, false}, "user": nil, "auth_kind": p.Kind})
		return
	}
	prof, err := h.svc.Me(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	idents := []map[string]any{}
	for _, i := range prof.Identities {
		idents = append(idents, map[string]any{"provider": i.Provider, "email": i.Email, "linked_at": i.CreatedAt})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"tenant": tenantView{prof.User.TenantID, prof.TenantName, p.Suspended}, "user": toUser(prof.User),
		"auth_kind": p.Kind, "identities": idents, "google_enabled": h.svc.GoogleEnabled(),
	})
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}](w, r)
	if !ok {
		return
	}
	p, _ := auth.FromContext(r.Context())
	if err := h.svc.ChangePassword(r.Context(), p, req.CurrentPassword, req.NewPassword, h.client(r)); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.clearCookie(w, deviceCookie)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setTwoStep(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Enabled  bool   `json:"enabled"`
		Password string `json:"password"`
	}](w, r)
	if !ok {
		return
	}
	p, _ := auth.FromContext(r.Context())
	if err := h.svc.SetTwoStep(r.Context(), p, req.Enabled, req.Password, h.client(r)); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"two_step_enabled": req.Enabled})
}

func (h *Handler) sessions(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	list, err := h.svc.ListSessions(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := []map[string]any{}
	for _, s := range list {
		out = append(out, map[string]any{"id": s.ID, "device": s.Device, "ip": s.IP, "auth_method": s.AuthMethod,
			"created_at": s.CreatedAt, "last_seen_at": s.LastSeenAt, "expires_at": s.ExpiresAt, "current": s.Current})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	if err := h.svc.RevokeSession(r.Context(), p, r.PathValue("id"), h.client(r)); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) revokeOthers(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	if err := h.svc.RevokeOtherSessions(r.Context(), p, h.client(r)); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) securityEvents(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	evs, err := h.svc.SecurityEvents(r.Context(), p, httpx.QueryLimit(r, 25, 100))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := []map[string]any{}
	for _, e := range evs {
		out = append(out, map[string]any{"id": e.ID, "type": e.Type, "ip": e.IP, "device": domain.DescribeDevice(e.UserAgent),
			"data": e.Data, "created_at": e.CreatedAt})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// ---------------------------------------------------------------- Google

// origin picks the dashboard origin to return to: an explicit ?origin= (or
// body field) that is allow-listed, else the canonical app URL.
func (h *Handler) origin(requested string) string {
	requested = strings.TrimRight(requested, "/")
	if requested != "" && (slices.Contains(h.cfg.AllowedOrigins, requested) || requested == h.cfg.AppBaseURL) {
		return requested
	}
	return h.cfg.AppBaseURL
}

func (h *Handler) redirectURI(origin string) string {
	return origin + h.cfg.APIPublicPath + "/v1/auth/google/callback"
}

func safeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") && !strings.Contains(next, "\\") {
		return next
	}
	return "/dashboard"
}

func (h *Handler) googleStart(w http.ResponseWriter, r *http.Request) {
	origin := h.origin(r.URL.Query().Get("origin"))
	u, state, err := h.svc.GoogleStart(r.Context(), app.IntentSignin, safeNext(r.URL.Query().Get("next")), origin, h.redirectURI(origin), "")
	if err != nil {
		http.Redirect(w, r, origin+"/auth?error="+url.QueryEscape(errorCode(err)), http.StatusFound)
		return
	}
	h.setCookie(w, stateCookie, state, domain.OAuthStateTTL)
	http.Redirect(w, r, u, http.StatusFound)
}

func (h *Handler) linkGoogle(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Origin string `json:"origin"`
	}](w, r)
	if !ok {
		return
	}
	p, _ := auth.FromContext(r.Context())
	origin := h.origin(req.Origin)
	u, state, err := h.svc.GoogleStart(r.Context(), app.IntentLink, "/dashboard/settings", origin, h.redirectURI(origin), p.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.setCookie(w, stateCookie, state, domain.OAuthStateTTL)
	httpx.JSON(w, http.StatusOK, map[string]string{"url": u})
}

func (h *Handler) googleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cookieState := cookie(r, stateCookie)
	h.clearCookie(w, stateCookie)
	if q.Get("error") != "" { // user cancelled at Google
		http.Redirect(w, r, h.cfg.AppBaseURL+"/auth?error=google_cancelled", http.StatusFound)
		return
	}
	out, err := h.svc.GoogleCallback(r.Context(), q.Get("state"), cookieState, q.Get("code"), h.client(r))
	origin := h.cfg.AppBaseURL
	if out != nil && out.Origin != "" {
		origin = out.Origin
	}
	switch {
	case err != nil:
		target := "/auth?error="
		if out != nil && out.Next == "/dashboard/settings" {
			target = "/dashboard/settings?google_error="
		}
		http.Redirect(w, r, origin+target+url.QueryEscape(errorCode(err)), http.StatusFound)
	case out.Linked:
		http.Redirect(w, r, origin+"/dashboard/settings?google=linked", http.StatusFound)
	case out.SignupToken != "":
		http.Redirect(w, r, origin+"/auth/google/complete?token="+url.QueryEscape(out.SignupToken), http.StatusFound)
	default:
		h.setCookies(w, out.Auth)
		http.Redirect(w, r, origin+safeNext(out.Next), http.StatusFound)
	}
}

func errorCode(err error) string {
	if e, ok := errs.As(err); ok && e.Kind != errs.KindInternal {
		return e.Code
	}
	return "google_failed"
}

func (h *Handler) googleSignupInfo(w http.ResponseWriter, r *http.Request) {
	email, name, err := h.svc.GoogleSignupInfo(r.Context(), r.URL.Query().Get("token"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"email": email, "name": name})
}

func (h *Handler) googleComplete(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Token        string `json:"token"`
		BusinessName string `json:"business_name"`
	}](w, r)
	if !ok {
		return
	}
	a, err := h.svc.GoogleComplete(r.Context(), req.Token, req.BusinessName, h.client(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.signedIn(w, a, http.StatusCreated)
}

func (h *Handler) unlinkGoogle(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	if err := h.svc.UnlinkGoogle(r.Context(), p, h.client(r)); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
