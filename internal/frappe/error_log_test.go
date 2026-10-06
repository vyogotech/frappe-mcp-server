package frappe

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
)

// A Frappe error reaches the log and the caller as its path, status and Frappe's one-line exception: never the query
// string, which holds the user's question for rag.search, and never the raw body, whose traceback can hold document text.
func TestAFrappeErrorCarriesNeitherTheQuestionNorTheRawBody(t *testing.T) {
	frappe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"exc_type":"PermissionError","exception":"frappe.exceptions.PermissionError: No permission",` +
			`"exc":"[\"Traceback ... the-private-traceback\"]"}`))
	}))
	defer frappe.Close()
	var out bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	client, err := NewClient(config.ERPNextConfig{
		BaseURL: frappe.URL, APIKey: "k", APISecret: "s", Timeout: 5 * time.Second,
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 10, Burst: 10},
		Retry:     config.RetryConfig{MaxAttempts: 1, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.SearchKnowledgeBase(context.Background(), "the-private-question", 5, "")
	if err == nil {
		t.Fatal("want the 403 as an error")
	}

	logged := out.String()
	for _, want := range []string{"/api/method/rag.search.search", `"status_code":403`, "PermissionError"} {
		if !strings.Contains(logged, want) {
			t.Errorf("the error log lacks %s:\n%s", want, logged)
		}
	}
	if !strings.Contains(err.Error(), "No permission") {
		t.Errorf("the error lost Frappe's own message: %v", err)
	}
	for _, private := range []string{"the-private-question", "the-private-traceback"} {
		if strings.Contains(logged, private) || strings.Contains(err.Error(), private) {
			t.Errorf("%q reached the log or the error:\nlog: %s\nerror: %v", private, logged, err)
		}
	}
}
