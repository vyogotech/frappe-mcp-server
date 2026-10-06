package frappe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"frappe-mcp-server/internal/auth"
	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/types"

	"github.com/stretchr/testify/require"
)

// Frappe grants the document to alice's session only; nothing the MCP server keeps may hand it to bob.
func TestADocumentIsNeverServedToAUserFrappeRefuses(t *testing.T) {
	frappe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("sid"); err == nil && c.Value == "alice-sid" {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"name": "SECRET-1", "owner": "alice"}})
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"exc_type": "PermissionError"})
	}))
	defer frappe.Close()
	client, err := NewClient(config.ERPNextConfig{BaseURL: frappe.URL, Timeout: 5 * time.Second,
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 100, Burst: 100}, Retry: config.RetryConfig{MaxAttempts: 1}})
	require.NoError(t, err)
	as := func(sid string) context.Context {
		return auth.WithUser(context.Background(), &types.User{Email: sid, SessionID: sid})
	}

	doc, err := client.GetDocument(as("alice-sid"), "File", "SECRET-1")
	require.NoError(t, err)
	require.Equal(t, "SECRET-1", doc["name"])

	doc, err = client.GetDocument(as("bob-sid"), "File", "SECRET-1")
	require.Error(t, err, "bob received alice's document: %v", doc)
	require.Nil(t, doc)
}
