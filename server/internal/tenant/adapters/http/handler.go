// Package http exposes the tenant module's REST endpoints: auth, API keys,
// channels and webhook settings.
package http

import (
	"net/http"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
	"github.com/eslieh/paytrack-go/server/internal/tenant/app"
	"github.com/eslieh/paytrack-go/server/internal/tenant/domain"
)

type Handler struct {
	svc *app.Service
}

func New(svc *app.Service) *Handler { return &Handler{svc: svc} }

// Register mounts routes. authed wraps handlers that require a principal.
func (h *Handler) Register(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	mux.HandleFunc("POST /v1/auth/register", h.register)
	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.Handle("GET /v1/me", authed(http.HandlerFunc(h.me)))

	userOnly := func(f http.HandlerFunc) http.Handler { return authed(auth.RequireUser(f)) }
	mux.Handle("GET /v1/api-keys", userOnly(h.listKeys))
	mux.Handle("POST /v1/api-keys", userOnly(h.createKey))
	mux.Handle("DELETE /v1/api-keys/{id}", userOnly(h.revokeKey))

	mux.Handle("GET /v1/channels", authed(http.HandlerFunc(h.listChannels)))
	mux.Handle("POST /v1/channels", authed(http.HandlerFunc(h.createChannel)))
	mux.Handle("GET /v1/channels/{id}", authed(http.HandlerFunc(h.getChannel)))
	mux.Handle("DELETE /v1/channels/{id}", authed(http.HandlerFunc(h.deactivateChannel)))

	mux.Handle("GET /v1/webhook", authed(http.HandlerFunc(h.getWebhook)))
	mux.Handle("PUT /v1/webhook", authed(http.HandlerFunc(h.putWebhook)))
	mux.Handle("POST /v1/webhook/rotate-secret", userOnly(h.rotateSecret))
}

// ---------------------------------------------------------------- views

type tenantView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type userView struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type sessionView struct {
	Token     string     `json:"token"`
	ExpiresAt time.Time  `json:"expires_at"`
	User      userView   `json:"user"`
	Tenant    tenantView `json:"tenant"`
}

func toTenant(t *domain.Tenant) tenantView {
	return tenantView{ID: t.ID, Name: t.Name, CreatedAt: t.CreatedAt}
}

func toUser(u *domain.User) userView { return userView{ID: u.ID, Email: u.Email, Role: string(u.Role)} }

type apiKeyView struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Key        string     `json:"key,omitempty"` // only on creation
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

func toKey(k *domain.APIKey) apiKeyView {
	return apiKeyView{ID: k.ID, Name: k.Name, Prefix: k.Prefix, CreatedAt: k.CreatedAt, LastUsedAt: k.LastUsedAt, RevokedAt: k.RevokedAt}
}

// channelView never includes credentials.
type channelView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Environment string    `json:"environment"`
	ShortCode   string    `json:"short_code"`
	TillNumber  string    `json:"till_number,omitempty"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

func toChannel(c *domain.Channel) channelView {
	return channelView{ID: c.ID, Name: c.Name, Type: string(c.Type), Environment: string(c.Environment),
		ShortCode: c.ShortCode, TillNumber: c.TillNumber, Active: c.Active, CreatedAt: c.CreatedAt}
}

type webhookView struct {
	URL           string `json:"url"`
	SigningSecret string `json:"signing_secret"`
}

type list[T any] struct {
	Data []T `json:"data"`
}

// ---------------------------------------------------------------- handlers

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BusinessName string `json:"business_name"`
		Email        string `json:"email"`
		Password     string `json:"password"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	s, err := h.svc.Register(r.Context(), app.RegisterCmd{BusinessName: req.BusinessName, Email: req.Email, Password: req.Password})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, sessionView{s.Token, s.ExpiresAt, toUser(s.User), toTenant(s.Tenant)})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	s, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sessionView{s.Token, s.ExpiresAt, toUser(s.User), toTenant(s.Tenant)})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	t, u, err := h.svc.Me(r.Context(), p)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	resp := struct {
		Tenant   tenantView `json:"tenant"`
		User     *userView  `json:"user"`
		AuthKind string     `json:"auth_kind"`
	}{Tenant: toTenant(t), AuthKind: string(p.Kind)}
	if u != nil {
		uv := toUser(u)
		resp.User = &uv
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handler) listKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.svc.ListAPIKeys(r.Context(), auth.MustTenant(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := list[apiKeyView]{Data: []apiKeyView{}}
	for _, k := range keys {
		out.Data = append(out.Data, toKey(k))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) createKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	k, plain, err := h.svc.CreateAPIKey(r.Context(), auth.MustTenant(r.Context()), req.Name)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	v := toKey(k)
	v.Key = plain
	httpx.JSON(w, http.StatusCreated, v)
}

func (h *Handler) revokeKey(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RevokeAPIKey(r.Context(), auth.MustTenant(r.Context()), r.PathValue("id")); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listChannels(w http.ResponseWriter, r *http.Request) {
	cs, err := h.svc.ListChannels(r.Context(), auth.MustTenant(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := list[channelView]{Data: []channelView{}}
	for _, c := range cs {
		out.Data = append(out.Data, toChannel(c))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) createChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		Type           string `json:"type"`
		Environment    string `json:"environment"`
		ShortCode      string `json:"short_code"`
		TillNumber     string `json:"till_number"`
		ConsumerKey    string `json:"consumer_key"`
		ConsumerSecret string `json:"consumer_secret"`
		Passkey        string `json:"passkey"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	c, err := h.svc.RegisterChannel(r.Context(), auth.MustTenant(r.Context()), domain.NewChannelParams{
		Name: req.Name, Type: domain.ChannelType(req.Type), Environment: domain.Environment(req.Environment),
		ShortCode: req.ShortCode, TillNumber: req.TillNumber,
		Credentials: domain.Credentials{ConsumerKey: req.ConsumerKey, ConsumerSecret: req.ConsumerSecret, Passkey: req.Passkey},
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toChannel(c))
}

func (h *Handler) getChannel(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.GetChannel(r.Context(), auth.MustTenant(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toChannel(c))
}

func (h *Handler) deactivateChannel(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeactivateChannel(r.Context(), auth.MustTenant(r.Context()), r.PathValue("id")); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getWebhook(w http.ResponseWriter, r *http.Request) {
	wh, err := h.svc.GetWebhook(r.Context(), auth.MustTenant(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, webhookView{wh.URL, wh.Secret})
}

func (h *Handler) putWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	wh, err := h.svc.SetWebhookURL(r.Context(), auth.MustTenant(r.Context()), req.URL)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, webhookView{wh.URL, wh.Secret})
}

func (h *Handler) rotateSecret(w http.ResponseWriter, r *http.Request) {
	wh, err := h.svc.RotateWebhookSecret(r.Context(), auth.MustTenant(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, webhookView{wh.URL, wh.Secret})
}
