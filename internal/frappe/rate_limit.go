package frappe

import (
	"context"

	"github.com/patrickmn/go-cache"
	"golang.org/x/time/rate"

	"frappe-mcp-server/internal/auth"
)

// limiter returns the caller's own bucket, keyed by the identity the auth middleware put in the context. One bucket
// for the whole process lets a single user's loop spend everyone else's budget.
func (c *Client) limiter(ctx context.Context) *rate.Limiter {
	// no user means the configured API key, which is one identity of its own
	key := ""
	if user := auth.UserFromContext(ctx); user != nil && user.Email != "" {
		key = user.Email
	}

	c.limitersMu.Lock()
	defer c.limitersMu.Unlock()
	if cached, found := c.limiters.Get(key); found {
		if limiter, ok := cached.(*rate.Limiter); ok {
			return limiter
		}
	}
	limiter := rate.NewLimiter(rate.Limit(c.rateLimit.RequestsPerSecond), c.rateLimit.Burst)
	c.limiters.Set(key, limiter, cache.DefaultExpiration)
	return limiter
}
