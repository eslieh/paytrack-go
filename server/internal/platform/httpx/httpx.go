// Package httpx holds shared HTTP plumbing for inbound adapters: JSON
// encoding, error mapping, middleware and graceful serving.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/ids"
)

const maxBody = 1 << 20

type ErrorBody struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id,omitempty"`
	} `json:"error"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// WriteError maps err to a status code and the standard error envelope.
// Errors that are not *errs.Error are logged and reported as 500 without
// leaking details.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, msg := http.StatusInternalServerError, "internal_error", "internal server error"
	if e, ok := errs.As(err); ok && e.Kind != errs.KindInternal {
		code, msg = e.Code, e.Message
		switch e.Kind {
		case errs.KindInvalid:
			status = http.StatusUnprocessableEntity
		case errs.KindNotFound:
			status = http.StatusNotFound
		case errs.KindConflict:
			status = http.StatusConflict
		case errs.KindUnauthorized:
			status = http.StatusUnauthorized
		case errs.KindForbidden:
			status = http.StatusForbidden
		case errs.KindRateLimited:
			status = http.StatusTooManyRequests
		case errs.KindUnavailable:
			status = http.StatusServiceUnavailable
		}
	} else {
		slog.ErrorContext(r.Context(), "request failed", "err", err, "path", r.URL.Path, "request_id", RequestID(r.Context()))
	}
	var body ErrorBody
	body.Error.Code, body.Error.Message, body.Error.RequestID = code, msg, RequestID(r.Context())
	JSON(w, status, body)
}

// Decode reads a JSON body into v, rejecting oversize or malformed input.
func Decode(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errs.Invalid("invalid_json", fmt.Sprintf("invalid request body: %v", err))
	}
	return nil
}

type reqIDKey struct{}

func RequestID(ctx context.Context) string {
	s, _ := ctx.Value(reqIDKey{}).(string)
	return s
}

// Middleware chain: request ID, access log, panic recovery.
func Common(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" || len(id) > 64 {
			id = ids.New("req")
		}
		w.Header().Set("X-Request-Id", id)
		r = r.WithContext(context.WithValue(r.Context(), reqIDKey{}, id))
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		start := time.Now()
		defer func() {
			if v := recover(); v != nil {
				slog.ErrorContext(r.Context(), "panic", "panic", v, "stack", string(debug.Stack()), "request_id", id)
				WriteError(rec, r, errors.New("panic"))
			}
			slog.InfoContext(r.Context(), "http", "method", r.Method, "path", r.URL.Path, "status", rec.status,
				"dur_ms", time.Since(start).Milliseconds(), "request_id", id)
		}()
		next.ServeHTTP(rec, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// CORS allows the separately hosted dashboard to call the API. An origin of
// "*" allows any origin (dev only).
func CORS(origins []string) func(http.Handler) http.Handler {
	allowAll := slices.Contains(origins, "*")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (allowAll || slices.Contains(origins, origin)) {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-Id")
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Expose-Headers", "X-Request-Id")
				h.Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Chain applies middleware so the first listed is outermost.
func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// Serve runs srv until ctx is cancelled, then drains in-flight requests.
func Serve(ctx context.Context, name, addr string, h http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		slog.Info("http listening", "server", name, "addr", addr)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return fmt.Errorf("%s server: %w", name, err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		slog.Info("http draining", "server", name)
		return srv.Shutdown(shutdownCtx)
	}
}

// QueryLimit parses ?limit= with a default and cap.
func QueryLimit(r *http.Request, def, max int) int {
	var n int
	if _, err := fmt.Sscan(r.URL.Query().Get("limit"), &n); err != nil || n <= 0 {
		return def
	}
	return min(n, max)
}

// TrimmedQuery returns a trimmed query parameter.
func TrimmedQuery(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

// ClientIP returns the caller's IP. With trustXFF (behind our own proxy) it
// uses the first X-Forwarded-For entry; it is only used for rate limiting and
// audit display, never for authorization.
func ClientIP(r *http.Request, trustXFF bool) string {
	if trustXFF {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			return strings.TrimSpace(first)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// StatsWindow parses ?days= (default 30, 1-365) for staff stats endpoints
// and returns the window start: midnight Nairobi time, days-1 days ago, so
// daily buckets are whole days.
func StatsWindow(r *http.Request, now time.Time) (days int, since time.Time) {
	days = 30
	if _, err := fmt.Sscan(r.URL.Query().Get("days"), &days); err != nil || days < 1 {
		days = 30
	}
	days = min(days, 365)
	local := now.In(Nairobi)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, Nairobi)
	return days, midnight.AddDate(0, 0, -(days - 1))
}

// Nairobi is the platform's reporting time zone (EAT, no DST).
var Nairobi = time.FixedZone("EAT", 3*60*60)

// StatsHandler serves a staff stats endpoint over the ?days= window as
// {"days", "since", "stats"}.
func StatsHandler[T any](stats func(ctx context.Context, since time.Time) (T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		days, since := StatsWindow(r, time.Now())
		v, err := stats(r.Context(), since)
		if err != nil {
			WriteError(w, r, err)
			return
		}
		JSON(w, http.StatusOK, map[string]any{"days": days, "since": since, "stats": v})
	}
}
