// Package errs defines the small set of error kinds that cross the
// hexagon boundary, so inbound adapters (HTTP) can map domain/app errors to
// responses without knowing about each module's internals.
package errs

import (
	"errors"
	"fmt"
)

type Kind int

const (
	KindInternal Kind = iota
	KindInvalid
	KindNotFound
	KindConflict
	KindUnauthorized
	KindForbidden
	KindRateLimited
	KindUnavailable // an upstream (e.g. Daraja) failed; retry later
)

type Error struct {
	Kind    Kind
	Code    string // stable machine-readable code, e.g. "channel_not_found"
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func E(kind Kind, code, msg string) *Error { return &Error{Kind: kind, Code: code, Message: msg} }

func Invalid(code, msg string) *Error      { return E(KindInvalid, code, msg) }
func NotFound(code, msg string) *Error     { return E(KindNotFound, code, msg) }
func Conflict(code, msg string) *Error     { return E(KindConflict, code, msg) }
func Unauthorized(code, msg string) *Error { return E(KindUnauthorized, code, msg) }
func Forbidden(code, msg string) *Error    { return E(KindForbidden, code, msg) }
func RateLimited(msg string) *Error        { return E(KindRateLimited, "rate_limited", msg) }
func Unavailable(code, msg string) *Error  { return E(KindUnavailable, code, msg) }

// As extracts an *Error from err's chain.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// IsKind reports whether err carries the given kind.
func IsKind(err error, k Kind) bool {
	e, ok := As(err)
	return ok && e.Kind == k
}
