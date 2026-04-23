package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	gosdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracerName is the instrumentation scope for mcp dispatch spans.
const tracerName = "frappe-mcp-server/internal/mcp"

// Server wraps the go-sdk MCP server with a simplified registration API that
// maintains backward compatibility with the existing ToolHandler signature.
type Server struct {
	name      string
	version   string
	sdkServer *gosdk.Server
	// toolNames and resourceURIs are tracked locally for legacy helper methods.
	toolNames    []string
	resourceURIs []string
	// toolMeta stores the description + input schema for each registered tool
	// so the MCP tools/list handler can return real schemas to clients.
	toolMeta map[string]ToolMeta
}

// ToolMeta is the public metadata for a registered MCP tool — the pieces a
// client needs to know when the server answers tools/list.
type ToolMeta struct {
	Description string
	InputSchema map[string]interface{}
}

// ToolHandler defines the interface for MCP tools.
type ToolHandler func(ctx context.Context, request ToolRequest) (*ToolResponse, error)

// ToolRequest represents an MCP tool request (adapter type).
type ToolRequest struct {
	ID     string          `json:"id"`
	Tool   string          `json:"tool"`
	Params json.RawMessage `json:"params"`
}

// ToolResponse represents an MCP tool response (adapter type).
type ToolResponse struct {
	ID      string    `json:"id"`
	Content []Content `json:"content"`
	Error   *Error    `json:"error,omitempty"`
}

// Content represents MCP content.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
}

// Error represents an MCP error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewServer creates a new MCP server backed by the go-sdk.
func NewServer(name, version string) *Server {
	sdkServer := gosdk.NewServer(&gosdk.Implementation{
		Name:    name,
		Version: version,
	}, nil)

	return &Server{
		name:      name,
		version:   version,
		sdkServer: sdkServer,
		toolMeta:  make(map[string]ToolMeta),
	}
}

// SDKServer returns the underlying go-sdk server for direct access.
func (s *Server) SDKServer() *gosdk.Server {
	return s.sdkServer
}

// RegisterTool registers a tool with no published metadata — clients calling
// tools/list will see an empty description and a permissive input schema.
// Prefer RegisterToolWithSchema; this wrapper remains for callers that have
// no catalogued metadata to hand over.
func (s *Server) RegisterTool(name string, handler ToolHandler) {
	s.RegisterToolWithSchema(name, "", nil, handler)
}

// RegisterToolWithSchema registers a tool along with the description and input
// schema that tools/list should publish. inputSchema may be nil, in which case
// a permissive {"type":"object"} is used.
func (s *Server) RegisterToolWithSchema(name, description string, inputSchema map[string]interface{}, handler ToolHandler) {
	if s.toolMeta == nil {
		s.toolMeta = make(map[string]ToolMeta)
	}
	if inputSchema == nil {
		inputSchema = map[string]interface{}{"type": "object"}
	}
	s.toolMeta[name] = ToolMeta{Description: description, InputSchema: inputSchema}
	s.toolNames = append(s.toolNames, name)

	schemaBytes, err := json.Marshal(inputSchema)
	if err != nil {
		// Fall back to permissive schema if the provided one can't be marshalled.
		slog.Warn("Failed to marshal tool input schema; using permissive fallback", "tool", name, "err", err)
		schemaBytes = []byte(`{"type":"object"}`)
	}
	// Wrap as json.RawMessage so the go-sdk emits the bytes as a JSON object
	// rather than base64-encoding them as a string (the default for []byte).
	s.sdkServer.AddTool(
		&gosdk.Tool{
			Name:        name,
			Description: description,
			InputSchema: json.RawMessage(schemaBytes),
		},
		func(ctx context.Context, req *gosdk.CallToolRequest) (*gosdk.CallToolResult, error) {
			toolReq := ToolRequest{
				Tool:   name,
				Params: req.Params.Arguments,
			}
			resp, err := handler(ctx, toolReq)
			if err != nil {
				// Return as a tool-level error (IsError=true), not a protocol error.
				return &gosdk.CallToolResult{
					IsError: true,
					Content: []gosdk.Content{&gosdk.TextContent{Text: err.Error()}},
				}, nil
			}

			content := make([]gosdk.Content, 0, len(resp.Content))
			for _, c := range resp.Content {
				content = append(content, &gosdk.TextContent{Text: c.Text})
			}
			return &gosdk.CallToolResult{Content: content}, nil
		},
	)
	slog.Debug("Registered MCP tool", "name", name, "has_schema", len(inputSchema) > 1)
}

// ToolMetadata returns the description and input schema registered for a tool,
// or an empty ToolMeta with a permissive schema if the tool had no metadata.
func (s *Server) ToolMetadata(name string) ToolMeta {
	if meta, ok := s.toolMeta[name]; ok {
		return meta
	}
	return ToolMeta{InputSchema: map[string]interface{}{"type": "object"}}
}

