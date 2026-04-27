package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// HandleStreamableHTTP is the POST /mcp handler implementing the JSON-RPC 2.0
// "Streamable HTTP" transport from the MCP specification, JSON-only response
// flavour. SSE upgrade is not supported (returns 406).
//
// This method is public so it can be registered on the REST API HTTP mux in
// internal/server, which already has the OAuth2 auth middleware applied.
//
// Errors are categorised:
//   - Transport-level (bad headers)        → non-200 HTTP response
//   - Protocol-level (bad JSON, bad shape) → 200 + JSON-RPC error body
//   - Application-level (tool failure)     → 200 + JSON-RPC error body
func (s *Server) HandleStreamableHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Validate Content-Type. We accept only application/json (with optional
	//    charset). Anything else is a transport error.
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if !strings.HasPrefix(contentType, "application/json") {
		slog.Debug("POST /mcp rejected: wrong Content-Type", "content_type", strings.ReplaceAll(contentType, "\n", " "))
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	// 2. Validate Accept. We accept anything that does not exclude
	//    application/json. The pure-SSE flavour ("text/event-stream" only) is
	//    rejected because we do not implement SSE responses in this server.
	accept := r.Header.Get("Accept")
	if accept != "" && !acceptsJSON(accept) {
		slog.Debug("POST /mcp rejected: SSE-only Accept", "accept", strings.ReplaceAll(accept, "\n", " "))
		http.Error(w, "Accept must include application/json", http.StatusNotAcceptable)
		return
	}

	// 3. Decode JSON-RPC request body. Malformed JSON is a JSON-RPC parse error.
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONRPC(w, newJSONRPCError(nil, JSONRPCParseError, "Parse error: "+err.Error()))
		return
	}

	// 4. Validate JSON-RPC envelope.
	if req.JSONRPC != "2.0" || req.Method == "" {
		writeJSONRPC(w, newJSONRPCError(req.ID, JSONRPCInvalidRequest, "Invalid Request: missing jsonrpc or method"))
		return
	}

	// 5. Dispatch by method.
	resp := s.dispatchJSONRPC(r.Context(), req)
	writeJSONRPC(w, resp)
}

// dispatchJSONRPC routes a JSON-RPC request to the appropriate handler.
func (s *Server) dispatchJSONRPC(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.dispatchInitialize(req)
	case "tools/list":
		return s.dispatchToolsList(req)
	case "tools/call":
		return s.dispatchToolsCall(ctx, req)
	default:
		return newJSONRPCError(req.ID, JSONRPCMethodNotFound, "Method not found: "+req.Method)
	}
}

// dispatchInitialize handles the JSON-RPC "initialize" method. Returns the
// MCP protocol version and the server's tool capability advertisement.
func (s *Server) dispatchInitialize(req JSONRPCRequest) JSONRPCResponse {
	return newJSONRPCResult(req.ID, initializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		ServerInfo: serverInfo{
			Name:    s.name,
			Version: s.version,
		},
	})
}

// dispatchToolsList handles the JSON-RPC "tools/list" method. Returns each
// registered tool with the description and input schema supplied at
// registration time. Tools registered via the bare RegisterTool (no metadata)
// still get a permissive {"type":"object"} schema, preserving legacy behaviour.
func (s *Server) dispatchToolsList(req JSONRPCRequest) JSONRPCResponse {
	tools := make([]toolDefinition, 0, len(s.toolNames))
	for _, name := range s.toolNames {
		meta := s.ToolMetadata(name)
		tools = append(tools, toolDefinition{
			Name:        name,
			Description: meta.Description,
			InputSchema: meta.InputSchema,
		})
	}
	return newJSONRPCResult(req.ID, toolsListResult{Tools: tools})
}

// toolsCallParams is the params shape for the JSON-RPC tools/call method.
type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// dispatchToolsCall handles the JSON-RPC "tools/call" method. It translates
// the JSON-RPC request into the internal ToolRequest shape, runs it through
// executeToolRequest (which emits the OpenTelemetry span), then translates
// the resulting ToolResponse back into the JSON-RPC tools/call result shape.
func (s *Server) dispatchToolsCall(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	if len(req.Params) == 0 {
		return newJSONRPCError(req.ID, JSONRPCInvalidParams, "tools/call requires params")
	}

	var params toolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return newJSONRPCError(req.ID, JSONRPCInvalidParams, "Invalid params: "+err.Error())
	}
	if params.Name == "" {
		return newJSONRPCError(req.ID, JSONRPCInvalidParams, "tools/call requires params.name")
	}

	// Check tool exists before dispatching to return a proper -32601 error.
	found := false
	for _, name := range s.toolNames {
		if name == params.Name {
			found = true
			break
		}
	}
	if !found {
		return newJSONRPCError(req.ID, JSONRPCMethodNotFound, "unknown tool: "+params.Name)
	}

	// Default arguments to an empty object so tools that ignore params can be
	// called with `arguments` omitted entirely.
	if len(params.Arguments) == 0 {
		params.Arguments = json.RawMessage(`{}`)
	}

	toolReq := ToolRequest{
		ID:     string(req.ID),
		Tool:   params.Name,
		Params: params.Arguments,
	}

	resp := s.executeToolRequest(ctx, toolReq)

	if resp.Error != nil {
		return newJSONRPCError(req.ID, JSONRPCServerError, resp.Error.Message)
	}

	content := make([]toolContent, 0, len(resp.Content))
	for _, c := range resp.Content {
		content = append(content, toolContent{
			Type: c.Type,
			Text: c.Text,
		})
	}
	return newJSONRPCResult(req.ID, toolsCallResult{
		Content: content,
		IsError: false,
	})
}

// writeJSONRPC encodes a JSON-RPC response and writes it with HTTP 200.
// JSON-RPC errors travel in the body, not the HTTP layer (per spec), so this
// helper always uses status 200.
func writeJSONRPC(w http.ResponseWriter, resp JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode JSON-RPC response", "error", err)
	}
}

// acceptsJSON reports whether the Accept header allows application/json.
// Empty header means "anything", which is fine. */* is fine. Specific
// application/json is fine. Pure text/event-stream is not.
func acceptsJSON(accept string) bool {
	for _, part := range strings.Split(accept, ",") {
		mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		if mediaType == "application/json" || mediaType == "*/*" || mediaType == "application/*" {
			return true
		}
	}
	return false
}
