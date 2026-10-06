package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gosdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"frappe-mcp-server/internal/auth"
	"frappe-mcp-server/internal/types"
)

// Each tool call leaves one line saying who called which tool and how it ended, over HTTP and stdio alike, and never
// what was asked or answered.
func TestEveryToolCallLeavesOneAuditLine(t *testing.T) {
	var out bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	server := NewServer("test-server", "1.0.0")
	server.RegisterTool("update_document", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return &ToolResponse{ID: request.ID, Content: []Content{{Type: "text", Text: "the-private-result"}}}, nil
	})
	server.RegisterTool("broken", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return nil, errors.New("frappe said no")
	})

	// the HTTP path: tools/call over /mcp, with the user the auth middleware attached
	ctx := auth.WithUser(context.Background(), &types.User{Email: "alice@example.test"})
	call := func(body string) {
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		server.StreamableHTTPHandler().ServeHTTP(httptest.NewRecorder(), req)
	}
	call(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"update_document",` +
		`"arguments":{"doctype":"ToDo","name":"TD-1","data":{"description":"the-private-argument"}}}}`)
	call(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"broken","arguments":{}}}`)

	// the stdio path: a client talks to the SDK server directly, not over HTTP
	serverSide, clientSide := gosdk.NewInMemoryTransports()
	if _, err := server.sdkServer.Connect(context.Background(), serverSide, nil); err != nil {
		t.Fatal(err)
	}
	client, err := gosdk.NewClient(&gosdk.Implementation{Name: "stdio-like", Version: "1"}, nil).Connect(context.Background(), clientSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	if _, err := client.CallTool(context.Background(), &gosdk.CallToolParams{Name: "update_document", Arguments: map[string]any{"doctype": "Note"}}); err != nil {
		t.Fatal(err)
	}

	var calls []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err == nil && entry["msg"] == "tool call" {
			calls = append(calls, entry)
		}
	}
	want := []map[string]any{
		{"user": "alice@example.test", "tool": "update_document", "doctype": "ToDo", "name": "TD-1", "outcome": "ok"},
		{"user": "alice@example.test", "tool": "broken", "outcome": "tool_error", "error_type": "*errors.errorString"},
		{"user": "", "tool": "update_document", "doctype": "Note", "outcome": "ok"},
	}
	if len(calls) != len(want) {
		t.Fatalf("want one audit line per call, got %d in:\n%s", len(calls), out.String())
	}
	for i, fields := range want {
		for key, value := range fields {
			if calls[i][key] != value {
				t.Errorf("call %d: %s = %v, want %v", i, key, calls[i][key], value)
			}
		}
	}
	if strings.Contains(out.String(), "the-private-argument") || strings.Contains(out.String(), "the-private-result") {
		t.Errorf("an argument or result value reached the log:\n%s", out.String())
	}
}
