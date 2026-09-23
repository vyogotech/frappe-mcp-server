package frappe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/patrickmn/go-cache"

	"frappe-mcp-server/internal/types"
)

// csrfToken returns the session's CSRF token, which Frappe only publishes in the rendered desk page, so it is
// read once per session and only when a write is about to need it.
func (c *Client) csrfToken(ctx context.Context, user *types.User) string {
	if user.CSRFToken != "" {
		return user.CSRFToken
	}
	if cached, found := c.csrfTokens.Get(user.SessionID); found {
		if token, ok := cached.(string); ok {
			return token
		}
	}
	token, err := c.fetchCSRFToken(ctx, user.SessionID)
	if err != nil {
		// A write without the token fails at Frappe with a clear CSRFTokenError; a read never needs one, so
		// this must not turn into a failed request here.
		slog.Warn("Failed to read the session's CSRF token; this write will be refused", "user", user.Email, "error", err)
		return ""
	}
	c.csrfTokens.Set(user.SessionID, token, cache.DefaultExpiration)
	return token
}

func (c *Client) fetchCSRFToken(ctx context.Context, sid string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/app", nil)
	if err != nil {
		return "", fmt.Errorf("build desk request: %w", err)
	}
	// a request carries a cookie as name=value only: Secure, HttpOnly and SameSite belong to Set-Cookie
	req.Header.Set("Cookie", "sid="+sid)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("desk request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("desk returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read desk body: %w", err)
	}

	m := csrfTokenPattern.FindSubmatch(body)
	if m == nil {
		return "", errors.New("csrf_token not found in desk HTML")
	}
	return string(m[1]), nil
}
