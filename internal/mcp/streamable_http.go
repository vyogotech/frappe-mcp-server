package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

// HandleStreamableHTTP serves POST /mcp (Streamable HTTP, JSON responses only); mount it behind the auth middleware.
// Only transport errors (method, headers, body size) get an HTTP error status; JSON-RPC and tool errors are 200.
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
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
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

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// dispatchToolsCall runs tools/call through executeToolRequest, which owns the tool's OpenTelemetry span.
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
	var args map[string]json.RawMessage
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return newJSONRPCError(req.ID, JSONRPCInvalidParams, "tools/call params.arguments must be an object")
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
		IsError: resp.IsError,
	})
}

// writeJSONRPC always answers 200: a JSON-RPC error travels in the body, not in the HTTP status.
func writeJSONRPC(w http.ResponseWriter, resp JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode JSON-RPC response", "error", err)
	}
}

// acceptsJSON reports whether a non-empty Accept header allows application/json; callers accept an empty one.
func acceptsJSON(accept string) bool {
	for _, part := range strings.Split(accept, ",") {
		mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		if mediaType == "application/json" || mediaType == "*/*" || mediaType == "application/*" {
			return true
		}
	}
	return false
}
