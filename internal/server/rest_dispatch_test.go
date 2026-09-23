package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
)

// GET /tools and POST /tool/ were two hand-kept copies of the tool list, so three advertised tools answered 404.
func TestEveryAdvertisedToolDispatchesOverREST(t *testing.T) {
	cfg := &config.Config{
		ERPNext: config.ERPNextConfig{
			BaseURL:   "http://frappe.invalid",
			RateLimit: config.RateLimitConfig{RequestsPerSecond: 100, Burst: 100},
			Retry:     config.RetryConfig{MaxAttempts: 1},
		},
		Tools: config.ToolsConfig{KnowledgeBase: true},
	}
	client, err := frappe.NewClient(cfg.ERPNext)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewMCPServer(cfg, client)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	s.listTools(rec, httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil))
	var listing struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if len(listing.Tools) == 0 {
		t.Fatal("/tools advertised nothing")
	}

	for _, tool := range listing.Tools {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tools/"+tool.Name, strings.NewReader(`{"params":{}}`))
		s.handleToolCall(w, req)
		if w.Code == http.StatusNotFound {
			t.Errorf("/tools advertises %s but POST /api/v1/tools/%s answered 404", tool.Name, tool.Name)
		}
	}
}
