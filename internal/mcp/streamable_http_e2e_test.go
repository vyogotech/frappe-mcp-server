package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamableHTTP_EndToEnd(t *testing.T) {
	server := NewServer("test-server", "1.0.0")
	server.RegisterTool("ping", func(ctx context.Context, request ToolRequest) (*ToolResponse, error) {
		return &ToolResponse{
			ID:      request.ID,
			Content: []Content{{Type: "text", Text: "pong"}},
		}, nil
	})

	ts := httptest.NewServer(server.router)
	defer ts.Close()

	body := bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":42,"method":"tools/call",
		"params":{"name":"ping","arguments":{}}}`))
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/mcp", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var jr JSONRPCResponse
	require.NoError(t, json.Unmarshal(bodyBytes, &jr))
	require.Nil(t, jr.Error)
	require.NotNil(t, jr.Result)

	raw, err := json.Marshal(jr.Result)
	require.NoError(t, err)

	var result toolsCallResult
	require.NoError(t, json.Unmarshal(raw, &result))
	require.Len(t, result.Content, 1)
	assert.Equal(t, "pong", result.Content[0].Text)
}
