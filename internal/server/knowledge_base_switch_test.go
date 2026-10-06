package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
)

// search_knowledge_base answers only where the rag app is installed, so a server offers it only when told to.
func TestTheKnowledgeBaseToolIsOfferedOnlyWhenSwitchedOn(t *testing.T) {
	erp := config.ERPNextConfig{BaseURL: "http://frappe.test", APIKey: "k", APISecret: "s",
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 1, Burst: 1}, Retry: config.RetryConfig{MaxAttempts: 1}}
	client, err := frappe.NewClient(erp)
	if err != nil {
		t.Fatal(err)
	}
	for _, on := range []bool{false, true} {
		s, err := NewMCPServer(&config.Config{ERPNext: erp, Tools: config.ToolsConfig{KnowledgeBase: on}}, client)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		s.listTools(w, httptest.NewRequest("GET", "/tools", nil))
		if listed := strings.Contains(w.Body.String(), `"search_knowledge_base"`); listed != on {
			t.Errorf("knowledge_base %v: search_knowledge_base listed %v", on, listed)
		}
	}
}
