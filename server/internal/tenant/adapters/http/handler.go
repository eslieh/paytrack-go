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
	svc     *app.Service
	samples func(template string) map[string]any // sample data for email previews
}

func New(svc *app.Service, samples func(template string) map[string]any) *Handler {
	return &Handler{svc: svc, samples: samples}
}

// Register mounts routes. authed wraps handlers that require a principal.
func (h *Handler) Register(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	userOnly := func(f http.HandlerFunc) http.Handler { return authed(auth.RequireUser(f)) }
	verified := func(f http.HandlerFunc) http.Handler { return authed(auth.RequireVerified(f)) }
	mux.Handle("GET /v1/api-keys", userOnly(h.listKeys))
	mux.Handle("POST /v1/api-keys", authed(auth.RequireUser(auth.RequireVerified(http.HandlerFunc(h.createKey)))))
	mux.Handle("DELETE /v1/api-keys/{id}", userOnly(h.revokeKey))

	mux.HandleFunc("GET /v1/banks", h.banks)
	mux.Handle("GET /v1/channels", authed(http.HandlerFunc(h.listChannels)))
	mux.Handle("POST /v1/channels", verified(h.createChannel))
	mux.Handle("GET /v1/channels/{id}", authed(http.HandlerFunc(h.getChannel)))
	mux.Handle("DELETE /v1/channels/{id}", authed(http.HandlerFunc(h.deactivateChannel)))

	mux.Handle("GET /v1/webhook", authed(http.HandlerFunc(h.getWebhook)))
	mux.Handle("PUT /v1/webhook", authed(http.HandlerFunc(h.putWebhook)))
	mux.Handle("POST /v1/webhook/rotate-secret", userOnly(h.rotateSecret))

	mux.Handle("GET /v1/onboarding", authed(http.HandlerFunc(h.onboarding)))
	mux.Handle("GET /v1/settings/notifications", userOnly(h.getNotifications))
	mux.Handle("PUT /v1/settings/notifications", userOnly(h.putNotifications))
	mux.Handle("GET /v1/settings/notifications/preview", userOnly(h.previewNotification))
	mux.Handle("POST /v1/settings/notifications/test-receipt", userOnly(h.testReceipt))
}

// ---------------------------------------------------------------- views

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

type channelView struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	BusinessNumber string    `json:"business_number"` // PartyB
	BankName       string    `json:"bank_name,omitempty"`
	AccountNumber  string    `json:"account_number,omitempty"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
}

func toChannel(c *domain.Channel) channelView {
	return channelView{ID: c.ID, Name: c.Name, Type: string(c.Type), BusinessNumber: c.PartyB,
		BankName: c.BankName, AccountNumber: c.AccountNumber, Active: c.Active, CreatedAt: c.CreatedAt}
}

type webhookView struct {
	URL           string `json:"url"`
	SigningSecret string `json:"signing_secret"`
}

type list[T any] struct {
	Data []T `json:"data"`
}

// ---------------------------------------------------------------- handlers

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

func (h *Handler) banks(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, list[domain.Bank]{Data: h.svc.Banks()})
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
		BankCode       string `json:"bank_code"`
		BusinessNumber string `json:"business_number"`
		AccountNumber  string `json:"account_number"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	c, err := h.svc.RegisterChannel(r.Context(), auth.MustTenant(r.Context()), domain.NewChannelParams{
		Name: req.Name, Type: domain.ChannelType(req.Type), BankCode: req.BankCode,
		BusinessNumber: req.BusinessNumber, AccountNumber: req.AccountNumber,
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

// ---------------------------------------------------------------- onboarding & notifications

func (h *Handler) onboarding(w http.ResponseWriter, r *http.Request) {
	steps, err := h.svc.Onboarding(r.Context(), auth.MustTenant(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]map[string]any, len(steps))
	complete := true
	for i, st := range steps {
		out[i] = map[string]any{"key": st.Key, "label": st.Label, "done": st.Done, "href": st.Path}
		complete = complete && st.Done
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"steps": out, "complete": complete})
}

type notificationsView struct {
	PaymentReceipts      bool      `json:"payment_receipts"`
	ReceiptEmail         string    `json:"receipt_email"`
	ReceiptRecipient     string    `json:"receipt_recipient"` // effective address
	WebhookFailureAlerts bool      `json:"webhook_failure_alerts"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (h *Handler) notificationsView(r *http.Request, s domain.NotificationSettings) (notificationsView, error) {
	to, err := h.svc.ReceiptRecipient(r.Context(), s.TenantID)
	return notificationsView{s.PaymentReceipts, s.ReceiptEmail, to, s.WebhookFailureAlerts, s.UpdatedAt}, err
}

func (h *Handler) getNotifications(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.NotificationSettings(r.Context(), auth.MustTenant(r.Context()))
	if err == nil {
		var v notificationsView
		if v, err = h.notificationsView(r, s); err == nil {
			httpx.JSON(w, http.StatusOK, v)
			return
		}
	}
	httpx.WriteError(w, r, err)
}

func (h *Handler) putNotifications(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PaymentReceipts      *bool   `json:"payment_receipts"`
		ReceiptEmail         *string `json:"receipt_email"`
		WebhookFailureAlerts *bool   `json:"webhook_failure_alerts"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	s, err := h.svc.UpdateNotificationSettings(r.Context(), auth.MustTenant(r.Context()), app.NotificationUpdate{
		PaymentReceipts: req.PaymentReceipts, ReceiptEmail: req.ReceiptEmail, WebhookFailureAlerts: req.WebhookFailureAlerts,
	})
	if err == nil {
		var v notificationsView
		if v, err = h.notificationsView(r, s); err == nil {
			httpx.JSON(w, http.StatusOK, v)
			return
		}
	}
	httpx.WriteError(w, r, err)
}

// previewNotification returns rendered HTML for the settings preview iframe.
func (h *Handler) previewNotification(w http.ResponseWriter, r *http.Request) {
	tpl := r.URL.Query().Get("template")
	if tpl == "" {
		tpl = "payment_receipt"
	}
	out, err := h.svc.PreviewEmail(r.Context(), auth.MustTenant(r.Context()), tpl, h.samples(tpl))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"subject": out.Subject, "html": out.HTML, "text": out.Text})
}

func (h *Handler) testReceipt(w http.ResponseWriter, r *http.Request) {
	to, err := h.svc.SendTestReceipt(r.Context(), auth.MustTenant(r.Context()), h.samples("payment_receipt"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]string{"sent_to": to})
}
