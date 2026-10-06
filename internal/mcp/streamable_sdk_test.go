package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"frappe-mcp-server/internal/auth"
	"frappe-mcp-server/internal/types"
)

// rpcResponse is only what these tests read of a JSON-RPC response; the server no longer owns an envelope type.
type rpcResponse struct {
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Result struct {
		ProtocolVersion string `json:"protocolVersion"`
		Tools           []struct {
			Name string `json:"name"`
		} `json:"tools"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	} `json:"result"`
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	server := NewServer("test-server", "1.0.0")
	server.RegisterTool("stub_tool", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return &ToolResponse{ID: request.ID, Content: []Content{{Type: "text", Text: "stub-result"}}}, nil
	})
	server.RegisterTool("failing_tool", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return nil, errors.New("tool exploded")
	})
	server.RegisterTool("whoami", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		user := auth.UserFromContext(ctx)
		if user == nil {
			return nil, errors.New("no user in context")
		}
		return &ToolResponse{ID: request.ID, Content: []Content{{Type: "text", Text: user.Email}}}, nil
	})
	return server
}

func postMCP(t *testing.T, server *Server, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	server.StreamableHTTPHandler().ServeHTTP(rr, req)
	return rr
}

func decodeRPC(t *testing.T, rr *httptest.ResponseRecorder) rpcResponse {
	t.Helper()
	var resp rpcResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding %q: %v", rr.Body.String(), err)
	}
	return resp
}

// A notification carries no id, so there is nothing to answer: the transport spec requires 202 and no body
// (https://modelcontextprotocol.io/specification/2025-06-18/basic/transports, "Sending Messages to the Server").
func TestANotificationIsAcceptedWithNoBody(t *testing.T) {
	rr := postMCP(t, newTestServer(t), `{"jsonrpc":"2.0","method":"notifications/initialized"}`, nil)

	if rr.Code != http.StatusAccepted {
		t.Errorf("status %d, want 202", rr.Code)
	}
	if body := strings.TrimSpace(rr.Body.String()); body != "" {
		t.Errorf("body %q, want none", body)
	}
}

// initialize answers the version the client asked for, not one the server picked years ago
// (https://modelcontextprotocol.io/specification/versioning).
func TestInitializeAnswersTheVersionTheClientAsked(t *testing.T) {
	for _, version := range []string{"2025-06-18", "2025-03-26", "2024-11-05"} {
		rr := postMCP(t, newTestServer(t),
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"`+version+
				`","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
		}
		if got := decodeRPC(t, rr).Result.ProtocolVersion; got != version {
			t.Errorf("client asked %s, server answered %s", version, got)
		}
	}
}

// An unsupported MCP-Protocol-Version must be refused, not ignored
// (https://modelcontextprotocol.io/specification/2025-06-18/basic/transports, "Protocol Version Header").
func TestAnUnsupportedProtocolVersionHeaderIsRefused(t *testing.T) {
	rr := postMCP(t, newTestServer(t), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		map[string]string{"MCP-Protocol-Version": "1999-01-01"})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400: %s", rr.Code, rr.Body.String())
	}
}

func TestToolsListPublishesEveryRegisteredTool(t *testing.T) {
	resp := decodeRPC(t, postMCP(t, newTestServer(t), `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, nil))

	var names []string
	for _, tool := range resp.Result.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "failing_tool,stub_tool,whoami" {
		t.Errorf("tools/list returned %v", names)
	}
}

func TestAToolCallReturnsItsContent(t *testing.T) {
	resp := decodeRPC(t, postMCP(t, newTestServer(t),
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"stub_tool","arguments":{"doctype":"Customer"}}}`, nil))

	if resp.Error != nil {
		t.Fatalf("error %+v", resp.Error)
	}
	if len(resp.Result.Content) != 1 || resp.Result.Content[0].Text != "stub-result" {
		t.Errorf("content %+v", resp.Result.Content)
	}
	if resp.Result.IsError {
		t.Error("isError set on a successful call")
	}
}

// A tool that fails is a result the model can read, never a JSON-RPC error
// (https://modelcontextprotocol.io/specification/2025-06-18/server/tools, "Error Handling").
func TestAFailingToolIsAResultNotAProtocolError(t *testing.T) {
	resp := decodeRPC(t, postMCP(t, newTestServer(t),
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"failing_tool","arguments":{}}}`, nil))

	if resp.Error != nil {
		t.Fatalf("a failing tool answered with a JSON-RPC error: %+v", resp.Error)
	}
	if !resp.Result.IsError {
		t.Error("isError not set")
	}
	if len(resp.Result.Content) != 1 || !strings.Contains(resp.Result.Content[0].Text, "tool exploded") {
		t.Errorf("content %+v", resp.Result.Content)
	}
}

// A tool nobody registered is the caller's mistake, not a tool failure, so it stays a JSON-RPC error.
func TestAnUnknownToolIsAProtocolError(t *testing.T) {
	resp := decodeRPC(t, postMCP(t, newTestServer(t),
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nope","arguments":{}}}`, nil))

	if resp.Error == nil {
		t.Fatal("an unknown tool answered with a result")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("code %d, want -32602", resp.Error.Code)
	}
}

// Every request carries its own caller: the user the auth middleware put in the request context is the one the tool
// handler runs as, with no session left over from whoever initialized first.
func TestTheAuthenticatedUserReachesTheToolHandler(t *testing.T) {
	server := newTestServer(t)
	call := func(email string) string {
		req := httptest.NewRequest(http.MethodPost, "/mcp",
			strings.NewReader(`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"whoami","arguments":{}}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req = req.WithContext(auth.WithUser(req.Context(), &types.User{Email: email}))
		rr := httptest.NewRecorder()
		server.StreamableHTTPHandler().ServeHTTP(rr, req)
		resp := decodeRPC(t, rr)
		if len(resp.Result.Content) != 1 {
			t.Fatalf("content %+v", resp.Result)
		}
		return resp.Result.Content[0].Text
	}

	if got := call("alice@example.test"); got != "alice@example.test" {
		t.Errorf("tool ran as %q", got)
	}
	if got := call("bob@example.test"); got != "bob@example.test" {
		t.Errorf("tool ran as %q", got)
	}
}
