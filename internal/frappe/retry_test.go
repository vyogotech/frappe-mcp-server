package frappe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/types"
)

// A write retried after a timeout can run twice (RFC 9110 9.2.2), and a 500 will not fix itself: only a GET is
// retried, and only on a gateway error.
func TestRetryOnlySafeRequestsOnGatewayErrors(t *testing.T) {
	cases := []struct {
		method string
		status int
		want   int32
	}{
		{"GET", http.StatusServiceUnavailable, 3},
		{"GET", http.StatusBadGateway, 3},
		{"GET", http.StatusGatewayTimeout, 3},
		{"GET", http.StatusInternalServerError, 1},
		{"GET", http.StatusNotFound, 1},
		{"POST", http.StatusServiceUnavailable, 1},
		{"PUT", http.StatusServiceUnavailable, 1},
		{"DELETE", http.StatusServiceUnavailable, 1},
	}
	for _, tc := range cases {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(tc.status)
		}))
		c, err := NewClient(config.ERPNextConfig{
			BaseURL: srv.URL, APIKey: "k", APISecret: "s", Timeout: time.Second,
			RateLimit: config.RateLimitConfig{RequestsPerSecond: 1000, Burst: 1000},
			Retry:     config.RetryConfig{MaxAttempts: 3, InitialDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond},
		})
		if err != nil {
			t.Fatal(err)
		}
		err = c.makeRequest(context.Background(), tc.method, "/api/resource/ToDo/x", nil, nil)
		srv.Close()
		var erpErr *types.ERPNextError
		if !errors.As(err, &erpErr) || erpErr.StatusCode != tc.status {
			t.Errorf("%s %d: the error lost its cause: %v", tc.method, tc.status, err)
		}
		if got := calls.Load(); got != tc.want {
			t.Errorf("%s %d: %d requests, want %d", tc.method, tc.status, got, tc.want)
		}
	}
}

func TestNoRetryConfigStillSendsTheRequest(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"data": {}}`))
	}))
	defer srv.Close()
	c, _ := NewClient(config.ERPNextConfig{BaseURL: srv.URL, APIKey: "k", APISecret: "s", Timeout: time.Second,
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 1000, Burst: 1000}})
	if err := c.makeRequest(context.Background(), "GET", "/api/resource/ToDo/x", nil, nil); err != nil || calls.Load() != 1 {
		t.Fatalf("with no retry settings: %d requests, error %v", calls.Load(), err)
	}
}
