// Package http exposes support tickets: /v1/support for dashboard users
// (never API keys) and /admin/v1/support for staff.
package http

import (
	"net/http"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/platform/auth"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
	"github.com/eslieh/paytrack-go/server/internal/support/app"
	"github.com/eslieh/paytrack-go/server/internal/support/domain"
)

type Handler struct{ svc *app.Service }

func New(svc *app.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the tenant routes behind authed and the staff routes
// behind staff (the admin module's session check).
func (h *Handler) Register(mux *http.ServeMux, authed, staff func(http.Handler) http.Handler) {
	user := func(f http.HandlerFunc) http.Handler { return authed(auth.RequireUser(f)) }
	mux.Handle("GET /v1/support/tickets", user(h.list))
	mux.Handle("POST /v1/support/tickets", user(h.create))
	mux.Handle("GET /v1/support/tickets/{id}", user(h.get))
	mux.Handle("POST /v1/support/tickets/{id}/messages", user(h.reply))
	mux.Handle("POST /v1/support/tickets/{id}/rating", user(h.rate))
	mux.Handle("POST /v1/support/feedback", user(h.feedback))

	st := func(f http.HandlerFunc) http.Handler { return staff(f) }
	mux.Handle("GET /admin/v1/support/summary", st(h.staffSummary))
	mux.Handle("GET /admin/v1/support/tickets", st(h.staffList))
	mux.Handle("GET /admin/v1/support/tickets/{id}", st(h.staffGet))
	mux.Handle("POST /admin/v1/support/tickets/{id}/messages", st(h.staffReply))
	mux.Handle("PATCH /admin/v1/support/tickets/{id}", st(h.staffUpdate))
	mux.Handle("GET /admin/v1/support/feedback", st(h.staffFeedback))
}

// ---------------------------------------------------------------- views

type ticketView struct {
	ID              string     `json:"id"`
	Number          int64      `json:"number"`
	Subject         string     `json:"subject"`
	Category        string     `json:"category"`
	Priority        string     `json:"priority"`
	Status          string     `json:"status"`
	LastMessageAt   time.Time  `json:"last_message_at"`
	FirstResponseAt *time.Time `json:"first_response_at"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	Rating          *int       `json:"rating"`
	RatingComment   string     `json:"rating_comment,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	// Staff views only.
	TenantID       string `json:"tenant_id,omitempty"`
	TenantName     string `json:"tenant_name,omitempty"`
	UserID         string `json:"user_id,omitempty"`
	RequesterEmail string `json:"requester_email,omitempty"`
}

func toTicket(t *domain.Ticket, staff bool) ticketView {
	v := ticketView{ID: t.ID, Number: t.Number, Subject: t.Subject, Category: string(t.Category), Priority: string(t.Priority),
		Status: string(t.Status), LastMessageAt: t.LastMessageAt, FirstResponseAt: t.FirstResponseAt, ResolvedAt: t.ResolvedAt,
		Rating: t.Rating, RatingComment: t.RatingComment, CreatedAt: t.CreatedAt}
	if staff {
		v.TenantID, v.TenantName, v.UserID, v.RequesterEmail = t.TenantID, t.TenantName, t.UserID, t.RequesterEmail
	}
	return v
}

type messageView struct {
	ID         string    `json:"id"`
	AuthorKind string    `json:"author_kind"`
	AuthorName string    `json:"author_name"`
	Body       string    `json:"body"`
	Internal   bool      `json:"internal,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func toMessage(m *domain.Message) messageView {
	return messageView{ID: m.ID, AuthorKind: string(m.AuthorKind), AuthorName: m.AuthorName, Body: m.Body,
		Internal: m.Internal, CreatedAt: m.CreatedAt}
}

func thread(t *domain.Ticket, msgs []*domain.Message, staff bool) map[string]any {
	out := make([]messageView, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toMessage(m))
	}
	return map[string]any{"ticket": toTicket(t, staff), "messages": out}
}

// page writes a cursor page of tickets; ts holds up to limit+1 rows.
func page(w http.ResponseWriter, ts []*domain.Ticket, limit int, staff bool) {
	out := struct {
		Data       []ticketView `json:"data"`
		HasMore    bool         `json:"has_more"`
		NextCursor string       `json:"next_cursor,omitempty"`
	}{Data: []ticketView{}}
	if len(ts) > limit {
		ts, out.HasMore = ts[:limit], true
		out.NextCursor = ts[limit-1].ID
	}
	for _, t := range ts {
		out.Data = append(out.Data, toTicket(t, staff))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func decode[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := httpx.Decode(r, &v); err != nil {
		httpx.WriteError(w, r, err)
		return v, false
	}
	return v, true
}

func principal(r *http.Request) auth.Principal {
	p, _ := auth.FromContext(r.Context())
	return p
}

// ---------------------------------------------------------------- tenant users

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit := httpx.QueryLimit(r, 25, 100)
	ts, err := h.svc.ListTickets(r.Context(), principal(r), httpx.TrimmedQuery(r, "cursor"), limit+1)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	page(w, ts, limit, false)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Subject  string `json:"subject"`
		Category string `json:"category"`
		Body     string `json:"body"`
	}](w, r)
	if !ok {
		return
	}
	t, err := h.svc.CreateTicket(r.Context(), principal(r), domain.NewTicket{Subject: req.Subject, Category: req.Category, Body: req.Body})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toTicket(t, false))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	t, msgs, err := h.svc.GetTicket(r.Context(), principal(r), r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, thread(t, msgs, false))
}

func (h *Handler) reply(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Body string `json:"body"`
	}](w, r)
	if !ok {
		return
	}
	m, err := h.svc.Reply(r.Context(), principal(r), r.PathValue("id"), req.Body)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toMessage(m))
}

func (h *Handler) rate(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}](w, r)
	if !ok {
		return
	}
	t, err := h.svc.Rate(r.Context(), principal(r), r.PathValue("id"), req.Rating, req.Comment)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toTicket(t, false))
}

func (h *Handler) feedback(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Rating  *int   `json:"rating"`
		Message string `json:"message"`
		Page    string `json:"page"`
	}](w, r)
	if !ok {
		return
	}
	if err := h.svc.SendFeedback(r.Context(), principal(r), req.Rating, req.Message, req.Page); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------- staff

func (h *Handler) staffSummary(w http.ResponseWriter, r *http.Request) {
	counts, err := h.svc.Summary(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := map[string]int{}
	for _, s := range []domain.Status{domain.StatusAwaitingStaff, domain.StatusAwaitingUser, domain.StatusResolved, domain.StatusClosed} {
		out[string(s)] = counts[s]
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"counts": out})
}

func (h *Handler) staffList(w http.ResponseWriter, r *http.Request) {
	limit := httpx.QueryLimit(r, 50, 200)
	ts, err := h.svc.StaffListTickets(r.Context(), app.TicketFilter{
		TenantID: httpx.TrimmedQuery(r, "tenant_id"), Status: domain.Status(httpx.TrimmedQuery(r, "status")),
		Before: httpx.TrimmedQuery(r, "cursor"), Limit: limit + 1,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	page(w, ts, limit, true)
}

func (h *Handler) staffGet(w http.ResponseWriter, r *http.Request) {
	t, msgs, err := h.svc.StaffGetTicket(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, thread(t, msgs, true))
}

func (h *Handler) staffReply(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Body     string `json:"body"`
		Internal bool   `json:"internal"`
	}](w, r)
	if !ok {
		return
	}
	m, err := h.svc.StaffReply(r.Context(), principal(r), r.PathValue("id"), req.Body, req.Internal)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toMessage(m))
}

func (h *Handler) staffUpdate(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[struct {
		Status   *domain.Status   `json:"status"`
		Priority *domain.Priority `json:"priority"`
	}](w, r)
	if !ok {
		return
	}
	t, err := h.svc.StaffUpdate(r.Context(), r.PathValue("id"), app.TicketUpdate{Status: req.Status, Priority: req.Priority})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toTicket(t, true))
}

func (h *Handler) staffFeedback(w http.ResponseWriter, r *http.Request) {
	limit := httpx.QueryLimit(r, 50, 200)
	fs, err := h.svc.ListFeedback(r.Context(), httpx.TrimmedQuery(r, "cursor"), limit+1)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	type view struct {
		ID        string    `json:"id"`
		TenantID  string    `json:"tenant_id"`
		UserEmail string    `json:"user_email"`
		Rating    *int      `json:"rating"`
		Message   string    `json:"message"`
		Page      string    `json:"page"`
		CreatedAt time.Time `json:"created_at"`
	}
	out := struct {
		Data       []view `json:"data"`
		HasMore    bool   `json:"has_more"`
		NextCursor string `json:"next_cursor,omitempty"`
	}{Data: []view{}}
	if len(fs) > limit {
		fs, out.HasMore = fs[:limit], true
		out.NextCursor = fs[limit-1].ID
	}
	for _, f := range fs {
		out.Data = append(out.Data, view{f.ID, f.TenantID, f.UserEmail, f.Rating, f.Message, f.Page, f.CreatedAt})
	}
	httpx.JSON(w, http.StatusOK, out)
}
