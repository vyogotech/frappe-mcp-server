package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The container probes /health every 30 s; at the default level those probes leave no lines of their own.
func TestAHealthProbeLogsNothingAtInfo(t *testing.T) {
	var out bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	(&MCPServer{}).healthCheck(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))

	if strings.Contains(out.String(), "/health") {
		t.Errorf("a health probe logged at info:\n%s", out.String())
	}
}
