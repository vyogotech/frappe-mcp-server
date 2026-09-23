package auth

import (
	"encoding/json"
	"errors"
	"frappe-mcp-server/internal/auth/strategies"
	"log/slog"
	"net/http"
)

type Middleware struct {
	strategy    *strategies.OAuth2Strategy
	requireAuth bool
}

func NewMiddleware(strategy *strategies.OAuth2Strategy, requireAuth bool) *Middleware {
	return &Middleware{
		strategy:    strategy,
		requireAuth: requireAuth,
	}
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to authenticate
		user, err := m.strategy.Authenticate(r.Context(), r)

		if !m.requireAuth {
			// Optional auth - continue even if auth fails
			if user != nil {
				r = r.WithContext(WithUser(r.Context(), user))
			}
			next.ServeHTTP(w, r)
			return
		}

		// Required auth - fail if no valid auth
		if err != nil {
			status, message := http.StatusUnauthorized, "Valid authentication required"
			if errors.Is(err, strategies.ErrFrappeUnavailable) {
				// the session was never judged: 401 would have the caller tell the user it was rejected
				status, message = http.StatusServiceUnavailable, "Frappe did not answer"
			}
			// a refused request is a security event: record why, never the credential.
			// #nosec G706 -- main.go installs slog.NewJSONHandler, which JSON-encodes every value, so a
			// newline in a path or a remote address cannot forge a second log entry (CWE-117).
			slog.Warn("authentication failed", "reason", err.Error(), "status", status, "path", r.URL.Path, "remote_addr", r.RemoteAddr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   http.StatusText(status),
				"message": message,
			})
			return
		}

		// Add user to context
		r = r.WithContext(WithUser(r.Context(), user))
		next.ServeHTTP(w, r)
	})
}
