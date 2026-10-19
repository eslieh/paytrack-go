// Package http exposes the mail module's staff-only stats.
package http

import (
	"net/http"

	"github.com/eslieh/paytrack-go/server/internal/mail/app"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
)

// RegisterAdmin mounts email delivery stats behind staff.
func RegisterAdmin(mux *http.ServeMux, staff func(http.Handler) http.Handler, svc *app.Service) {
	mux.Handle("GET /admin/v1/stats/emails", staff(httpx.StatsHandler(svc.Stats)))
}
