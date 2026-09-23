package mcp

import (
	"testing"
)

// Arguments that are not an object are the caller's mistake (invalid params), not an empty call for the tool to refuse.
func TestToolsCallRejectsArgumentsThatAreNotAnObject(t *testing.T) {
	resp := decodeRPC(t, postMCP(t, newTestServer(t),
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"stub_tool","arguments":[1,2]}}`, nil))

	if resp.Error == nil {
		t.Fatalf("arguments [1,2] answered with a result: %+v", resp.Result)
	}
	if resp.Error.Code != -32602 {
		t.Errorf("code %d, want -32602", resp.Error.Code)
	}
}
