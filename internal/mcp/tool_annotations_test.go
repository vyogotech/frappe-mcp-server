package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

// A client has to be able to tell a read from a write before it calls one, and MCP's way of saying so is
// annotations.readOnlyHint. A tool registered with no declaration publishes no annotation at all, which is what makes
// the agent treat it as a write.
func TestToolsListPublishesReadOnlyHint(t *testing.T) {
	readOnly := func(b bool) *bool { return &b }
	handler := func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return &ToolResponse{ID: request.ID}, nil
	}

	server := NewServer("test-server", "1.0.0")
	server.RegisterToolWithSchema("a_read", ToolMeta{Description: "reads", ReadOnly: readOnly(true)}, handler)
	server.RegisterToolWithSchema("a_write", ToolMeta{Description: "writes", ReadOnly: readOnly(false)}, handler)
	server.RegisterTool("undeclared", handler)

	rr := postMCP(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, nil)
	var resp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Annotations *struct {
					ReadOnlyHint *bool `json:"readOnlyHint"`
				} `json:"annotations"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode tools/list: %v (%s)", err, rr.Body.String())
	}

	got := map[string]*bool{}
	seen := map[string]bool{}
	for _, tool := range resp.Result.Tools {
		seen[tool.Name] = true
		if tool.Annotations != nil {
			got[tool.Name] = tool.Annotations.ReadOnlyHint
		}
	}
	for _, name := range []string{"a_read", "a_write", "undeclared"} {
		if !seen[name] {
			t.Fatalf("tools/list did not list %q: %s", name, rr.Body.String())
		}
	}

	if hint, ok := got["a_read"]; !ok || hint == nil || !*hint {
		t.Errorf("a_read: readOnlyHint = %v, want true", hint)
	}
	// readOnlyHint is omitted when false, so what a write publishes is an annotations object without it
	if hint, ok := got["a_write"]; !ok {
		t.Error("a_write published no annotations object")
	} else if hint != nil && *hint {
		t.Error("a_write published readOnlyHint true")
	}
	if _, ok := got["undeclared"]; ok {
		t.Error("a tool registered with no ReadOnly declaration published an annotation")
	}
}
