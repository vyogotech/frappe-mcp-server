package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/mcp"

	"github.com/stretchr/testify/require"
)

// A linked document the user may not read is reported as such, not left out as if the field were empty.
func TestARelatedDocumentThatFailsSaysSo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/resource/Note/N-1" {
			_, _ = w.Write([]byte(`{"data": {"name": "N-1", "customer": "C-1"}}`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"exc_type": "PermissionError"}`))
	}))
	t.Cleanup(srv.Close)
	client, err := frappe.NewClient(config.ERPNextConfig{BaseURL: srv.URL, APIKey: "k", APISecret: "s", Timeout: 5 * time.Second,
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 100, Burst: 100},
		Retry:     config.RetryConfig{MaxAttempts: 1, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond}})
	require.NoError(t, err)

	resp, err := NewRegistry(client).AnalyzeDocument(context.Background(), mcp.ToolRequest{
		Tool: "analyze_document", Params: []byte(`{"doctype": "Note", "name": "N-1", "include_related": true}`)})
	require.NoError(t, err)
	var out struct {
		Related map[string]map[string]interface{} `json:"related_documents"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp.Content[1].Text), &out))
	require.Contains(t, out.Related["customer"]["error"], "403")
}
