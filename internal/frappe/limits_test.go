package frappe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
)

func limitedClient(t *testing.T, handler http.HandlerFunc, rps, burst int) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := NewClient(config.ERPNextConfig{
		BaseURL:   srv.URL,
		APIKey:    "k",
		APISecret: "s",
		Timeout:   5 * time.Second,
		RateLimit: config.RateLimitConfig{RequestsPerSecond: rps, Burst: burst},
		Retry:     config.RetryConfig{MaxAttempts: 1, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// A Frappe that answers with more bytes than the client should ever read must not be read whole.
func TestResponseBodyIsBounded(t *testing.T) {
	client := limitedClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"name":"` + strings.Repeat("x", maxResponseBody+1024) + `"}}`))
	}, 1000, 1000)

	_, err := client.GetDocument(context.Background(), "Project", "PROJ-0001")
	if err == nil {
		t.Fatal("an over-long Frappe response was accepted")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error does not say the response was too large: %v", err)
	}
}
