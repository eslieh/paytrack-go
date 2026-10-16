// Package auth carries the authenticated principal through request
// contexts. Every tenant-scoped use case takes its tenant ID from here, never
// from request input.
package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/eslieh/paytrack-go/server/internal/platform/errs"
	"github.com/eslieh/paytrack-go/server/internal/platform/httpx"
)

type Kind string

const (
	KindUser   Kind = "user"    // dashboard session (JWT)
	KindAPIKey Kind = "api_key" // tenant backend
)

type Principal struct {
	Kind          Kind
	TenantID      string
	UserID        string // set for KindUser
	SessionID     string // set for KindUser
	EmailVerified bool   // set for KindUser
	APIKeyID      string // set for KindAPIKey
}

// AuthenticatorFunc adapts a function to Authenticator.
type AuthenticatorFunc func(ctx context.Context, bearer string) (Principal, error)

func (f AuthenticatorFunc) Authenticate(ctx context.Context, bearer string) (Principal, error) {
	return f(ctx, bearer)
}

type Authenticator interface {
	Authenticate(ctx context.Context, bearer string) (Principal, error)
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

// Middleware resolves the bearer token to a Principal. Handlers wrapped by
// it can rely on FromContext succeeding.
func Middleware(a Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(h, "Bearer ")
			if !ok || token == "" {
				httpx.WriteError(w, r, errs.Unauthorized("missing_token", "Authorization: Bearer <token> required"))
				return
			}
			p, err := a.Authenticate(r.Context(), token)
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
		})
	}
}

// RequireUser restricts a handler to dashboard sessions (e.g. managing API
// keys must not be possible with an API key).
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, _ := FromContext(r.Context()); p.Kind != KindUser {
			httpx.WriteError(w, r, errs.Forbidden("user_session_required", "this endpoint requires a dashboard session"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireVerified blocks dashboard sessions whose email is not verified.
// API keys pass: only verified users can create them.
func RequireVerified(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, _ := FromContext(r.Context()); p.Kind == KindUser && !p.EmailVerified {
			httpx.WriteError(w, r, errs.Forbidden("email_not_verified", "verify your email address to do this"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// MustTenant returns the tenant ID of the authenticated principal.
func MustTenant(ctx context.Context) string {
	p, ok := FromContext(ctx)
	if !ok {
		panic("auth: no principal in context; route not wrapped by Middleware")
	}
	return p.TenantID
}
