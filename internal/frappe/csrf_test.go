package frappe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"frappe-mcp-server/internal/auth"
	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/types"
)

// csrfFrappe answers any resource call and the desk page, counting the desk renders.
func csrfFrappe(t *testing.T) (*Client, *atomic.Int64) {
	t.Helper()
	desk := &atomic.Int64{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app" {
			desk.Add(1)
			_, _ = w.Write([]byte(`<script>csrf_token = "` + strings.Repeat("b", 40) + `";</script>`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"name": "BOB-001"}}`))
	}))
	t.Cleanup(srv.Close)

	client, err := NewClient(config.ERPNextConfig{
		BaseURL:   srv.URL,
		Timeout:   5 * time.Second,
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 100, Burst: 100},
		Retry:     config.RetryConfig{MaxAttempts: 1, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	return client, desk
}

func sidContext(sid string) context.Context {
	return auth.WithUser(context.Background(), &types.User{Email: "alice@example.com", SessionID: sid})
}

// A read needs no CSRF token, so it must not pay for a desk render.
func TestReadsDoNotRenderTheDesk(t *testing.T) {
	client, desk := csrfFrappe(t)

	if _, err := client.GetDocument(sidContext("sid-read"), "Project", "PROJ-0001"); err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if n := desk.Load(); n != 0 {
		t.Errorf("desk renders on a read = %d, want 0", n)
	}
}

// A write needs the session's token, and one desk render serves every later write for that sid.
func TestWritesFetchTheCSRFTokenOncePerSession(t *testing.T) {
	client, desk := csrfFrappe(t)
	ctx := sidContext("sid-write")

	for range 2 {
		if _, err := client.CreateDocument(ctx, types.CreateDocumentRequest{
			DocType: "User",
			Data:    types.Document{"email": "bob@example.com"},
		}); err != nil {
			t.Fatalf("CreateDocument: %v", err)
		}
	}
	if n := desk.Load(); n != 1 {
		t.Errorf("desk renders for two writes on one sid = %d, want 1", n)
	}
}
