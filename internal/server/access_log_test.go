package server

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The access log records who asked for what and how it ended, never a request or response body: those hold users'
// questions, tool arguments and document passages.
func TestTheAccessLogHoldsNoRequestOrResponseContent(t *testing.T) {
	var out bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	s := &MCPServer{}
	handler := s.loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, _ := io.ReadAll(r.Body); !strings.Contains(string(body), "the-private-question") {
			t.Error("the handler did not get the request body")
		}
		_, _ = w.Write([]byte(`{"result":"the-private-passage"}`))
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"q":"the-private-question"}`)))

	logged := out.String()
	for _, want := range []string{`"path":"/mcp"`, `"status":200`} {
		if !strings.Contains(logged, want) {
			t.Errorf("the access log lacks %s:\n%s", want, logged)
		}
	}
	for _, private := range []string{"the-private-question", "the-private-passage"} {
		if strings.Contains(logged, private) {
			t.Errorf("the access log holds %q:\n%s", private, logged)
		}
	}
}
