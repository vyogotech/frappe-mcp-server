package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer returns an mcp.Server with one stub tool registered. Used by
// the streamable HTTP handler tests.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	server := NewServer("test-server", "1.0.0")
	server.RegisterTool("stub_tool", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return &ToolResponse{
			ID:      request.ID,
			Content: []Content{{Type: "text", Text: "stub-result"}},
		}, nil
	})
	return server
}

func postMCP(t *testing.T, server *Server, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)
	return rr
}

func TestStreamableHTTP_WrongContentType(t *testing.T) {
	rr := postMCP(t, newTestServer(t), `{}`, map[string]string{
		"Content-Type": "text/plain",
	})
	assert.Equal(t, http.StatusUnsupportedMediaType, rr.Code)
}

func TestStreamableHTTP_SSEOnlyAccept(t *testing.T) {
	rr := postMCP(t, newTestServer(t), `{}`, map[string]string{
		"Accept": "text/event-stream",
	})
	assert.Equal(t, http.StatusNotAcceptable, rr.Code)
}

func TestStreamableHTTP_MalformedJSON(t *testing.T) {
	rr := postMCP(t, newTestServer(t), `{not json`, nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp JSONRPCResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, JSONRPCParseError, resp.Error.Code)
}

func TestStreamableHTTP_MissingMethod(t *testing.T) {
	rr := postMCP(t, newTestServer(t), `{"jsonrpc":"2.0","id":1}`, nil)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp JSONRPCResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, JSONRPCInvalidRequest, resp.Error.Code)
}

func TestStreamableHTTP_GETStillRoutesToWebSocket(t *testing.T) {
	// A GET on /mcp should NOT hit the streamable handler. It should reach the
	// existing WebSocket upgrade path. Since httptest does not perform WS
	// upgrades, we simply assert the response did not come from streamableHTTP
	// (which would set Content-Type: application/json). The WS upgrader writes
	// a 400-class response when no Upgrade header is present.
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rr := httptest.NewRecorder()
	server.router.ServeHTTP(rr, req)

	// The gorilla/websocket upgrader returns 400 when the Upgrade header is
	// missing. If we saw any 2xx or 4xx response with Content-Type
	// application/json, the route would have dispatched to the streamable
	// handler instead.
	assert.Equal(t, http.StatusBadRequest, rr.Code,
		"GET /mcp without Upgrade header should hit the WS handler (400)")
	assert.NotEqual(t, "application/json", rr.Header().Get("Content-Type"),
		"GET /mcp should not be routed to streamable HTTP handler")
}
