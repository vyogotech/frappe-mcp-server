package auth

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"frappe-mcp-server/internal/auth/strategies"
)

// A refused request is a security event: it leaves one line with the path and the real reason (here, Frappe did not
// accept the sid), and never the credential itself.
func TestARefusedRequestIsLoggedWithItsReason(t *testing.T) {
	frappe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer frappe.Close()
	var out bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	strategy := strategies.NewOAuth2Strategy(strategies.OAuth2StrategyConfig{
		TokenInfoURL: frappe.URL + "/userinfo", IssuerURL: frappe.URL, ValidateRemote: true, Timeout: 5 * time.Second,
	})
	handler := NewMiddleware(strategy, true).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("a refused request reached the handler")
	}))
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Cookie", "sid=the-refused-sid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
	logged := out.String()
	for _, want := range []string{"authentication failed", `"path":"/mcp"`, "status 403"} {
		if !strings.Contains(logged, want) {
			t.Errorf("the log lacks %q:\n%s", want, logged)
		}
	}
	if strings.Contains(logged, "the-refused-sid") {
		t.Errorf("the log holds the sid:\n%s", logged)
	}
}
