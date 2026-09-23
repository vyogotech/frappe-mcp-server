package mcp

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A tool whose Frappe calls hang must end within the deadline with a tool error, not hold the caller until the relay
// behind it gives up.
func TestAToolCallEndsAtItsDeadline(t *testing.T) {
	previous := ToolDeadline
	ToolDeadline = 50 * time.Millisecond
	t.Cleanup(func() { ToolDeadline = previous })

	server := NewServer("test-server", "1.0.0")
	server.RegisterTool("hangs", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	done := make(chan rpcResponse, 1)
	go func() {
		done <- decodeRPC(t, postMCP(t, server,
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"hangs","arguments":{}}}`, nil))
	}()
	select {
	case resp := <-done:
		text := ""
		for _, c := range resp.Result.Content {
			text += c.Text
		}
		if !resp.Result.IsError || !strings.Contains(text, "deadline") {
			t.Fatalf("expected a deadline error, got %+v", resp)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the tool call ran past its deadline")
	}
}
