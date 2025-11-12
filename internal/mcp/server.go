package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// Server represents an MCP server
type Server struct {
	name      string
	version   string
	tools     map[string]ToolHandler
	resources map[string]string
	router    *mux.Router
	upgrader  websocket.Upgrader
}

// ToolHandler defines the interface for MCP tools
type ToolHandler func(ctx context.Context, request ToolRequest) (*ToolResponse, error)

// ToolRequest represents an MCP tool request
type ToolRequest struct {
	ID     string          `json:"id"`
	Tool   string          `json:"tool"`
	Params json.RawMessage `json:"params"`
}

// ToolResponse represents an MCP tool response
type ToolResponse struct {
	ID      string    `json:"id"`
	Content []Content `json:"content"`
	Error   *Error    `json:"error,omitempty"`
}

// Content represents MCP content
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
}

// Error represents an MCP error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewServer creates a new MCP server
func NewServer(name, version string) *Server {
	router := mux.NewRouter()

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for now
		},
	}

	server := &Server{
		name:      name,
		version:   version,
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]string),
		router:    router,
		upgrader:  upgrader,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// RegisterTool registers a new tool
func (s *Server) RegisterTool(name string, handler ToolHandler) {
	s.tools[name] = handler
	slog.Debug("Registered MCP tool", "name", name)
}

// RegisterResource registers a new resource
func (s *Server) RegisterResource(uri, description string) {
	s.resources[uri] = description
	slog.Debug("Registered MCP resource", "uri", uri)
}

// setupRoutes sets up HTTP routes
func (s *Server) setupRoutes() {
	s.router.HandleFunc("/mcp", s.handleWebSocket)
	s.router.HandleFunc("/tools", s.handleToolsList).Methods("GET")
	s.router.HandleFunc("/tools/{tool}", s.handleToolCall).Methods("POST")
	s.router.HandleFunc("/resources", s.handleResourcesList).Methods("GET")
}

// ListenAndServe starts the MCP server
func (s *Server) ListenAndServe(addr string) error {
	slog.Info("Starting MCP server", "addr", addr, "name", s.name, "version", s.version)

	server := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("Shutting down MCP server")
	return nil
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}
	defer func() { _ = conn.Close() }()

	slog.Info("WebSocket connection established")

	for {
		var request ToolRequest
		if err := conn.ReadJSON(&request); err != nil {
			slog.Error("Failed to read WebSocket message", "error", err)
			break
		}

		response := s.executeToolRequest(r.Context(), request)

		if err := conn.WriteJSON(response); err != nil {
			slog.Error("Failed to write WebSocket response", "error", err)
			break
		}
	}
}

// handleToolsList returns a list of available tools
func (s *Server) handleToolsList(w http.ResponseWriter, r *http.Request) {
	tools := make([]string, 0, len(s.tools))
	for name := range s.tools {
		tools = append(tools, name)
	}

	response := map[string]interface{}{
		"tools": tools,
		"count": len(tools),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleToolCall handles direct tool calls via HTTP
func (s *Server) handleToolCall(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	toolName := vars["tool"]

	if toolName == "" {
		http.Error(w, "Tool name is required", http.StatusBadRequest)
		return
	}

	var request ToolRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	request.Tool = toolName
	response := s.executeToolRequest(r.Context(), request)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleResourcesList returns a list of available resources
func (s *Server) handleResourcesList(w http.ResponseWriter, r *http.Request) {
	resources := make([]map[string]string, 0, len(s.resources))
	for uri, description := range s.resources {
		resources = append(resources, map[string]string{
			"uri":         uri,
			"description": description,
		})
	}

	response := map[string]interface{}{
		"resources": resources,
		"count":     len(resources),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// executeToolRequest executes a tool request
func (s *Server) executeToolRequest(ctx context.Context, request ToolRequest) *ToolResponse {
	if request.ID == "" {
		request.ID = fmt.Sprintf("req_%d", time.Now().UnixNano())
	}

	handler, exists := s.tools[request.Tool]
	if !exists {
		return &ToolResponse{
			ID: request.ID,
			Error: &Error{
				Code:    404,
				Message: fmt.Sprintf("Tool '%s' not found", request.Tool),
			},
		}
	}

	slog.Debug("Executing tool", "tool", request.Tool, "id", request.ID)

	response, err := handler(ctx, request)
	if err != nil {
		slog.Error("Tool execution failed", "tool", request.Tool, "error", err)
		return &ToolResponse{
			ID: request.ID,
			Error: &Error{
				Code:    500,
				Message: err.Error(),
			},
		}
	}

	if response.ID == "" {
		response.ID = request.ID
	}

	return response
}
