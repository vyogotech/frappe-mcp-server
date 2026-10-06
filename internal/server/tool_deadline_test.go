package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/mcp"
)

// The REST tool routes (/tool/ and /api/v1/tools/) call tools without the MCP closure, so they need the deadline too.
func TestARestToolCallEndsAtItsDeadline(t *testing.T) {
	previous := mcp.ToolDeadline
	mcp.ToolDeadline = 50 * time.Millisecond
	t.Cleanup(func() { mcp.ToolDeadline = previous })

	hang := make(chan struct{})
	frappeStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-hang }))
	t.Cleanup(func() { close(hang); frappeStub.Close() })
	erp := config.ERPNextConfig{BaseURL: frappeStub.URL, APIKey: "k", APISecret: "s",
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 100, Burst: 100}, Retry: config.RetryConfig{MaxAttempts: 1}}
	client, err := frappe.NewClient(erp)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewMCPServer(&config.Config{ERPNext: erp}, client)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		s.handleToolCall(w, httptest.NewRequest("POST", "/tool/get_document", strings.NewReader(`{"params":{"doctype":"ToDo","name":"x"}}`)))
		done <- w.Code
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the REST tool call ran past its deadline")
	}
}
