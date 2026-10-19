package http

import (
	"net/http"

	"github.com/eslieh/paytrack-go/server/internal/contracts"
	"github.com/eslieh/paytrack-go/server/internal/payment/app"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
)

// RegisterAdmin mounts the staff view of one tenant's payments behind staff.
func (h *Handler) RegisterAdmin(mux *http.ServeMux, staff func(http.Handler) http.Handler) {
	mux.Handle("GET /admin/v1/payments", staff(http.HandlerFunc(h.adminList)))
	mux.Handle("GET /admin/v1/stats/payments", staff(httpx.StatsHandler(h.svc.Stats)))
}

func (h *Handler) adminList(w http.ResponseWriter, r *http.Request) {
	tenantID := httpx.TrimmedQuery(r, "tenant_id")
	if tenantID == "" {
		httpx.WriteError(w, r, errs.Invalid("tenant_id_required", "tenant_id is required"))
		return
	}
	limit := httpx.QueryLimit(r, 25, 100)
	ps, err := h.svc.List(r.Context(), app.ListFilter{
		TenantID: tenantID, Status: httpx.TrimmedQuery(r, "status"), Before: httpx.TrimmedQuery(r, "cursor"), Limit: limit + 1,
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
