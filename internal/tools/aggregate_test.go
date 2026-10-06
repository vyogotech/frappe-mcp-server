package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// countingFrappe records the calls of a stub whose get_list caps a page at Frappe's default 20 rows, while get_count
// returns the true total.
type countingFrappe struct {
	total    int
	hits     []string
	lastBody map[string]interface{}
}

func newCountingFrappe(t *testing.T, total int) (*frappe.Client, *countingFrappe) {
	t.Helper()
	rec := &countingFrappe{total: total}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.hits = append(rec.hits, r.URL.Path)
		raw, _ := io.ReadAll(r.Body)
		rec.lastBody = map[string]interface{}{}
		_ = json.Unmarshal(raw, &rec.lastBody)

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/method/frappe.client.get_count":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": rec.total})
		case "/api/method/frappe.client.get_list":
			page := rec.total
			if page > 20 {
				page = 20
			}
			rows := make([]map[string]interface{}, 0, page)
			for i := 0; i < page; i++ {
				rows = append(rows, map[string]interface{}{"name": fmt.Sprintf("ROW-%03d", i)})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": rows})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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

func aggregate(t *testing.T, reg *ToolRegistry, params string) map[string]interface{} {
	t.Helper()
	resp, err := reg.AggregateDocuments(context.Background(), mcp.ToolRequest{
		ID: "agg-1", Tool: "aggregate_documents", Params: []byte(params),
	})
	require.NoError(t, err)
	require.Len(t, resp.Content, 2)

	got := map[string]interface{}{}
	require.NoError(t, json.Unmarshal([]byte(resp.Content[1].Text), &got))
	return got
}

func TestAggregateDocuments_Count(t *testing.T) {
	t.Run("returns the true total, not one page of get_list", func(t *testing.T) {
		client, rec := newCountingFrappe(t, 50)
		got := aggregate(t, NewRegistry(client), `{"doctype":"Role","metric":"count"}`)

		assert.Equal(t, float64(50), got["count"])
		assert.Equal(t, "count", got["metric"])
		assert.NotContains(t, got, "results", "a count returns no rows")
		assert.Equal(t, []string{"/api/method/frappe.client.get_count"}, rec.hits)
	})

	t.Run("never sends limit, which would cap the count", func(t *testing.T) {
		client, rec := newCountingFrappe(t, 50)
		aggregate(t, NewRegistry(client), `{"doctype":"Role","metric":"count","limit":5}`)

		// frappe.desk.reportview.get_count reads the whole request body via
		// frappe.form_dict, so any limit key there caps the returned count.
		assert.NotContains(t, rec.lastBody, "limit")
		assert.NotContains(t, rec.lastBody, "limit_page_length")
	})

	t.Run("forwards filters", func(t *testing.T) {
		client, rec := newCountingFrappe(t, 47)
		got := aggregate(t, NewRegistry(client), `{"doctype":"Role","metric":"count","filters":{"desk_access":1}}`)

		assert.Equal(t, float64(47), got["count"])
		assert.Equal(t, map[string]interface{}{"desk_access": float64(1)}, rec.lastBody["filters"])
	})

	t.Run("accepts any casing or padding of the metric", func(t *testing.T) {
		for _, metric := range []string{"count", "COUNT", "Count", " count "} {
			client, _ := newCountingFrappe(t, 50)
			got := aggregate(t, NewRegistry(client), fmt.Sprintf(`{"doctype":"Role","metric":%q}`, metric))
			assert.Equal(t, float64(50), got["count"], "metric %q", metric)
		}
	})

	t.Run("a grouped count still goes through get_list", func(t *testing.T) {
		client, rec := newCountingFrappe(t, 50)
		got := aggregate(t, NewRegistry(client), `{"doctype":"Role","metric":"count","group_by":"desk_access"}`)

		// get_list returns one row per group, so its row count is the answer.
		assert.Equal(t, []string{"/api/method/frappe.client.get_list"}, rec.hits)
		assert.Contains(t, got, "results")
	})

	t.Run("requires a doctype", func(t *testing.T) {
		client, _ := newCountingFrappe(t, 50)
		_, err := NewRegistry(client).AggregateDocuments(context.Background(), mcp.ToolRequest{
			ID: "agg-1", Tool: "aggregate_documents", Params: []byte(`{"metric":"count"}`),
		})
		assert.EqualError(t, err, "doctype is required")
	})
}
