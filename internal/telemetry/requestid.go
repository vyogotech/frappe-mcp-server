package telemetry

import (
	"context"
	"net/http"
	"regexp"

	"github.com/google/uuid"
)

// requestIDHeader is the name the agent sends. Frappe parses only FrappeRequestIDHeader, so an outbound
// Frappe call carries the same value under that name instead (ADR-008).
const requestIDHeader = "X-Request-ID"

// FrappeRequestIDHeader is the one correlation header frappe.monitor adopts (frappe/monitor.py:79).
const FrappeRequestIDHeader = "X-Frappe-Request-Id"

// requestIDShape is ADR-008's. An inbound id is the caller's word: unvalidated it would put an unbounded
// caller-chosen string on every log line of the request.
var requestIDShape = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type requestIDKey struct{}

// RequestIDFrom returns the caller's id when it has the agreed shape, and a fresh uuid4 when it does not,
// so every request is identified whether or not the caller identified it.
func RequestIDFrom(r *http.Request) string {
	if id := r.Header.Get(requestIDHeader); requestIDShape.MatchString(id) {
		return id
	}
	return uuid.NewString()
}

// WithRequestID carries id down the request's context, where the tool audit line and the Frappe client read it.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext is "" off the HTTP path — the stdio binary, and any call made outside a request.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
