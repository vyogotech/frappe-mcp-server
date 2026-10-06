package strategies

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// frappeCounter is a Frappe that answers get_logged_user and the desk page, counting both.
func frappeCounter(t *testing.T) (url string, getLoggedUser, desk *atomic.Int64) {
	t.Helper()
	getLoggedUser, desk = &atomic.Int64{}, &atomic.Int64{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "frappe.auth.get_logged_user"):
			getLoggedUser.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"alice@example.com"}`))
		case r.URL.Path == "/app":
			desk.Add(1)
			_, _ = w.Write([]byte(`<script>csrf_token = "` + strings.Repeat("a", 40) + `";</script>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, getLoggedUser, desk
}

func authenticateWithSid(t *testing.T, s *OAuth2Strategy, sid string) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Header.Set("Cookie", "sid="+sid)
	if _, err := s.Authenticate(context.Background(), r); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
}

// The sid cache must expire on the configured token_cache.ttl, not on a TTL of its own.
func TestSidCacheHonoursTheConfiguredTTL(t *testing.T) {
	url, getLoggedUser, _ := frappeCounter(t)
	s := NewOAuth2Strategy(OAuth2StrategyConfig{BaseURL: url, ValidateRemote: true, CacheTTL: 30 * time.Millisecond})

	authenticateWithSid(t, s, "sid-ttl")
	authenticateWithSid(t, s, "sid-ttl")
	if n := getLoggedUser.Load(); n != 1 {
		t.Fatalf("within the TTL the session should be validated once, got %d", n)
	}

	time.Sleep(60 * time.Millisecond)
	authenticateWithSid(t, s, "sid-ttl")
	if n := getLoggedUser.Load(); n != 2 {
		t.Errorf("after the configured 30ms TTL the session should be validated again, got %d validations", n)
	}
}

// Authenticating costs one call to Frappe: the desk page is only needed before a write.
func TestAuthenticateDoesNotRenderTheDesk(t *testing.T) {
	url, getLoggedUser, desk := frappeCounter(t)
	s := NewOAuth2Strategy(OAuth2StrategyConfig{BaseURL: url, ValidateRemote: true, CacheTTL: time.Minute})

	authenticateWithSid(t, s, "sid-read")

	if n := getLoggedUser.Load(); n != 1 {
		t.Errorf("get_logged_user calls = %d, want 1", n)
	}
	if n := desk.Load(); n != 0 {
		t.Errorf("desk renders during authentication = %d, want 0", n)
	}
}
