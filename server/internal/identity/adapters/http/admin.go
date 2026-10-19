package http

import (
	"net/http"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/identity/app"
	"github.com/eslieh/paytrack-go/server/internal/identity/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
)

// RegisterAdmin mounts staff-only lookups behind staff (the admin module's
// session check). Account actions live in the admin module, which audits
// them.
func (h *Handler) RegisterAdmin(mux *http.ServeMux, staff func(http.Handler) http.Handler) {
	mux.Handle("GET /admin/v1/users", staff(http.HandlerFunc(h.adminUsers)))
	mux.Handle("GET /admin/v1/users/{id}", staff(http.HandlerFunc(h.adminUser)))
	mux.Handle("GET /admin/v1/stats/users", staff(httpx.StatsHandler(h.svc.Stats)))
}

type adminUserView struct {
	userView
	TenantID       string     `json:"tenant_id"`
	DisabledAt     *time.Time `json:"disabled_at"`
	DisabledReason string     `json:"disabled_reason,omitempty"`
}

func toAdminUser(u *domain.User) adminUserView {
	return adminUserView{userView: toUser(u), TenantID: u.TenantID, DisabledAt: u.DisabledAt, DisabledReason: u.DisabledReason}
}

func (h *Handler) adminUsers(w http.ResponseWriter, r *http.Request) {
	f := app.UserFilter{TenantID: httpx.TrimmedQuery(r, "tenant_id"), EmailQuery: httpx.TrimmedQuery(r, "q"),
		Limit: httpx.QueryLimit(r, 50, 200)}
	if f.TenantID == "" && len(f.EmailQuery) < 3 {
		httpx.WriteError(w, r, errs.Invalid("query_too_short", "search with at least 3 characters, or pass tenant_id"))
		return
	}
	users, err := h.svc.ListUsers(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]adminUserView, 0, len(users))
	for _, u := range users {
		out = append(out, toAdminUser(u))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) adminUser(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.UserDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	idents := []map[string]any{}
	for _, i := range d.Identities {
		idents = append(idents, map[string]any{"provider": i.Provider, "email": i.Email, "linked_at": i.CreatedAt})
	}
	sessions := []map[string]any{}
	for _, s := range d.Sessions {
		sessions = append(sessions, map[string]any{"id": s.ID, "device": s.Device, "ip": s.IP, "auth_method": s.AuthMethod,
			"created_at": s.CreatedAt, "last_seen_at": s.LastSeenAt, "expires_at": s.ExpiresAt})
	}
	events := []map[string]any{}
	for _, e := range d.Events {
		events = append(events, map[string]any{"id": e.ID, "type": e.Type, "ip": e.IP, "device": domain.DescribeDevice(e.UserAgent),
			"data": e.Data, "created_at": e.CreatedAt})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"user": toAdminUser(d.User), "identities": idents, "sessions": sessions, "security_events": events,
	})
}
