package mcp

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"frappe-mcp-server/internal/telemetry"
)

// The audit line says which tool ran; without the caller's id it cannot be joined to the question that
// asked for it.
func TestTheAuditLineCarriesTheCallersRequestID(t *testing.T) {
	var out bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	server := NewServer("test-server", "1.0.0")
	server.RegisterTool("get_document", func(_ context.Context, request ToolRequest) (*ToolResponse, error) {
		return &ToolResponse{ID: request.ID, Content: []Content{{Type: "text", Text: "ok"}}}, nil
	})

	ctx := telemetry.WithRequestID(context.Background(), "3f2b1c4d5e6f7a8b9c0d1e2f3a4b5c6d")
	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_document","arguments":{"doctype":"Note"}}}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	server.StreamableHTTPHandler().ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(out.String(), `"request_id":"3f2b1c4d5e6f7a8b9c0d1e2f3a4b5c6d"`) {
		t.Errorf("the audit line does not carry the caller's request id:\n%s", out.String())
	}
}
