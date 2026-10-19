// Package http exposes webhook delivery history and manual redelivery.
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/notify/app"
	"github.com/eslieh/paytrack-go/server/internal/notify/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
)

type Handler struct{ svc *app.Service }

func New(svc *app.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/webhook/deliveries", authed(http.HandlerFunc(h.list)))
	mux.Handle("POST /v1/webhook/deliveries/{id}/redeliver", authed(http.HandlerFunc(h.redeliver)))
}

type deliveryView struct {
	ID             string          `json:"id"`
	EventID        string          `json:"event_id"`
	EventType      string          `json:"event_type"`
	PaymentID      string          `json:"payment_id"`
	Status         string          `json:"status"`
	Attempts       int             `json:"attempts"`
	NextAttemptAt  *time.Time      `json:"next_attempt_at"`
	LastError      string          `json:"last_error,omitempty"`
	LastStatusCode int             `json:"last_status_code,omitempty"`
	DeliveredAt    *time.Time      `json:"delivered_at"`
	CreatedAt      time.Time       `json:"created_at"`
	Payload        json.RawMessage `json:"payload"`
}

func toView(d *domain.Delivery) deliveryView {
	return deliveryView{ID: d.ID, EventID: d.EventID, EventType: d.EventType, PaymentID: d.PaymentID,
		Status: string(d.Status), Attempts: d.Attempts, NextAttemptAt: d.NextAttemptAt, LastError: d.LastError,
		LastStatusCode: d.LastStatusCode, DeliveredAt: d.DeliveredAt, CreatedAt: d.CreatedAt, Payload: d.Payload}
}

// RegisterAdmin mounts the staff view of one tenant's deliveries behind staff.
func (h *Handler) RegisterAdmin(mux *http.ServeMux, staff func(http.Handler) http.Handler) {
	mux.Handle("GET /admin/v1/webhook-deliveries", staff(http.HandlerFunc(h.adminList)))
	mux.Handle("GET /admin/v1/stats/webhooks", staff(httpx.StatsHandler(h.svc.Stats)))
}

func (h *Handler) adminList(w http.ResponseWriter, r *http.Request) {
	tenantID := httpx.TrimmedQuery(r, "tenant_id")
	if tenantID == "" {
		httpx.WriteError(w, r, errs.Invalid("tenant_id_required", "tenant_id is required"))
		return
	}
	h.listFor(w, r, tenantID)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	h.listFor(w, r, auth.MustTenant(r.Context()))
}

func (h *Handler) listFor(w http.ResponseWriter, r *http.Request, tenantID string) {
	limit := httpx.QueryLimit(r, 25, 100)
	status := httpx.TrimmedQuery(r, "status")
	switch domain.Status(status) {
	case "", domain.StatusPending, domain.StatusDelivering, domain.StatusDelivered, domain.StatusFailed:
	default:
		httpx.WriteError(w, r, errs.Invalid("invalid_status", "unknown status filter"))
		return
	}
	ds, err := h.svc.List(r.Context(), app.ListFilter{
		TenantID: tenantID, PaymentID: httpx.TrimmedQuery(r, "payment_id"),
		Status: status, Before: httpx.TrimmedQuery(r, "cursor"), Limit: limit + 1,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := struct {
		Data       []deliveryView `json:"data"`
		HasMore    bool           `json:"has_more"`
		NextCursor string         `json:"next_cursor,omitempty"`
	}{Data: []deliveryView{}}
	if len(ds) > limit {
		ds, out.HasMore = ds[:limit], true
		out.NextCursor = ds[limit-1].ID
	}
	for _, d := range ds {
		out.Data = append(out.Data, toView(d))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) redeliver(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.Redeliver(r.Context(), auth.MustTenant(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, toView(d))
}
