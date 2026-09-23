package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/mcp"
)

// rowsFrappe answers a list with as many rows as it was asked for and a report with reportRows rows,
// recording the query string of the last list call.
func rowsFrappe(t *testing.T, reportRows int) (*ToolRegistry, *string) {
	t.Helper()
	lastQuery := new(string)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "query_report.run") {
			data := make([][]interface{}, reportRows)
			for i := range data {
				data[i] = []interface{}{fmt.Sprintf("ROW-%04d", i), i}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": map[string]interface{}{
				"columns": []interface{}{map[string]interface{}{"label": "Name"}, map[string]interface{}{"label": "N"}},
				"result":  data,
			}})
			return
		}
		*lastQuery = r.URL.Query().Get("limit_page_length")
		asked := 20
		if n := r.URL.Query().Get("limit_page_length"); n != "" {
			_, _ = fmt.Sscanf(n, "%d", &asked)
		}
		rows := make([]map[string]interface{}, asked)
		for i := range rows {
			rows[i] = map[string]interface{}{"name": fmt.Sprintf("DOC-%04d", i)}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": rows})
	}))
	t.Cleanup(srv.Close)

	client, err := frappe.NewClient(config.ERPNextConfig{
		BaseURL:   srv.URL,
		APIKey:    "k",
		APISecret: "s",
		Timeout:   5 * time.Second,
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 1000, Burst: 1000},
		Retry:     config.RetryConfig{MaxAttempts: 1, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewRegistry(client), lastQuery
}

func call(t *testing.T, handler mcp.ToolHandler, args string) *mcp.ToolResponse {
	t.Helper()
	resp, err := handler(context.Background(), mcp.ToolRequest{ID: "t", Params: json.RawMessage(args)})
	if err != nil {
		t.Fatalf("tool call: %v", err)
	}
	return resp
}

// A model may ask for any page_length; the server must not pass it on.
func TestListDocumentsClampsPageLength(t *testing.T) {
	registry, lastQuery := rowsFrappe(t, 0)

	call(t, registry.ListDocuments, `{"doctype":"Project","page_length":10000}`)

	if want := fmt.Sprint(maxRows); *lastQuery != want {
		t.Errorf("Frappe was asked for limit_page_length=%s, want %s", *lastQuery, want)
	}
}

func TestSearchDocumentsClampsPageLength(t *testing.T) {
	registry, lastQuery := rowsFrappe(t, 0)

	call(t, registry.SearchDocuments, `{"doctype":"Project","search":"x","page_length":10000}`)

	if want := fmt.Sprint(maxRows); *lastQuery != want {
		t.Errorf("Frappe was asked for limit_page_length=%s, want %s", *lastQuery, want)
	}
}

// A report answers with every row it has; the result the model reads must be bounded, and must say so.
func TestRunReportTruncatesAndSaysSo(t *testing.T) {
	registry, _ := rowsFrappe(t, 500)

	resp := call(t, registry.RunReport, `{"report_name":"Sales Analytics"}`)

	var payload struct {
		Data      []interface{} `json:"data"`
		RowCount  int           `json:"row_count"`
		Truncated bool          `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(resp.Content[1].Text), &payload); err != nil {
		t.Fatalf("report payload: %v", err)
	}
	if len(payload.Data) != maxRows {
		t.Errorf("rows in the result = %d, want %d", len(payload.Data), maxRows)
	}
	if payload.RowCount != 500 {
		t.Errorf("row_count = %d, want the report's own 500", payload.RowCount)
	}
	if !payload.Truncated {
		t.Error("truncated flag is not set on a report that was cut short")
	}
	if !strings.Contains(resp.Content[0].Text, "500") || !strings.Contains(resp.Content[0].Text, fmt.Sprint(maxRows)) {
		t.Errorf("the summary the model reads does not say what was cut: %q", resp.Content[0].Text)
	}
}
