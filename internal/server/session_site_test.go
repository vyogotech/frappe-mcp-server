package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"frappe-mcp-server/internal/config"
)

// A sid is valid only on the site that issued it, so the session and its CSRF token have to be checked on the site the
// tool calls go to, erpnext.base_url, whatever host auth.oauth2.issuer_url still names.
func TestTheSidIsCheckedOnTheSiteTheToolCallsGoTo(t *testing.T) {
	var otherSiteCalls atomic.Int64
	otherSite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		otherSiteCalls.Add(1)
		w.WriteHeader(http.StatusForbidden) // a site refuses a session it never issued
	}))
	defer otherSite.Close()

	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/method/frappe.auth.get_logged_user":
			_, _ = w.Write([]byte(`{"message":"alice@example.com"}`))
		case "/app":
			_, _ = w.Write([]byte(`<script>frappe.csrf_token = "` + strings.Repeat("a", 32) + `";</script>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer site.Close()

	cfg := &config.Config{}
	cfg.Server.Host = "127.0.0.1"
	cfg.ERPNext.BaseURL = site.URL
	cfg.Auth.Enabled = true
	cfg.Auth.RequireAuth = true
	cfg.Auth.OAuth2.ValidateRemote = true
	cfg.Auth.OAuth2.IssuerURL = otherSite.URL // the site the server was pointed away from
	cfg.Auth.OAuth2.TokenInfoURL = otherSite.URL + "/userinfo"

	s, err := NewMCPServer(cfg, nil)
	if err != nil {
		t.Fatalf("NewMCPServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	req.Header.Set("Cookie", "sid=a-sid-the-site-issued")
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status %d, want 200: the sid was not accepted", w.Code)
	}
	if n := otherSiteCalls.Load(); n != 0 {
		t.Errorf("the sid was sent to issuer_url %d times; it is valid only on the site that issued it", n)
	}
}
