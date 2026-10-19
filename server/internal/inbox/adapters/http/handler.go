// Package http exposes the dashboard's notifications (user sessions only).
package http

import (
	"net/http"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/inbox/app"
	"github.com/eslieh/paytrack-go/server/internal/inbox/domain"
	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
)

type Handler struct{ svc *app.Service }

func New(svc *app.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux, authed func(http.Handler) http.Handler) {
	user := func(f http.HandlerFunc) http.Handler { return authed(auth.RequireUser(f)) }
	mux.Handle("GET /v1/notifications", user(h.list))
	mux.Handle("GET /v1/notifications/unread-count", user(h.unread))
	mux.Handle("POST /v1/notifications/{id}/read", user(h.read))
	mux.Handle("POST /v1/notifications/read-all", user(h.readAll))
}

type view struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	Title     string     `json:"title"`
	Body      string     `json:"body,omitempty"`
	Link      string     `json:"link,omitempty"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func toView(n *domain.Notification) view {
	return view{n.ID, n.Kind, n.Title, n.Body, n.Link, n.ReadAt, n.CreatedAt}
}

func principal(r *http.Request) auth.Principal {
	p, _ := auth.FromContext(r.Context())
	return p
}

// list pages by created_at: ?before=<RFC 3339 time of the last item>.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	var before time.Time
	if s := httpx.TrimmedQuery(r, "before"); s != "" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			httpx.WriteError(w, r, errs.Invalid("invalid_before", "before must be an RFC 3339 time"))
			return
		}
		before = t
	}
	limit := httpx.QueryLimit(r, 20, 100)
	ns, err := h.svc.List(r.Context(), principal(r), before, limit+1)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := struct {
		Data    []view `json:"data"`
		HasMore bool   `json:"has_more"`
	}{Data: []view{}}
	if len(ns) > limit {
		ns, out.HasMore = ns[:limit], true
	}
	for _, n := range ns {
		out.Data = append(out.Data, toView(n))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) unread(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.UnreadCount(r.Context(), principal(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int{"unread": n})
}

func (h *Handler) read(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.MarkRead(r.Context(), principal(r), r.PathValue("id")); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) readAll(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.MarkAllRead(r.Context(), principal(r)); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
