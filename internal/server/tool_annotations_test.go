package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
)

// writeTools is what the gate covers today. A tool added to toolCatalog with ReadOnly false and left out of here fails
// TestEveryWriteToolIsAccountedFor, which is how a new write tool reaches whoever has to wire it.
var writeTools = []string{"create_document", "update_document", "delete_document"}

func newAnnotationsServer(t *testing.T) *MCPServer {
	t.Helper()
	cfg := &config.Config{Tools: config.ToolsConfig{KnowledgeBase: true}}
	cfg.ERPNext.BaseURL = "http://frappe.invalid"
	cfg.ERPNext.APIKey, cfg.ERPNext.APISecret = "k", "s"
	cfg.Server.Port = 8080
	client, err := frappe.NewClient(cfg.ERPNext)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewMCPServer(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Every tool has to say whether it writes, because the gate keys on that and nothing else. A missing declaration must
// stop the server rather than publish a tool nothing will stop.
func TestRegisterToolsRefusesAnUndeclaredTool(t *testing.T) {
	s := newAnnotationsServer(t)

	for name, meta := range toolCatalog() {
		if meta.ReadOnly == nil {
			t.Errorf("toolCatalog()[%q] does not declare ReadOnly", name)
		}
	}

	t.Run("no catalogue entry", func(t *testing.T) {
		catalog := toolCatalog()
		delete(catalog, "create_document")
		err := s.registerTools(catalog)
		if err == nil || !strings.Contains(err.Error(), "create_document") {
			t.Fatalf("registerTools err = %v; want one naming create_document", err)
		}
	})

	t.Run("entry without a declaration", func(t *testing.T) {
		catalog := toolCatalog()
		meta := catalog["get_document"]
		meta.ReadOnly = nil
		catalog["get_document"] = meta
		err := s.registerTools(catalog)
		if err == nil || !strings.Contains(err.Error(), "get_document") {
			t.Fatalf("registerTools err = %v; want one naming get_document", err)
		}
	})

	t.Run("the whole catalogue registers", func(t *testing.T) {
		if err := s.registerTools(toolCatalog()); err != nil {
			t.Fatalf("registerTools: %v", err)
		}
	})
}

// The catalogue is the list the gate is driven from, so a write tool added later has to be added here too.
func TestEveryWriteToolIsAccountedFor(t *testing.T) {
	var found []string
	for name, meta := range toolCatalog() {
		if meta.ReadOnly != nil && !*meta.ReadOnly {
			found = append(found, name)
		}
	}
	want := map[string]bool{}
	for _, name := range writeTools {
		want[name] = true
	}
	for _, name := range found {
		if !want[name] {
			t.Errorf("toolCatalog declares %q a write tool; add it to writeTools and to the confirmation tests", name)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("%q is expected to be a write tool but the catalogue does not declare it one", name)
	}
}

// What a client actually reads off the wire: the reads carry readOnlyHint, the three writes do not.
func TestToolsListCarriesReadOnlyHint(t *testing.T) {
	s := newAnnotationsServer(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp",
		bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rec, req)

	var resp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Annotations *struct {
					ReadOnlyHint bool `json:"readOnlyHint"`
				} `json:"annotations"`
				InputSchema struct {
					Properties map[string]json.RawMessage `json:"properties"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode tools/list: %v (%s)", err, rec.Body.String())
	}
	if len(resp.Result.Tools) == 0 {
		t.Fatalf("tools/list returned nothing: %s", rec.Body.String())
	}

	writes := map[string]bool{}
	for _, name := range writeTools {
		writes[name] = true
	}
	listed := 0
	for _, tool := range resp.Result.Tools {
		listed++
		if tool.Annotations == nil {
			t.Errorf("%s published no annotations object", tool.Name)
			continue
		}
		if got, want := tool.Annotations.ReadOnlyHint, !writes[tool.Name]; got != want {
			t.Errorf("%s readOnlyHint = %v, want %v", tool.Name, got, want)
		}
		if tool.Name == "delete_document" {
			if _, ok := tool.InputSchema.Properties["confirm"]; ok {
				t.Error("delete_document still publishes a confirm argument the model can set itself")
			}
		}
	}
	if listed != len(toolCatalog()) {
		t.Errorf("tools/list published %d tools, catalogue has %d", listed, len(toolCatalog()))
	}
}
