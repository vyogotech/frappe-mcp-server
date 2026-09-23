package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
)

// A browser page on another origin cannot drive /mcp with the user's cookie: its request is refused with 403 (the MCP
// specification's Origin rule), no route grants other origins access, and a server-to-server call (no Origin) passes.
func TestACrossOriginBrowserRequestIsRefused(t *testing.T) {
	cfg := &config.Config{ERPNext: config.ERPNextConfig{BaseURL: "http://frappe.invalid"}}
	client, err := frappe.NewClient(cfg.ERPNext)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewMCPServer(cfg, client)
	if err != nil {
		t.Fatal(err)
	}
	post := func(origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if origin != "" {
			req.Header.Set("Origin", origin)
			req.Header.Set("Sec-Fetch-Site", "cross-site")
		}
		rec := httptest.NewRecorder()
		s.httpServer.Handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := post("https://evil.example"); rec.Code != http.StatusForbidden {
		t.Errorf("a cross-origin browser POST: status %d, want 403", rec.Code)
	}
	rec := post("")
	if rec.Code != http.StatusOK {
		t.Errorf("a server-to-server POST: status %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want none", got)
	}
}
