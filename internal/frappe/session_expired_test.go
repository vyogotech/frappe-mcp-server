package frappe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
)

// the body audit.localhost answers with when a sid has ended: the session_expired flag, no message of its own
const endedSessionBody = `{"session_expired":1,"exc_type":"PermissionError","_server_messages":` +
	`"[\"{\\\"message\\\":\\\"You are not permitted to access this resource. Login to access\\\"}\"]"}`

func clientAgainst(t *testing.T, baseURL string) *Client {
	t.Helper()
	client, err := NewClient(config.ERPNextConfig{
		BaseURL: baseURL, APIKey: "k", APISecret: "s", Timeout: 5 * time.Second,
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 10, Burst: 10},
		Retry:     config.RetryConfig{MaxAttempts: 1, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// A tool's error is what the model repeats to the user, so a session that ended has to ask for a new sign-in, not tell
// the user they lack permission.
func TestAnEndedSessionAsksForANewSignIn(t *testing.T) {
	frappe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(endedSessionBody))
	}))
	defer frappe.Close()

	_, err := clientAgainst(t, frappe.URL).GetDocument(context.Background(), "Sales Invoice", "SINV-00001")
	if err == nil {
		t.Fatal("want the 403 as an error")
	}
	if !strings.Contains(err.Error(), "Session expired. Please sign in again.") {
		t.Errorf("the user is not asked to sign in again: %v", err)
	}
	for _, wrong := range []string{"permission", "API key"} {
		if strings.Contains(err.Error(), wrong) {
			t.Errorf("the ended session still reads as %q: %v", wrong, err)
		}
	}
}

// The other 403: a live session that may not read this record keeps the advice it had.
func TestADeniedRecordStillReadsAsPermissionDenied(t *testing.T) {
	frappe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"exc_type":"PermissionError"}`))
	}))
	defer frappe.Close()

	_, err := clientAgainst(t, frappe.URL).GetDocument(context.Background(), "Sales Invoice", "SINV-00001")
	if err == nil {
		t.Fatal("want the 403 as an error")
	}
	if !strings.Contains(err.Error(), "Permission denied (HTTP 403)") {
		t.Errorf("a real denial lost its message: %v", err)
	}
	if strings.Contains(err.Error(), "Session expired") {
		t.Errorf("a real denial reads as an ended session: %v", err)
	}
}
