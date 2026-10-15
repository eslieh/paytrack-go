// Package http is the public ingress for Safaricom callbacks. It is kept
// deliberately small: authenticate the source, store, acknowledge.
package http

import (
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/eslieh/paytrack-go/server/internal/callback/app"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
)

type Handler struct {
	svc      *app.Service
	allowed  []*net.IPNet // empty = allow all
	trustXFF bool
}

func New(svc *app.Service, allowed []*net.IPNet, trustXFF bool) *Handler {
	return &Handler{svc: svc, allowed: allowed, trustXFF: trustXFF}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /callbacks/mpesa/stk/{token}", h.stk)
}

// ack is the body Daraja expects back.
var ack = map[string]any{"ResultCode": 0, "ResultDesc": "Accepted"}

func (h *Handler) stk(w http.ResponseWriter, r *http.Request) {
	ip := h.clientIP(r)
	if !h.isAllowed(ip) {
		http.NotFound(w, r)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	if err := h.svc.Ingest(r.Context(), r.PathValue("token"), raw, ip); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ack)
}

// clientIP uses the right-most X-Forwarded-For entry (the one appended by
// our own load balancer) when trustXFF is set; left-most entries are
// client-controlled.
func (h *Handler) clientIP(r *http.Request) string {
	if h.trustXFF {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *Handler) isAllowed(ip string) bool {
	if len(h.allowed) == 0 {
		return true
	}
	parsed := net.ParseIP(ip)
	for _, n := range h.allowed {
		if parsed != nil && n.Contains(parsed) {
			return true
		}
	}
	return false
}
