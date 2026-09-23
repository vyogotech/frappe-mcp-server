package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
)

// A request body is read up to a fixed size, so one caller cannot make the server buffer an unbounded body.
func TestAnOversizedBodyIsRefusedBeforeItIsRead(t *testing.T) {
	cfg := &config.Config{ERPNext: config.ERPNextConfig{BaseURL: "http://frappe.invalid"}}
	client, err := frappe.NewClient(cfg.ERPNext)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewMCPServer(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		rec := httptest.NewRecorder()
		s.httpServer.Handler.ServeHTTP(rec, req)
		return rec
	}

	pad := strings.Repeat("x", 2<<20)
	if rec := post(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"pad":"` + pad + `"}}`); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("a 2 MiB body: status %d, want 413", rec.Code)
	}
	if rec := post(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"tools"`) {
		t.Errorf("an ordinary body: status %d body %.80s", rec.Code, rec.Body.String())
	}
}
