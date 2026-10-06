package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/mcp"
)

// kbFrappe records what reaches rag.search.search, which owns the vector search and the permission filter.
type kbFrappe struct {
	hits []string
	// the question travels in the body since ADR-037, so this is what rag.search.search was sent
	sent map[string]any
}

func newKBFrappe(t *testing.T, passages []map[string]interface{}) (*frappe.Client, *kbFrappe) {
	t.Helper()
	rec := &kbFrappe{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.hits = append(rec.hits, r.URL.Path)
		rec.sent = map[string]any{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&rec.sent)
		}

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/method/rag.search.search" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": passages})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	client, err := frappe.NewClient(config.ERPNextConfig{
		BaseURL:   srv.URL,
		APIKey:    "test_key",
		APISecret: "test_secret",
		Timeout:   30 * time.Second,
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 100, Burst: 100},
		Retry:     config.RetryConfig{MaxAttempts: 1, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond},
	})
	require.NoError(t, err)
	return client, rec
}

func searchKB(t *testing.T, reg *ToolRegistry, params string) (*mcp.ToolResponse, error) {
	t.Helper()
	return reg.SearchKnowledgeBase(context.Background(), mcp.ToolRequest{
		ID: "kb-1", Tool: "search_knowledge_base", Params: []byte(params),
	})
}

func TestSearchKnowledgeBase(t *testing.T) {
	passages := []map[string]interface{}{
		{"file": "abc", "seq": 0, "content": "Mileage is reimbursed at 88 cents per kilometre.", "distance": 0.14},
	}

	t.Run("passes the question to the app and returns its passages", func(t *testing.T) {
		client, rec := newKBFrappe(t, passages)
		resp, err := searchKB(t, NewRegistry(client), `{"query":"what is the mileage rate","limit":3}`)
		require.NoError(t, err)
		require.Len(t, resp.Content, 2)

		assert.Equal(t, []string{"/api/method/rag.search.search"}, rec.hits)
		assert.Equal(t, "what is the mileage rate", rec.sent["query"])
		assert.Equal(t, float64(3), rec.sent["limit"])

		var got []map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(resp.Content[1].Text), &got))
		require.Len(t, got, 1)
		assert.Contains(t, got[0]["content"], "88 cents")
	})

	t.Run("defaults the limit so the model cannot flood its own prompt", func(t *testing.T) {
		client, rec := newKBFrappe(t, passages)
		_, err := searchKB(t, NewRegistry(client), `{"query":"anything"}`)
		require.NoError(t, err)
		assert.Equal(t, float64(5), rec.sent["limit"])
	})

	t.Run("passes the chat on, so its attached files are searched too", func(t *testing.T) {
		client, rec := newKBFrappe(t, passages)
		_, err := searchKB(t, NewRegistry(client), `{"query":"q","session":"chat-1"}`)
		require.NoError(t, err)
		assert.Equal(t, "chat-1", rec.sent["session"])
	})

	t.Run("sends no chat when none is named", func(t *testing.T) {
		client, rec := newKBFrappe(t, passages)
		_, err := searchKB(t, NewRegistry(client), `{"query":"q"}`)
		require.NoError(t, err)
		_, sent := rec.sent["session"]
		assert.False(t, sent)
	})

	t.Run("a question is required", func(t *testing.T) {
		client, rec := newKBFrappe(t, passages)
		_, err := searchKB(t, NewRegistry(client), `{"limit":3}`)
		require.Error(t, err)
		assert.Empty(t, rec.hits, "an empty question must not reach the app")
	})
}
