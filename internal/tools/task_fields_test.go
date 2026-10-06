package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/mcp"
)

// ERPNext's Task carries exp_start_date and exp_end_date; the expected_* pair belongs to Project. Asking Task for
// the Project names makes Frappe answer "Unknown column", so the tool returns an error instead of a timeline.
func TestAnalyzeProjectTimelineAsksTaskForItsOwnDateFields(t *testing.T) {
	var taskQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/api/resource/Task") {
			taskQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"name":"P-1","expected_start_date":"2026-01-01"}}`))
	}))
	t.Cleanup(server.Close)

	client, err := frappe.NewClient(config.ERPNextConfig{
		BaseURL: server.URL, APIKey: "k", APISecret: "s", Timeout: 5 * time.Second,
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 10, Burst: 20},
		Retry:     config.RetryConfig{MaxAttempts: 1, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = NewRegistry(client).AnalyzeProjectTimeline(context.Background(), mcp.ToolRequest{
		ID: "t", Tool: "analyze_project_timeline", Params: []byte(`{"project_name":"P-1"}`),
	})
	if err != nil {
		t.Fatalf("AnalyzeProjectTimeline: %v", err)
	}
	for _, field := range []string{"exp_start_date", "exp_end_date"} {
		if !strings.Contains(taskQuery, field) {
			t.Errorf("the Task query does not ask for %s: %s", field, taskQuery)
		}
	}
	if strings.Contains(taskQuery, "expected_start_date") || strings.Contains(taskQuery, "expected_end_date") {
		t.Errorf("the Task query asks for a Project field, which Frappe answers with Unknown column: %s", taskQuery)
	}
}
