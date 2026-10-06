package frappe

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/telemetry"
	"frappe-mcp-server/internal/types"
)

// Every call this server makes to Frappe names the answer that caused it, the CSRF fetch included: that one
// builds its own request and does not go through doRequest.
func TestEveryFrappeCallCarriesTheRequestID(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen[r.URL.Path] = r.Header.Get("X-Frappe-Request-Id")
		mu.Unlock()
		if r.URL.Path == "/app" {
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

	ctx := telemetry.WithRequestID(sidContext("alice-sid"), "3f2b1c4d5e6f7a8b9c0d1e2f3a4b5c6d")
	if _, err := client.UpdateDocument(ctx, types.UpdateDocumentRequest{
		DocType: "ToDo", Name: "BOB-001", Data: types.Document{"description": "done"},
	}); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) < 2 {
		t.Fatalf("want the desk page and the resource call, got %v", seen)
	}
	for path, id := range seen {
		if id != "3f2b1c4d5e6f7a8b9c0d1e2f3a4b5c6d" {
			t.Errorf("%s carried request id %q", path, id)
		}
	}
}
