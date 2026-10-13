package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
)

func TestMultiTenantRequestRoutingAndHeaders(t *testing.T) {
	var bench1Calls, bench2Calls atomic.Int64
	var lastSiteHeader1, lastSiteHeader2 string

	bench1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bench1Calls.Add(1)
		lastSiteHeader1 = r.Header.Get("X-Frappe-Site-Name")
		switch r.URL.Path {
		case "/api/method/frappe.auth.get_logged_user":
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "user1@tenant1.local"})
		case "/app":
			_, _ = w.Write([]byte(`<script>frappe.csrf_token = "` + strings.Repeat("1", 32) + `";</script>`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer bench1.Close()

	bench2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bench2Calls.Add(1)
		lastSiteHeader2 = r.Header.Get("X-Frappe-Site-Name")
		switch r.URL.Path {
		case "/api/method/frappe.auth.get_logged_user":
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "user2@tenant2.local"})
		case "/app":
			_, _ = w.Write([]byte(`<script>frappe.csrf_token = "` + strings.Repeat("2", 32) + `";</script>`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer bench2.Close()

	tmpDir := t.TempDir()
	routesFile := filepath.Join(tmpDir, "routes.yaml")
	routesYAML := "routes:\n  tenant1.local: " + bench1.URL + "\n  tenant2.local: " + bench2.URL + "\n"
	if err := os.WriteFile(routesFile, []byte(routesYAML), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Server.Host = "127.0.0.1"
	cfg.ERPNext.BaseURL = bench1.URL
	cfg.ERPNext.SiteRoutesFile = routesFile
	cfg.Auth.Enabled = true
	cfg.Auth.RequireAuth = true
	cfg.Auth.OAuth2.ValidateRemote = true

	client, err := frappe.NewClient(cfg.ERPNext)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	s, err := NewMCPServer(cfg, client)
	if err != nil {
		t.Fatalf("NewMCPServer: %v", err)
	}

	// 1. Request for tenant1.local
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	req1.Header.Set("Cookie", "sid=sess-001")
	req1.Header.Set("X-Frappe-Site-Name", "tenant1.local")
	w1 := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("tenant1 request failed: status %d, body %s", w1.Code, w1.Body.String())
	}
	if bench1Calls.Load() == 0 {
		t.Errorf("bench1 was not called for tenant1.local")
	}
	if lastSiteHeader1 != "tenant1.local" {
		t.Errorf("bench1 expected X-Frappe-Site-Name=tenant1.local, got %q", lastSiteHeader1)
	}

	// 2. Request for tenant2.local
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	req2.Header.Set("Cookie", "sid=sess-002")
	req2.Header.Set("X-Frappe-Site-Name", "tenant2.local")
	w2 := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("tenant2 request failed: status %d, body %s", w2.Code, w2.Body.String())
	}
	if bench2Calls.Load() == 0 {
		t.Errorf("bench2 was not called for tenant2.local")
	}
	if lastSiteHeader2 != "tenant2.local" {
		t.Errorf("bench2 expected X-Frappe-Site-Name=tenant2.local, got %q", lastSiteHeader2)
	}

	// 3. Request for unknown site returns 400
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	req3.Header.Set("Cookie", "sid=sess-003")
	req3.Header.Set("X-Frappe-Site-Name", "unregistered.local")
	w3 := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w3, req3)

	if w3.Code != http.StatusBadRequest {
		t.Errorf("unknown site expected 400 Bad Request, got %d: %s", w3.Code, w3.Body.String())
	}

	// 4. Request with missing site when routes file configured returns 400
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	req4.Header.Set("Cookie", "sid=sess-004")
	w4 := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w4, req4)

	if w4.Code != http.StatusBadRequest {
		t.Errorf("missing site header with routes file expected 400 Bad Request, got %d: %s", w4.Code, w4.Body.String())
	}
}
