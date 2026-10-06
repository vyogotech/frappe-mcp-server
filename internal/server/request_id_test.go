package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// One answer's agent log line and MCP log line join on the id the agent sends (ADR-008).
func TestTheAccessLogCarriesTheCallersRequestID(t *testing.T) {
	logged := accessLogFor(t, "3f2b1c4d5e6f7a8b9c0d1e2f3a4b5c6d")
	if !strings.Contains(logged, `"request_id":"3f2b1c4d5e6f7a8b9c0d1e2f3a4b5c6d"`) {
		t.Errorf("the access log does not carry the caller's request id:\n%s", logged)
	}
}

// An id from another service is attacker-controlled: anything but ADR-008's shape is replaced, so nothing
// unbounded reaches a log line.
func TestAnInboundRequestIDOfTheWrongShapeIsReplaced(t *testing.T) {
	for _, hostile := range []string{"has spaces", strings.Repeat("a", 65), "", "line\nbreak"} {
		logged := accessLogFor(t, hostile)
		if strings.Contains(logged, hostile) && hostile != "" {
			t.Errorf("the access log echoed %q:\n%s", hostile, logged)
		}
		if !strings.Contains(logged, `"request_id":"`) {
			t.Errorf("no request id was minted for %q:\n%s", hostile, logged)
		}
	}
}

func accessLogFor(t *testing.T, header string) string {
	t.Helper()
	var out bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	s := &MCPServer{}
	handler := s.loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("X-Request-ID", header)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	return out.String()
}
