package auth

import (
	"encoding/json"
	"frappe-mcp-server/internal/auth/strategies"
	"net/http"
)

// Middleware provides authentication middleware for HTTP handlers
type Middleware struct {
	strategy    *strategies.OAuth2Strategy
	requireAuth bool
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(strategy *strategies.OAuth2Strategy, requireAuth bool) *Middleware {
	return &Middleware{
		strategy:    strategy,
		requireAuth: requireAuth,
	}
}

// Handler wraps an HTTP handler with authentication
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "Unauthorized",
				"message": "Valid authentication required",
			})
			return
		}

		// Add user to context
		r = r.WithContext(WithUser(r.Context(), user))
		next.ServeHTTP(w, r)
	})
}

