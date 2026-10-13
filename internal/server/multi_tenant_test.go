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

// fakeBench answers as one tenant's Frappe would: get_logged_user names the user, /app carries a CSRF token, and
// every call is counted with the site header it arrived with, which is what the routing assertions read.
type fakeBench struct {
	*httptest.Server
	calls    atomic.Int64
	lastSite string
}

func newFakeBench(t *testing.T, user, csrfDigit string) *fakeBench {
	t.Helper()
	b := &fakeBench{}
	b.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.calls.Add(1)
		b.lastSite = r.Header.Get("X-Frappe-Site-Name")
		switch r.URL.Path {
		case "/api/method/frappe.auth.get_logged_user":
			_ = json.NewEncoder(w).Encode(map[string]string{"message": user})
		case "/app":
			_, _ = w.Write([]byte(`<script>frappe.csrf_token = "` + strings.Repeat(csrfDigit, 32) + `";</script>`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(b.Close)
	return b
}

// serveAs sends one tools request as a session on a site; an empty site sends no site header.
func serveAs(s *MCPServer, site, sid string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	req.Header.Set("Cookie", "sid="+sid)
	if site != "" {
		req.Header.Set("X-Frappe-Site-Name", site)
	}
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)
	return w
}

// serveWithBench sends one tools request as a session on a site with an explicit bench URL header.
func serveWithBench(s *MCPServer, site, benchURL, sid string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	req.Header.Set("Cookie", "sid="+sid)
	if site != "" {
		req.Header.Set("X-Frappe-Site-Name", site)
	}
	if benchURL != "" {
		req.Header.Set("X-Frappe-Bench-URL", benchURL)
	}
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)
	return w
}

func TestMultiTenantRequestRoutingAndHeaders(t *testing.T) {
	bench1 := newFakeBench(t, "user1@tenant1.local", "1")
	bench2 := newFakeBench(t, "user2@tenant2.local", "2")
	benchDynamic := newFakeBench(t, "user3@dynamic.local", "3")

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

	t.Run("tenant1 is served by bench1 with its site named", func(t *testing.T) {
		w := serveAs(s, "tenant1.local", "sess-001")
		if w.Code != http.StatusOK {
			t.Errorf("status %d, body %s", w.Code, w.Body.String())
		}
		if bench1.calls.Load() == 0 || bench1.lastSite != "tenant1.local" {
			t.Errorf("bench1 calls=%d site header=%q", bench1.calls.Load(), bench1.lastSite)
		}
	})

	t.Run("tenant2 is served by bench2 with its site named", func(t *testing.T) {
		w := serveAs(s, "tenant2.local", "sess-002")
		if w.Code != http.StatusOK {
			t.Errorf("status %d, body %s", w.Code, w.Body.String())
		}
		if bench2.calls.Load() == 0 || bench2.lastSite != "tenant2.local" {
			t.Errorf("bench2 calls=%d site header=%q", bench2.calls.Load(), bench2.lastSite)
		}
	})

	t.Run("dynamic bench URL routes directly even when absent from routes file", func(t *testing.T) {
		w := serveWithBench(s, "dynamic.local", benchDynamic.URL, "sess-003")
		if w.Code != http.StatusOK {
			t.Errorf("status %d, body %s", w.Code, w.Body.String())
		}
		if benchDynamic.calls.Load() == 0 || benchDynamic.lastSite != "dynamic.local" {
			t.Errorf("benchDynamic calls=%d site header=%q", benchDynamic.calls.Load(), benchDynamic.lastSite)
		}
	})

	t.Run("dynamic bench URL overrides routes file mapping", func(t *testing.T) {
		callsBefore := benchDynamic.calls.Load()
		w := serveWithBench(s, "tenant1.local", benchDynamic.URL, "sess-004")
		if w.Code != http.StatusOK {
			t.Errorf("status %d, body %s", w.Code, w.Body.String())
		}
		if benchDynamic.calls.Load() <= callsBefore {
			t.Errorf("expected benchDynamic to receive call, calls=%d", benchDynamic.calls.Load())
		}
	})

	t.Run("invalid X-Frappe-Bench-URL header is rejected with 400", func(t *testing.T) {
		w := serveWithBench(s, "tenant1.local", "not-a-valid-url", "sess-005")
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	for name, site := range map[string]string{
		"a site absent from the routes file": "unregistered.local",
		"no site at all, with a routes file": "",
	} {
		t.Run(name+" is refused with 400", func(t *testing.T) {
			if w := serveAs(s, site, "sess-006"); w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
