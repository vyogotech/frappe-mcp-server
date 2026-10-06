package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"frappe-mcp-server/internal/config"
)

// The only endpoints a probe may reach without credentials are the two health paths. /metrics reported
// time.Since(time.Now()) and a pinned version, and nothing read it; the chat and OpenAPI routes left with the
// server's own LLM pipeline (ADR-012).
func TestServedPathsAndProbes(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 0
	s, err := NewMCPServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewMCPServer: %v", err)
	}
	for _, path := range []string{"/metrics", "/api/v1/chat", "/api/v1/openapi.json"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		s.httpServer.Handler.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s: got status %d, want %d", path, w.Code, http.StatusNotFound)
		}
	}
	for _, path := range []string{"/health", "/api/v1/health"} {
		if !publicPaths[path] {
			t.Errorf("%s: a probe must reach it without credentials", path)
		}
	}
	if publicPaths["/metrics"] {
		t.Error("/metrics: no longer served, so nothing may skip auth for it")
	}
}
