package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
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
	// toolNames is tracked locally for legacy helper methods.
	toolNames []string
	// toolMeta stores the description + input schema for each registered tool
	// so the MCP tools/list handler can return real schemas to clients.
	toolMeta map[string]ToolMeta
}

// ToolMeta is what tools/list publishes for a tool. ReadOnly is nil when the tool never declared whether it writes;
// the server refuses to register such a tool, so the gate is never skipped by omission. OutputSchema is nil unless
// the tool fills ToolResponse.Structured, and then it describes that value: a result the tool returns must conform.
type ToolMeta struct {
	Description  string
	InputSchema  map[string]interface{}
	OutputSchema map[string]interface{}
	ReadOnly     *bool
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
	// Structured is the result as data, for a client that reads it instead of parsing the text blocks. It must
	// marshal to a JSON object and conform to the tool's declared OutputSchema.
	Structured any    `json:"structuredContent,omitempty"`
	IsError    bool   `json:"isError,omitempty"`
	Error      *Error `json:"error,omitempty"`
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

// StreamableHTTPHandler serves /mcp. Stateless: each request gets its own session, so the caller the auth middleware
// put in the request context is the one every tool handler runs as.
func (s *Server) StreamableHTTPHandler() http.Handler {
	return gosdk.NewStreamableHTTPHandler(
		func(*http.Request) *gosdk.Server { return s.sdkServer },
		&gosdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)
}

// RegisterTool publishes the tool with an empty description, a permissive schema and no ReadOnly declaration; prefer
// RegisterToolWithSchema.
func (s *Server) RegisterTool(name string, handler ToolHandler) {
	s.RegisterToolWithSchema(name, ToolMeta{}, handler)
}

// RegisterToolWithSchema publishes meta in tools/list; a nil InputSchema allows any object, and a declared ReadOnly
// becomes the tool's annotations.readOnlyHint.
func (s *Server) RegisterToolWithSchema(name string, meta ToolMeta, handler ToolHandler) {
	if s.toolMeta == nil {
		s.toolMeta = make(map[string]ToolMeta)
	}
	inputSchema := meta.InputSchema
	if inputSchema == nil {
		inputSchema = map[string]interface{}{"type": "object"}
	}
	meta.InputSchema = inputSchema
	s.toolMeta[name] = meta
	s.toolNames = append(s.toolNames, name)

	var annotations *gosdk.ToolAnnotations
	if meta.ReadOnly != nil {
		annotations = &gosdk.ToolAnnotations{ReadOnlyHint: *meta.ReadOnly}
	}

	schemaBytes, err := json.Marshal(inputSchema)
	if err != nil {
		// Fall back to permissive schema if the provided one can't be marshalled.
		slog.Warn("Failed to marshal tool input schema; using permissive fallback", "tool", name, "err", err)
		schemaBytes = []byte(`{"type":"object"}`)
	}
	// Wrap as json.RawMessage so the go-sdk emits the bytes as a JSON object
	// rather than base64-encoding them as a string (the default for []byte).
	tool := &gosdk.Tool{
		Name:        name,
		Description: meta.Description,
		InputSchema: json.RawMessage(schemaBytes),
		Annotations: annotations,
	}
	if meta.OutputSchema != nil {
		// assigned only when there is one: a nil map inside the interface is not nil, and the sdk panics on it
		tool.OutputSchema = meta.OutputSchema
	}
	s.sdkServer.AddTool(
		tool,
		func(ctx context.Context, req *gosdk.CallToolRequest) (*gosdk.CallToolResult, error) {
			var args map[string]json.RawMessage
			if len(req.Params.Arguments) > 0 {
				if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
					// the caller's mistake, not a call for the tool to refuse
					return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "tools/call params.arguments must be an object"}
				}
			}

			ctx, span := otel.Tracer(tracerName).Start(ctx, "tool."+name,
				trace.WithAttributes(attribute.String("tool.name", name)))
			defer span.End()
			var doctype string
			if json.Unmarshal(args["doctype"], &doctype) == nil && doctype != "" {
				span.SetAttributes(attribute.String("tool.doctype", doctype))
			}

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
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.SetAttributes(attribute.Bool("tool.success", false))
				// the tool ran and failed: MCP reports that as a result the model can read, not as a protocol error
				return &gosdk.CallToolResult{
					IsError: true,
					Content: []gosdk.Content{&gosdk.TextContent{Text: err.Error()}},
				}, nil
			}

			content := make([]gosdk.Content, 0, len(resp.Content))
			for _, c := range resp.Content {
				content = append(content, &gosdk.TextContent{Text: c.Text})
			}
			span.SetAttributes(attribute.Bool("tool.success", true))
			return &gosdk.CallToolResult{Content: content, StructuredContent: resp.Structured}, nil
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

func (s *Server) Run(ctx context.Context, transport gosdk.Transport) error {
	slog.Info("Starting MCP server", "name", s.name, "version", s.version)
	return s.sdkServer.Run(ctx, transport)
}