// RegisterResource registers a resource with the go-sdk server.
func (s *Server) RegisterResource(uri, description string) {
	s.resourceURIs = append(s.resourceURIs, uri)
	s.sdkServer.AddResource(
		&gosdk.Resource{URI: uri, Description: description},
		func(ctx context.Context, req *gosdk.ReadResourceRequest) (*gosdk.ReadResourceResult, error) {
			return &gosdk.ReadResourceResult{}, nil
		},
	)
	slog.Debug("Registered MCP resource", "uri", uri)
}

// Run runs the server with the provided transport (e.g. &gosdk.StdioTransport{}).
// This is the preferred entry-point for the stdio binary.
func (s *Server) Run(ctx context.Context, transport gosdk.Transport) error {
	slog.Info("Starting MCP server", "name", s.name, "version", s.version)
	return s.sdkServer.Run(ctx, transport)
}

// ListenAndServe starts an SSE-based MCP server on the given address.
// This replaces the old WebSocket/HTTP server with the go-sdk SSE transport.
func (s *Server) ListenAndServe(addr string) error {
	slog.Info("Starting MCP SSE server", "addr", addr, "name", s.name, "version", s.version)

	handler := gosdk.NewSSEHandler(
		func(_ *http.Request) *gosdk.Server { return s.sdkServer },
		nil,
	)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("Shutting down MCP server")
	return nil
}

// executeToolRequest executes a tool request using the go-sdk server and emits
// an OpenTelemetry span for the call. Both the Streamable HTTP handler and any
// other caller share identical traces through this function.
func (s *Server) executeToolRequest(ctx context.Context, request ToolRequest) *ToolResponse {
	if request.ID == "" {
		request.ID = fmt.Sprintf("req_%d", time.Now().UnixNano())
	}

	ctx, span := otel.Tracer(tracerName).Start(ctx, "tool."+request.Tool,
		trace.WithAttributes(attribute.String("tool.name", request.Tool)),
	)
	defer span.End()

	// Best-effort: extract doctype from params for observability.
	if len(request.Params) > 0 {
		var paramsMap map[string]interface{}
		if err := json.Unmarshal(request.Params, &paramsMap); err == nil {
			if dt, ok := paramsMap["doctype"].(string); ok && dt != "" {
				span.SetAttributes(attribute.String("tool.doctype", dt))
			}
		}
	}

	// Use in-memory transports to exercise the go-sdk server.
	t1, t2 := gosdk.NewInMemoryTransports()
	_, err := s.sdkServer.Connect(ctx, t1, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return &ToolResponse{
			ID:    request.ID,
			Error: &Error{Code: 500, Message: fmt.Sprintf("failed to connect: %v", err)},
		}
	}

	client := gosdk.NewClient(&gosdk.Implementation{Name: "internal", Version: "1.0.0"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return &ToolResponse{
			ID:    request.ID,
			Error: &Error{Code: 500, Message: fmt.Sprintf("failed to connect client: %v", err)},
		}
	}
	defer func() { _ = cs.Close() }()

	slog.Debug("Executing tool", "tool", request.Tool, "id", request.ID)

	sdkResult, err := cs.CallTool(ctx, &gosdk.CallToolParams{
		Name:      request.Tool,
		Arguments: mustUnmarshalMap(request.Params),
	})
	if err != nil {
		slog.Error("Tool execution failed", "tool", request.Tool, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.Bool("tool.success", false))
		return &ToolResponse{
			ID:    request.ID,
			Error: &Error{Code: 500, Message: err.Error()},
		}
	}

	if sdkResult.IsError {
		msg := ""
		if len(sdkResult.Content) > 0 {
			if tc, ok := sdkResult.Content[0].(*gosdk.TextContent); ok {
				msg = tc.Text
			}
		}
		span.SetAttributes(attribute.Bool("tool.success", false))
		span.SetStatus(codes.Error, msg)
		return &ToolResponse{
			ID:    request.ID,
			Error: &Error{Code: 500, Message: msg},
		}
	}

	content := make([]Content, 0, len(sdkResult.Content))
	for _, c := range sdkResult.Content {
		if tc, ok := c.(*gosdk.TextContent); ok {
			content = append(content, Content{Type: "text", Text: tc.Text})
		}
	}
	span.SetAttributes(attribute.Bool("tool.success", true))
	return &ToolResponse{ID: request.ID, Content: content}
}

// mustUnmarshalMap decodes a JSON object into a map; returns an empty map on
// any error so that tool handlers always receive a valid (possibly empty) map.
func mustUnmarshalMap(data json.RawMessage) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{}
	}
	return m
}
