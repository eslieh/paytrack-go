// Package http exposes payment endpoints to tenant backends and the
// dashboard.
package http

import (
	"net/http"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/payment/app"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
)

type Handler struct{ svc *app.Service }

func New(svc *app.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	mux.Handle("POST /v1/payments", authed(auth.RequireVerified(http.HandlerFunc(h.create))))
	mux.Handle("GET /v1/payments", authed(http.HandlerFunc(h.list)))
	mux.Handle("GET /v1/payments/{id}", authed(http.HandlerFunc(h.get)))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelID        string            `json:"channel_id"`
		Amount           int64             `json:"amount"`
		Phone            string            `json:"phone"`
		AccountReference string            `json:"account_reference"`
		Description      string            `json:"description"`
		Metadata         map[string]string `json:"metadata"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, replayed, err := h.svc.Initiate(r.Context(), app.InitiateCmd{
		TenantID: auth.MustTenant(r.Context()), IdempotencyKey: r.Header.Get("Idempotency-Key"),
		ChannelID: req.ChannelID, Amount: req.Amount, Phone: req.Phone,
		AccountReference: req.AccountReference, Description: req.Description, Metadata: req.Metadata,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		w.Header().Set("Idempotent-Replayed", "true")
		status = http.StatusOK
	}
	httpx.JSON(w, status, app.View(p))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Get(r.Context(), auth.MustTenant(r.Context()), r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, app.View(p))
}

type page struct {
	Data       []contracts.PaymentView `json:"data"`
	HasMore    bool                    `json:"has_more"`
	NextCursor string                  `json:"next_cursor,omitempty"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit := httpx.QueryLimit(r, 25, 100)
	ps, err := h.svc.List(r.Context(), app.ListFilter{
		TenantID: auth.MustTenant(r.Context()), Status: httpx.TrimmedQuery(r, "status"),
		ChannelID: httpx.TrimmedQuery(r, "channel_id"), Phone: httpx.TrimmedQuery(r, "phone"),
		Before: httpx.TrimmedQuery(r, "cursor"), Limit: limit + 1,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := page{Data: []contracts.PaymentView{}}
	if len(ps) > limit {
		ps, out.HasMore = ps[:limit], true
		out.NextCursor = ps[limit-1].ID
	}
	for _, p := range ps {
		out.Data = append(out.Data, app.View(p))
	}
	httpx.JSON(w, http.StatusOK, out)
}
