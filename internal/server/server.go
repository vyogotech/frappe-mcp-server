package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"frappe-mcp-server/internal/auth"
	"frappe-mcp-server/internal/auth/strategies"
	"frappe-mcp-server/internal/buildinfo"
	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/mcp"
	"frappe-mcp-server/internal/tools"
)

type MCPServer struct {
	config         *config.Config
	frappeClient   *frappe.Client
	server         *mcp.Server
	httpServer     *http.Server
	catalog        []tools.Tool
	authMiddleware *auth.Middleware
}

// tool finds the tool this server offers under name; registration, the listings and both dispatch paths read the one
// table, so nothing can be advertised without being callable.
func (s *MCPServer) tool(name string) (tools.Tool, bool) {
	for _, candidate := range s.catalog {
		if candidate.Name == name {
			return candidate, true
		}
	}
	return tools.Tool{}, false
}

// maxRequestBody bounds every request body: a tool call is a small JSON document.
const maxRequestBody = 1 << 20

func NewMCPServer(cfg *config.Config, frappeClient *frappe.Client) (*MCPServer, error) {
	// Create MCP server
	server := mcp.NewServer("frappe-mcp-server", buildinfo.Version())

	// Create tool registry
	toolRegistry := tools.NewRegistry(frappeClient)
	toolRegistry.ConfirmationRedeemMethod = cfg.Tools.ConfirmationRedeemMethod

	mcpServer := &MCPServer{
		config:       cfg,
		frappeClient: frappeClient,
		server:       server,
		catalog:      toolRegistry.Catalog(cfg.Tools.KnowledgeBase),
	}

	// Setup authentication if enabled
	if cfg.Auth.Enabled {
		slog.Info("Authentication enabled", "require_auth", cfg.Auth.RequireAuth)
		oauth2Strategy := strategies.NewOAuth2Strategy(strategies.OAuth2StrategyConfig{
			TokenInfoURL:   cfg.Auth.OAuth2.TokenInfoURL,
			BaseURL:        cfg.ERPNext.BaseURL,
			TrustedClients: cfg.Auth.OAuth2.TrustedClients,
			Timeout:        cfg.Auth.OAuth2.Timeout,
			CacheTTL:       cfg.Auth.TokenCache.TTL,
			ValidateRemote: cfg.Auth.OAuth2.ValidateRemote,
		})
		mcpServer.authMiddleware = auth.NewMiddleware(oauth2Strategy, cfg.Auth.RequireAuth)
	} else {
		slog.Info("Authentication disabled")
	}

	// Setup HTTP server with health checks and MCP endpoints
	mux := http.NewServeMux()

	// withMiddleware authenticates this route too; mounted outside that chain, every tool call would run as nobody.
	mcpHandler := mcpServer.server.StreamableHTTPHandler()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		// the SDK reads the body itself and calls an over-long one a bad request; too long is 413
		if r.ContentLength > maxRequestBody {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})

	// The REST tool routes, kept for callers that speak plain HTTP rather than MCP (ADR-012).
	mux.HandleFunc("/api/v1/health", mcpServer.healthCheck)
	mux.HandleFunc("/api/v1/tools", mcpServer.listTools)
	mux.HandleFunc("/api/v1/tools/", mcpServer.handleToolCall)

	mux.HandleFunc("/health", mcpServer.healthCheck)
	mux.HandleFunc("/tools", mcpServer.listTools)
	mux.HandleFunc("/tool/", mcpServer.handleToolCall)

	mcpServer.httpServer = &http.Server{
		Addr:              net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port)),
		Handler:           http.MaxBytesHandler(mcpServer.withMiddleware(mux), maxRequestBody),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.Server.Timeout,
		WriteTimeout:      cfg.Server.Timeout,
		IdleTimeout:       120 * time.Second,
	}

	// Register all tools
	if err := mcpServer.registerTools(mcpServer.catalog); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	return mcpServer, nil
}

func (s *MCPServer) Run(ctx context.Context) error {
	slog.Info("Starting MCP server",
		"host", s.config.Server.Host,
		"port", s.config.Server.Port)

	// Start HTTP server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		slog.Info("Starting HTTP server", "address", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		slog.Info("Shutting down MCP server...")
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()

		slog.Info("Shutting down HTTP server...")
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		}
		return nil
	case err := <-errChan:
		slog.Error("Server error", "error", err)
		return err
	}
}

