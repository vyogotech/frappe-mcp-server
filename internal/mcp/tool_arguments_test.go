package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Arguments that are not an object are the caller's mistake (invalid params), not an empty call for the tool to refuse.
func TestToolsCallRejectsArgumentsThatAreNotAnObject(t *testing.T) {
	rr := postMCP(t, newTestServer(t),
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"stub_tool","arguments":[1,2]}}`, nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp JSONRPCResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, JSONRPCInvalidParams, resp.Error.Code)
}
