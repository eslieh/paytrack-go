package http

import (
	"net/http"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
	"github.com/eslieh/paytrack-go/server/internal/tenant/app"
	"github.com/eslieh/paytrack-go/server/internal/tenant/domain"
)

// RegisterAdmin mounts staff-only lookups behind staff (the admin module's
// session check). Suspension lives in the admin module, which audits it.
func (h *Handler) RegisterAdmin(mux *http.ServeMux, staff func(http.Handler) http.Handler) {
	mux.Handle("GET /admin/v1/tenants", staff(http.HandlerFunc(h.adminTenants)))
	mux.Handle("GET /admin/v1/tenants/{id}", staff(http.HandlerFunc(h.adminTenant)))
	mux.Handle("GET /admin/v1/stats/tenants", staff(httpx.StatsHandler(h.svc.Stats)))
}

// adminTenantView never includes the webhook signing secret.
type adminTenantView struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	SuspendedAt     *time.Time `json:"suspended_at"`
	SuspendedReason string     `json:"suspended_reason,omitempty"`
	WebhookURL      string     `json:"webhook_url,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

func toAdminTenant(t *domain.Tenant) adminTenantView {
	return adminTenantView{ID: t.ID, Name: t.Name, Status: string(t.Status), SuspendedAt: t.SuspendedAt,
		SuspendedReason: t.SuspendedReason, WebhookURL: t.WebhookURL, CreatedAt: t.CreatedAt}
}

func (h *Handler) adminTenants(w http.ResponseWriter, r *http.Request) {
	limit := httpx.QueryLimit(r, 50, 200)
	ts, err := h.svc.ListTenants(r.Context(), app.TenantFilter{
		Query: httpx.TrimmedQuery(r, "q"), Status: domain.Status(httpx.TrimmedQuery(r, "status")),
		Before: httpx.TrimmedQuery(r, "cursor"), Limit: limit + 1,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := struct {
		Data       []adminTenantView `json:"data"`
		HasMore    bool              `json:"has_more"`
		NextCursor string            `json:"next_cursor,omitempty"`
	}{Data: []adminTenantView{}}
	if len(ts) > limit {
		ts, out.HasMore = ts[:limit], true
		out.NextCursor = ts[limit-1].ID
	}
	for _, t := range ts {
		out.Data = append(out.Data, toAdminTenant(t))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) adminTenant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t, err := h.svc.Tenant(ctx, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	channels, err := h.svc.ListChannels(ctx, t.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	keys, err := h.svc.ListAPIKeys(ctx, t.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	cv := make([]channelView, 0, len(channels))
	for _, c := range channels {
		cv = append(cv, toChannel(c))
	}
	kv := make([]apiKeyView, 0, len(keys))
	for _, k := range keys {
		kv = append(kv, toKey(k))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"tenant": toAdminTenant(t), "channels": cv, "api_keys": kv})
}
