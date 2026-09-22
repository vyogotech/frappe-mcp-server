package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	gosdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"frappe-mcp-server/internal/auth"
)

const tracerName = "frappe-mcp-server/internal/mcp"

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

// ToolMeta is what tools/list publishes for a tool.
type ToolMeta struct {
	Description string
	InputSchema map[string]interface{}
}

type ToolHandler func(ctx context.Context, request ToolRequest) (*ToolResponse, error)

type ToolRequest struct {
	ID     string          `json:"id"`
	Tool   string          `json:"tool"`
	Params json.RawMessage `json:"params"`
}

type ToolResponse struct {
	ID      string    `json:"id"`
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
	Error   *Error    `json:"error,omitempty"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

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

func (s *Server) SDKServer() *gosdk.Server {
	return s.sdkServer
}

// RegisterTool publishes the tool with an empty description and a permissive schema; prefer RegisterToolWithSchema.
func (s *Server) RegisterTool(name string, handler ToolHandler) {
	s.RegisterToolWithSchema(name, "", nil, handler)
}

// RegisterToolWithSchema publishes description and inputSchema in tools/list; a nil inputSchema allows any object.
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
			ctx, cancel := context.WithTimeout(ctx, ToolDeadline)
			defer cancel()
			toolReq := ToolRequest{
				Tool:   name,
				Params: req.Params.Arguments,
			}
			start := time.Now()
			resp, err := handler(ctx, toolReq)
			auditToolCall(ctx, name, req.Params.Arguments, err, time.Since(start))
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

// ToolDeadline bounds one tool call, Frappe retries included: frappe_ai's relay gives up after 120 s without a byte, and
// the agent sends none while a tool runs.
var ToolDeadline = 60 * time.Second

// auditToolCall writes the one line every tool call leaves, over HTTP and stdio alike: who, which tool and record, how
// it ended. Never an argument or result value, which hold users' questions and documents.
func auditToolCall(ctx context.Context, tool string, arguments json.RawMessage, err error, took time.Duration) {
	var ids struct{ Doctype, Name string }
	_ = json.Unmarshal(arguments, &ids)
	user, outcome, errorType := "", "ok", ""
	if u := auth.UserFromContext(ctx); u != nil {
		user = u.Email
	}
	if err != nil {
		outcome, errorType = "tool_error", fmt.Sprintf("%T", err)
	}
	slog.Info("tool call", "user", user, "tool", tool, "doctype", ids.Doctype, "name", ids.Name,
		"outcome", outcome, "error_type", errorType, "duration_ms", took.Milliseconds())
}

// ToolMetadata gives a name never registered an empty description and a permissive schema.
func (s *Server) ToolMetadata(name string) ToolMeta {
	if meta, ok := s.toolMeta[name]; ok {
		return meta
	}
	return ToolMeta{InputSchema: map[string]interface{}{"type": "object"}}
}

// RegisterResource registers a resource whose reads return an empty result.
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

func (s *Server) Run(ctx context.Context, transport gosdk.Transport) error {
	slog.Info("Starting MCP server", "name", s.name, "version", s.version)
	return s.sdkServer.Run(ctx, transport)
}

// executeToolRequest calls the tool through the go-sdk server, not its handler, so a /mcp tool call gets the same
// deadline and audit line as a stdio one.
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
		// the tool ran and failed: MCP reports that as a result the model can read, not as a JSON-RPC error
		return &ToolResponse{
			ID:      request.ID,
			Content: []Content{{Type: "text", Text: msg}},
			IsError: true,
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

// mustUnmarshalMap returns an empty map, never an error, for anything that is not a JSON object.
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
