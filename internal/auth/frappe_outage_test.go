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

// frappeThatAnswers returns the URL of a Frappe that answers every request with status, and of one that is down when
// status is 0.
func frappeThatAnswers(t *testing.T, status int) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	if status == 0 {
		server.Close() // nothing listens on that port any more
	}
	return server.URL
}

// A sid Frappe never judged is an outage, not a rejection: the agent turns 401 into "rejected the session" for the user,
// so a Frappe that is down or erroring has to come back as 503 (still fail-closed), and the cause has to be logged once.
func TestASidFrappeNeverJudgedIsAnOutageNotARejection(t *testing.T) {
	for _, c := range []struct {
		name       string
		frappe     int
		sid        string
		wantStatus int
	}{
		{"frappe is down", 0, "the-unjudged-sid", http.StatusServiceUnavailable},
		{"frappe errors", http.StatusInternalServerError, "the-unjudged-sid", http.StatusServiceUnavailable},
		{"frappe rejects the session", http.StatusForbidden, "the-rejected-sid", http.StatusUnauthorized},
		{"no sid and no bearer token", http.StatusOK, "", http.StatusUnauthorized},
	} {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })

			frappe := frappeThatAnswers(t, c.frappe)
			strategy := strategies.NewOAuth2Strategy(strategies.OAuth2StrategyConfig{
				TokenInfoURL: frappe + "/userinfo", IssuerURL: frappe, ValidateRemote: true, Timeout: 5 * time.Second,
			})
			handler := NewMiddleware(strategy, true).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("a refused request reached the handler")
			}))

			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if c.sid != "" {
				req.Header.Set("Cookie", "sid="+c.sid)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != c.wantStatus {
				t.Fatalf("status %d, want %d (body %s)", rec.Code, c.wantStatus, strings.TrimSpace(rec.Body.String()))
			}
			logged := out.String()
			if n := strings.Count(logged, "authentication failed"); n != 1 {
				t.Errorf("the refusal was logged %d times, want once:\n%s", n, logged)
			}
			if c.sid != "" && strings.Contains(logged, c.sid) {
				t.Errorf("the log holds the sid:\n%s", logged)
			}
		})
	}
}
