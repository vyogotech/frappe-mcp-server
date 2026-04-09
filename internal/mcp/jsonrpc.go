// internal/mcp/jsonrpc.go
package mcp

import "encoding/json"

// JSON-RPC 2.0 standard error codes (https://www.jsonrpc.org/specification#error_object).
const (
	JSONRPCParseError     = -32700 // Invalid JSON received
	JSONRPCInvalidRequest = -32600 // Required fields missing
	JSONRPCMethodNotFound = -32601 // Unknown method or unknown tool
	JSONRPCInvalidParams  = -32602 // Tool argument validation failed
	JSONRPCInternalError  = -32603 // Recovered panic
	JSONRPCServerError    = -32000 // Generic tool execution failure
)

// JSONRPCRequest is a JSON-RPC 2.0 request envelope.
//
// ID may be a number or a string per spec; we keep it as RawMessage and echo
// it verbatim in the response.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response envelope. Exactly one of Result
// or Error is set in any successfully-encoded response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError is the error object inside a JSON-RPC 2.0 response.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// newJSONRPCError builds a JSON-RPC error response that echoes the original
// request ID. Used by the streamable HTTP handler whenever dispatch fails.
func newJSONRPCError(id json.RawMessage, code int, message string) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
}

// newJSONRPCResult builds a successful JSON-RPC response with the given result
// payload.
func newJSONRPCResult(id json.RawMessage, result interface{}) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// MCP-specific result payload shapes (from the MCP specification, not JSON-RPC).

// initializeResult is the response to the "initialize" method.
type initializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      serverInfo             `json:"serverInfo"`
}

// serverInfo describes this MCP server.
type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// toolDefinition is one entry in the tools/list result.
type toolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// toolsListResult is the response to the "tools/list" method.
type toolsListResult struct {
	Tools []toolDefinition `json:"tools"`
}

// toolContent is one item in a tools/call result content array.
type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// toolsCallResult is the response to the "tools/call" method.
type toolsCallResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError"`
}