func (s *MCPServer) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Simple health check - could be enhanced to check ERPNext connectivity
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`)); err != nil {
		slog.Error("Failed to write health response", "error", err)
	}
}

// publicPaths skip auth so probes need no credentials; never add a tool path here.
var publicPaths = map[string]bool{
	"/health":        true,
	"/api/v1/health": true,
}

// withMiddleware skips auth for publicPaths only; recovery, logging and cross-origin protection wrap every path.
func (s *MCPServer) withMiddleware(handler http.Handler) http.Handler {
	// innermost, so /mcp and both REST tool paths carry the write gate's token the same way
	inner := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := r.Header.Get("X-Frappe-Confirmation"); token != "" {
			r = r.WithContext(auth.WithConfirmation(r.Context(), token))
		}
		handler.ServeHTTP(w, r)
	}))
	h := inner

	// Apply auth middleware if enabled, but skip for public paths.
	if s.authMiddleware != nil {
		authed := s.authMiddleware.Handler(inner)
		h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if publicPaths[r.URL.Path] {
				inner.ServeHTTP(w, r)
				return
			}
			authed.ServeHTTP(w, r)
		})
	}

	// Apply cross-origin protection and logging (these always run, including for public paths). A browser page on
	// another origin gets 403, as the MCP specification requires; server-to-server calls carry no Origin and pass.
	h = http.NewCrossOriginProtection().Handler(h)
	h = s.loggingMiddleware(h)
	// Recovery is outermost so it catches panics in every other middleware.
	h = s.recoveryMiddleware(h)

	return h
}

// recoveryMiddleware answers a panic anywhere in the chain with a 500 and a logged stack.
func (s *MCPServer) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("http handler panic recovered",
					"panic", fmt.Sprintf("%v", rec),
					"method", strings.ReplaceAll(r.Method, "\n", " "),
					"path", strings.ReplaceAll(r.URL.Path, "\n", " "),
					"stack", string(debug.Stack()),
				)
				// Safe after headers are sent (a logged no-op); before, the client gets a 500 instead of a hung request.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs each request's method, path, status, size and duration. Never a body: bodies hold users'
// questions, tool arguments and document passages (the tool call itself is audited in internal/mcp).
func (s *MCPServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(ww, r)

		// the health probe runs every few seconds, and at Info it is most of the log
		level := slog.LevelInfo
		if publicPaths[r.URL.Path] {
			level = slog.LevelDebug
		}
		slog.Log(r.Context(), level, "HTTP request",
			"method", strings.ReplaceAll(r.Method, "\n", " "),
			"path", strings.ReplaceAll(r.URL.Path, "\n", " "),
			"remote_addr", strings.ReplaceAll(r.RemoteAddr, "\n", " "),
			"user_agent", strings.ReplaceAll(r.UserAgent(), "\n", " "),
			"status", ww.status,
			"bytes", ww.bytes,
			"duration", time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusRecorder) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Flush keeps the wrapper an http.Flusher; without it the SSE handler's type assertion fails with "SSE not supported".
func (w *statusRecorder) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer for hijacking and deadlines.
func (w *statusRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// registerTools publishes catalog; the stdio binary registers from the same table, through tools.Register, which is
// also where a tool that never declared whether it writes stops the server.
func (s *MCPServer) registerTools(catalog []tools.Tool) error {
	if err := tools.Register(s.server, catalog); err != nil {
		return err
	}

	slog.Info("Registered MCP tools", "count", len(catalog))
	return nil
}

// listTools serves REST /tools from the table the tools are registered from; a legacy tool with no description stays
// callable but unlisted.
func (s *MCPServer) listTools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	listed := make([]map[string]interface{}, 0, len(s.catalog))
	for _, tool := range s.catalog {
		if tool.Description == "" {
			continue
		}
		listed = append(listed, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		})
	}

	response := map[string]interface{}{
		"tools": listed,
		"count": len(listed),
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode tools response", "error", err)
	}
}

func (s *MCPServer) handleToolCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		slog.Warn("Tool method not allowed",
			"method", strings.ReplaceAll(r.Method, "\n", " "),
			"path", strings.ReplaceAll(r.URL.Path, "\n", " "))
		return
	}

	// Support both /tool/ and /api/v1/tools/ prefixes.
	// Sanitize toolName immediately so all subsequent log calls are safe (gosec G706).
	rawPath := r.URL.Path
	rawTool := strings.TrimPrefix(rawPath, "/api/v1/tools/")
	if rawTool == rawPath {
		rawTool = strings.TrimPrefix(rawPath, "/tool/")
	}
	toolName := strings.ReplaceAll(rawTool, "\n", " ")
	if toolName == "" || toolName == strings.ReplaceAll(rawPath, "\n", " ") {
		http.Error(w, "Tool name is required", http.StatusBadRequest)
		slog.Warn("Missing tool name", "path", strings.ReplaceAll(rawPath, "\n", " "))
		return
	}

	var request mcp.ToolRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		slog.Error("/tool/ invalid request body", "error", err)
		return
	}

	slog.Info("Tool call received", "tool", toolName, "request_id", request.ID)
	request.Tool = toolName
	if request.ID == "" {
		request.ID = fmt.Sprintf("http-%d", time.Now().UnixNano())
	}

	ctx, cancel := context.WithTimeout(r.Context(), mcp.ToolDeadline)
	defer cancel()

	slog.Info("Executing tool", "tool", toolName, "request_id", request.ID)
	tool, ok := s.tool(toolName)
	if !ok {
		http.Error(w, "Tool not found", http.StatusNotFound)
		slog.Warn("Tool not found", "tool", toolName)
		return
	}
	result, err := tool.Handler(ctx, request)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		slog.Error("Tool execution error", "tool", toolName, "error", err)
		return
	}

	slog.Info("Tool executed successfully", "tool", toolName, "request_id", request.ID)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		slog.Error("Failed to encode tool response", "error", err)
	}
}
