package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"frappe-mcp-server/internal/auth"
	"frappe-mcp-server/internal/auth/strategies"
	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/llm"
	"frappe-mcp-server/internal/mcp"
	"frappe-mcp-server/internal/tools"
	"frappe-mcp-server/internal/types"
)

// MCPServer represents the MCP server
type MCPServer struct {
	config         *config.Config
	frappeClient   *frappe.Client
	llmClient      llm.Client
	server         *mcp.Server
	httpServer     *http.Server
	tools          *tools.ToolRegistry
	authMiddleware *auth.Middleware
}

// QueryIntent represents the extracted intent from a natural language query
type QueryIntent struct {
	Action           string                 // The intended action (get, list, search, analyze, etc.)
	DocType          string                 // ERPNext doctype (Project, Customer, Item, etc.)
	EntityName       string                 // Specific entity name if querying a specific document
	Tool             string                 // The MCP tool to call
	Params           json.RawMessage        // Parameters for the tool
	RequiresSearch   bool                   // Whether we need to search for the entity first
	IsERPNextRelated bool                   // Whether the query is about ERPNext data or general
	Confidence       float64                // AI confidence in the extraction (0-1)
}

// SearchResult represents the result of searching for an entity
type SearchResult struct {
	EntityName string // The actual name/ID of the found entity
	DocType    string // The doctype
	MatchScore float64 // How well it matched the search term
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer(cfg *config.Config, frappeClient *frappe.Client) (*MCPServer, error) {
	// Create MCP server
	server := mcp.NewServer("frappe-mcp-server", "1.0.0")

	// Create tool registry
	toolRegistry := tools.NewRegistry(frappeClient)
	
	// Create LLM client
	llmClient, err := llm.NewClient(cfg.LLM)
	if err != nil {
		slog.Warn("Failed to initialize LLM client", "error", err)
		slog.Info("AI-powered query processing will be disabled")
	}

	mcpServer := &MCPServer{
		config:       cfg,
		frappeClient: frappeClient,
		llmClient:    llmClient,
		server:       server,
		tools:        toolRegistry,
	}

	// Setup authentication if enabled
	if cfg.Auth.Enabled {
		slog.Info("Authentication enabled", "require_auth", cfg.Auth.RequireAuth)
		oauth2Strategy := strategies.NewOAuth2Strategy(strategies.OAuth2StrategyConfig{
			TokenInfoURL:   cfg.Auth.OAuth2.TokenInfoURL,
			IssuerURL:      cfg.Auth.OAuth2.IssuerURL,
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
	
	// API v1 endpoints (for Open WebUI integration)
	mux.HandleFunc("/api/v1/health", mcpServer.healthCheck)
	mux.HandleFunc("/api/v1/tools", mcpServer.listTools)
	mux.HandleFunc("/api/v1/tools/", mcpServer.handleToolCall)
	mux.HandleFunc("/api/v1/chat", mcpServer.handleChat)
	mux.HandleFunc("/api/v1/openapi.json", mcpServer.handleOpenAPI)
	
	// Legacy endpoints (for backward compatibility)
	mux.HandleFunc("/health", mcpServer.healthCheck)
	mux.HandleFunc("/metrics", mcpServer.metrics)
	mux.HandleFunc("/tools", mcpServer.listTools)
	mux.HandleFunc("/tool/", mcpServer.handleToolCall)
	mux.HandleFunc("/resources", mcpServer.listResources)

	mcpServer.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      mcpServer.withMiddleware(mux),
		ReadTimeout:  cfg.Server.Timeout,
		WriteTimeout: cfg.Server.Timeout,
		IdleTimeout:  120 * time.Second,
	}

	// Register all tools
	if err := mcpServer.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	// Register resources
	if err := mcpServer.registerResources(); err != nil {
		return nil, fmt.Errorf("failed to register resources: %w", err)
	}

	return mcpServer, nil
}

// Run starts the MCP server
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

	// Start MCP server in a goroutine
	go func() {
		mcpAddr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port+1)
		slog.Info("Starting MCP protocol server", "address", mcpAddr)
		if err := s.server.ListenAndServe(mcpAddr); err != nil {
			errChan <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		slog.Info("Shutting down MCP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		slog.Info("Shutting down HTTP server...")
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		}

		slog.Info("Shutting down MCP protocol server...")
		return s.server.Shutdown(context.Background())
	case err := <-errChan:
		slog.Error("Server error", "error", err)
		return err
	}
}

// healthCheck provides health status endpoint
func (s *MCPServer) healthCheck(w http.ResponseWriter, r *http.Request) {
	slog.Info("/health endpoint called", "method", r.Method, "remote_addr", r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")

	// Simple health check - could be enhanced to check ERPNext connectivity
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`)); err != nil {
		slog.Error("Failed to write health response", "error", err)
	}
	slog.Info("/health response sent")
}

// metrics provides basic metrics endpoint
func (s *MCPServer) metrics(w http.ResponseWriter, r *http.Request) {
	slog.Info("/metrics endpoint called", "method", r.Method, "remote_addr", r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")

	metrics := fmt.Sprintf(`{\n\t"uptime": "%s",\n\t"timestamp": "%s",\n\t"version": "1.0.0"\n}`, time.Since(time.Now()).String(), time.Now().Format(time.RFC3339))

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(metrics)); err != nil {
		slog.Error("Failed to write metrics response", "error", err)
	}
	slog.Info("/metrics response sent")
}

// withMiddleware applies middleware to HTTP handlers
func (s *MCPServer) withMiddleware(handler http.Handler) http.Handler {
	// Chain middleware: logging -> CORS -> auth -> handler
	h := handler
	
	// Apply auth middleware if enabled
	if s.authMiddleware != nil {
		h = s.authMiddleware.Handler(h)
	}
	
	// Apply CORS and logging
	h = s.corsMiddleware(h)
	h = s.loggingMiddleware(h)
	
	return h
}

// loggingMiddleware logs HTTP requests
func (s *MCPServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var reqBodyCopy, respBodyCopy strings.Builder

		// Read and log request body (if POST/PUT)
		if r.Method == "POST" || r.Method == "PUT" {
			bodyBytes, _ := io.ReadAll(r.Body)
			reqBodyCopy.Write(bodyBytes)
			r.Body = io.NopCloser(strings.NewReader(reqBodyCopy.String()))
			slog.Info("HTTP request received", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr, "user_agent", r.UserAgent(), "body", reqBodyCopy.String())
		} else {
			slog.Info("HTTP request received", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr, "user_agent", r.UserAgent())
		}

		// Wrap ResponseWriter to capture response body
		ww := &responseWriterWithBody{ResponseWriter: w, body: &respBodyCopy, status: 200}
		next.ServeHTTP(ww, r)

		slog.Info("HTTP request completed", "method", r.Method, "path", r.URL.Path, "status", ww.status, "duration", time.Since(start), "response_body", respBodyCopy.String())
	})
}

type responseWriterWithBody struct {
	http.ResponseWriter
	body   *strings.Builder
	status int
}

func (w *responseWriterWithBody) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriterWithBody) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// corsMiddleware handles CORS
func (s *MCPServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("CORS middleware invoked", "method", r.Method, "path", r.URL.Path)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			slog.Debug("CORS preflight response sent", "path", r.URL.Path)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// registerTools registers all MCP tools
func (s *MCPServer) registerTools() error {
	slog.Info("Registering MCP tools...")
	
	// Core CRUD tools (6 - Generic, work with ANY doctype)
	s.server.RegisterTool("get_document", s.tools.GetDocument)
	s.server.RegisterTool("list_documents", s.tools.ListDocuments)
	s.server.RegisterTool("create_document", s.tools.CreateDocument)
	s.server.RegisterTool("update_document", s.tools.UpdateDocument)
	s.server.RegisterTool("delete_document", s.tools.DeleteDocument)
	s.server.RegisterTool("search_documents", s.tools.SearchDocuments)
	
	// Aggregation and reporting tools (2 - For analytics and reports)
	s.server.RegisterTool("aggregate_documents", s.tools.AggregateDocuments)
	s.server.RegisterTool("run_report", s.tools.RunReport)

	// Generic analysis tool (1 - Replaces 9 doctype-specific tools!)
	s.server.RegisterTool("analyze_document", s.tools.AnalyzeDocument)

	// Legacy tools (kept for backward compatibility)
	// These will be deprecated in v2.0 - use analyze_document instead
	s.server.RegisterTool("get_project_status", s.tools.GetProjectStatus)
	s.server.RegisterTool("analyze_project_timeline", s.tools.AnalyzeProjectTimeline)
	s.server.RegisterTool("calculate_project_metrics", s.tools.CalculateProjectMetrics)
	s.server.RegisterTool("get_resource_allocation", s.tools.GetResourceAllocation)
	s.server.RegisterTool("project_risk_assessment", s.tools.ProjectRiskAssessment)
	s.server.RegisterTool("generate_project_report", s.tools.GenerateProjectReport)
	s.server.RegisterTool("portfolio_dashboard", s.tools.PortfolioDashboard)
	s.server.RegisterTool("resource_utilization_analysis", s.tools.ResourceUtilizationAnalysis)
	s.server.RegisterTool("budget_variance_analysis", s.tools.BudgetVarianceAnalysis)

	slog.Info("Registered MCP tools", "count", 18, "core", 9, "legacy", 9)
	return nil
}

// registerResources registers MCP resources
func (s *MCPServer) registerResources() error {
	slog.Info("Registering MCP resources...")
	// Register ERPNext doctypes as resources
	doctypes := []string{
		"Project",
		"Task",
		"Timesheet",
		"Sales Order",
		"Sales Invoice",
		"Purchase Order",
		"Item",
		"Customer",
		"Supplier",
		"Employee",
		"User",
	}

	for _, doctype := range doctypes {
		s.server.RegisterResource(fmt.Sprintf("erpnext://%s", doctype), fmt.Sprintf("ERPNext %s documents", doctype))
	}

	slog.Info("Registered MCP resources", "count", len(doctypes))
	return nil
}

// listTools provides HTTP endpoint for listing available tools
func (s *MCPServer) listTools(w http.ResponseWriter, r *http.Request) {
	slog.Info("/tools endpoint called", "method", r.Method, "remote_addr", r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")

	tools := []map[string]interface{}{
		// Core CRUD tools (Generic - work with ANY doctype)
		{
			"name":        "get_document",
			"description": "Retrieve a single ERPNext document by doctype and name",
		},
		{
			"name":        "list_documents",
			"description": "List ERPNext documents with optional filters and pagination",
		},
		{
			"name":        "create_document",
			"description": "Create a new ERPNext document of any doctype",
		},
		{
			"name":        "update_document",
			"description": "Update an existing ERPNext document",
		},
		{
			"name":        "delete_document",
			"description": "Delete an ERPNext document (with confirmation)",
		},
		{
			"name":        "search_documents",
			"description": "Search ERPNext documents using full-text search",
		},
		// Aggregation and reporting tools
		{
			"name":        "aggregate_documents",
			"description": "Perform aggregation queries (SUM, COUNT, AVG, GROUP BY, TOP N) on ERPNext data",
		},
		{
			"name":        "run_report",
			"description": "Execute Frappe/ERPNext reports (Sales Analytics, Purchase Register, etc.)",
		},
		// Generic analysis tool
		{
			"name":        "analyze_document",
			"description": "Analyze ANY ERPNext document (Project, Customer, Sales Order, custom doctypes, etc.)",
		},
		// Legacy tools (deprecated - use analyze_document instead)
		{
			"name":        "get_project_status",
			"description": "[DEPRECATED] Use analyze_document instead. Get comprehensive project status",
		},
		{
			"name":        "portfolio_dashboard",
			"description": "[DEPRECATED] Use analyze_document instead. Get portfolio-wide dashboard",
		},
		{
			"name":        "analyze_project_timeline",
			"description": "[DEPRECATED] Use analyze_document instead. Analyze project timeline",
		},
		{
			"name":        "calculate_project_metrics",
			"description": "[DEPRECATED] Use analyze_document instead. Calculate project metrics",
		},
		{
			"name":        "resource_utilization_analysis",
			"description": "[DEPRECATED] Use analyze_document instead. Analyze resource utilization",
		},
		{
			"name":        "budget_variance_analysis",
			"description": "[DEPRECATED] Use analyze_document instead. Analyze budget variance",
		},
	}

	response := map[string]interface{}{
		"tools": tools,
		"count": len(tools),
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode tools response", "error", err)
	}
	slog.Info("/tools response sent", "count", len(tools))
}

// handleToolCall handles direct tool calls via HTTP
func (s *MCPServer) handleToolCall(w http.ResponseWriter, r *http.Request) {
	slog.Info("Tool endpoint called", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr)
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		slog.Warn("Tool method not allowed", "method", r.Method, "path", r.URL.Path)
		return
	}

	// Support both /tool/ and /api/v1/tools/ prefixes
	toolName := strings.TrimPrefix(r.URL.Path, "/api/v1/tools/")
	if toolName == r.URL.Path {
		toolName = strings.TrimPrefix(r.URL.Path, "/tool/")
	}
	if toolName == "" || toolName == r.URL.Path {
		http.Error(w, "Tool name is required", http.StatusBadRequest)
		slog.Warn("Missing tool name", "path", r.URL.Path)
		return
	}

	var request mcp.ToolRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		slog.Error("/tool/ invalid request body", "error", err)
		return
	}

	slog.Info("Tool call received", "tool", toolName, "request_id", request.ID, "params", request.Params)
	request.Tool = toolName
	if request.ID == "" {
		request.ID = fmt.Sprintf("http-%d", time.Now().UnixNano())
	}

	ctx := r.Context()
	var result *mcp.ToolResponse
	var err error

	slog.Info("Executing tool", "tool", toolName, "request_id", request.ID, "params", request.Params)
	switch toolName {
	// Core CRUD tools
	case "get_document":
		slog.Info("Calling ERPNext GetDocument", "params", request.Params)
		result, err = s.tools.GetDocument(ctx, request)
	case "list_documents":
		slog.Info("Calling ERPNext ListDocuments", "params", request.Params)
		result, err = s.tools.ListDocuments(ctx, request)
	case "create_document":
		slog.Info("Calling ERPNext CreateDocument", "params", request.Params)
		result, err = s.tools.CreateDocument(ctx, request)
	case "update_document":
		slog.Info("Calling ERPNext UpdateDocument", "params", request.Params)
		result, err = s.tools.UpdateDocument(ctx, request)
	case "delete_document":
		slog.Info("Calling ERPNext DeleteDocument", "params", request.Params)
		result, err = s.tools.DeleteDocument(ctx, request)
	case "search_documents":
		slog.Info("Calling ERPNext SearchDocuments", "params", request.Params)
		result, err = s.tools.SearchDocuments(ctx, request)
	case "analyze_document":
		slog.Info("Calling ERPNext AnalyzeDocument", "params", request.Params)
		result, err = s.tools.AnalyzeDocument(ctx, request)
	case "get_project_status":
		slog.Info("Calling ERPNext GetProjectStatus", "params", request.Params)
		result, err = s.tools.GetProjectStatus(ctx, request)
	case "portfolio_dashboard":
		slog.Info("Calling ERPNext PortfolioDashboard", "params", request.Params)
		result, err = s.tools.PortfolioDashboard(ctx, request)
	case "analyze_project_timeline":
		slog.Info("Calling ERPNext AnalyzeProjectTimeline", "params", request.Params)
		result, err = s.tools.AnalyzeProjectTimeline(ctx, request)
	case "calculate_project_metrics":
		slog.Info("Calling ERPNext CalculateProjectMetrics", "params", request.Params)
		result, err = s.tools.CalculateProjectMetrics(ctx, request)
	case "resource_utilization_analysis":
		slog.Info("Calling ERPNext ResourceUtilizationAnalysis", "params", request.Params)
		result, err = s.tools.ResourceUtilizationAnalysis(ctx, request)
	case "budget_variance_analysis":
		slog.Info("Calling ERPNext BudgetVarianceAnalysis", "params", request.Params)
		result, err = s.tools.BudgetVarianceAnalysis(ctx, request)
	default:
		http.Error(w, "Tool not found", http.StatusNotFound)
		slog.Warn("Tool not found", "tool", toolName)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		slog.Error("Tool execution error", "tool", toolName, "error", err)
		return
	}

	slog.Info("Tool executed successfully", "tool", toolName, "request_id", request.ID, "erpnext_result", result)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		slog.Error("Failed to encode tool response", "error", err)
	}
	slog.Info("/tool/ response sent", "tool", toolName, "request_id", request.ID)
}

// listResources provides HTTP endpoint for listing available resources
func (s *MCPServer) listResources(w http.ResponseWriter, r *http.Request) {
	slog.Info("/resources endpoint called", "method", r.Method, "remote_addr", r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")

	resources := []map[string]interface{}{
		{
			"uri":         "erpnext://projects",
			"name":        "ERPNext Projects",
			"description": "Access to ERPNext project data",
		},
		{
			"uri":         "erpnext://tasks",
			"name":        "ERPNext Tasks",
			"description": "Access to ERPNext task data",
		},
		{
			"uri":         "erpnext://customers",
			"name":        "ERPNext Customers",
			"description": "Access to ERPNext customer data",
		},
		{
			"uri":         "erpnext://items",
			"name":        "ERPNext Items",
			"description": "Access to ERPNext item data",
		},
	}

	response := map[string]interface{}{
		"resources": resources,
		"count":     len(resources),
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode resources response", "error", err)
	}
	slog.Info("/resources response sent", "count", len(resources))
}

// handleChat handles natural language chat queries
func (s *MCPServer) handleChat(w http.ResponseWriter, r *http.Request) {
	slog.Info("/api/v1/chat endpoint called", "method", r.Method, "remote_addr", r.RemoteAddr)
	
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var chatRequest struct {
		Message string `json:"message"`
		Model   string `json:"model,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&chatRequest); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		slog.Error("Invalid chat request", "error", err)
		return
	}

	if chatRequest.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	slog.Info("Processing chat query", "message", chatRequest.Message)

	// Use AI to extract intent and entities from the query
	queryIntent, err := s.extractQueryIntent(r.Context(), chatRequest.Message)
	if err != nil {
		slog.Warn("Failed to extract intent with AI, falling back to simple routing", "error", err)
		queryIntent = s.fallbackQueryRouting(chatRequest.Message)
	}

	slog.Info("Query intent extracted", 
		"action", queryIntent.Action,
		"doctype", queryIntent.DocType,
		"entity_name", queryIntent.EntityName,
		"tool", queryIntent.Tool,
		"is_erpnext_related", queryIntent.IsERPNextRelated)

	var result *mcp.ToolResponse
	var toolsCalled []string
	ctx := r.Context()

	// Check if query is ERPNext-related
	if !queryIntent.IsERPNextRelated {
		slog.Info("Non-ERPNext query detected, providing polite decline")
		response := map[string]interface{}{
			"timestamp":      time.Now().Format(time.RFC3339),
			"tools_called":   []string{},
			"response":       "I'm an ERPNext assistant specialized in helping you with your business data (customers, invoices, projects, items, etc.). For general questions or other topics, please use a general-purpose AI assistant.",
			"is_valid_data":  false,
			"data_quality":   "not_applicable",
			"data_size":      0,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
		slog.Info("Chat response sent", "data_size", 0)
		return
	}

	// Special handling for create operations
	if queryIntent.Action == "create" {
		slog.Info("Processing create query", "doctype", queryIntent.DocType)
		params, err := s.extractCreateParams(ctx, chatRequest.Message, queryIntent.DocType)
		if err != nil {
			slog.Warn("Failed to extract create params", "error", err)
			err = fmt.Errorf("failed to process create query: %w", err)
		} else {
			toolsCalled = append(toolsCalled, "create_document")
			result, err = s.executeTool(ctx, "create_document", params)
		}
		// Skip to response building
		goto buildResponse
	}
	
	// Special handling for update operations
	if queryIntent.Action == "update" {
		slog.Info("Processing update query", "doctype", queryIntent.DocType, "entity", queryIntent.EntityName)
		params, err := s.extractUpdateParams(ctx, chatRequest.Message, queryIntent.DocType, queryIntent.EntityName)
		if err != nil {
			slog.Warn("Failed to extract update params", "error", err)
			err = fmt.Errorf("failed to process update query: %w", err)
		} else {
			toolsCalled = append(toolsCalled, "update_document")
			result, err = s.executeTool(ctx, "update_document", params)
		}
		// Skip to response building
		goto buildResponse
	}
	
	// Special handling for delete operations
	if queryIntent.Action == "delete" {
		slog.Info("Processing delete query", "doctype", queryIntent.DocType, "entity", queryIntent.EntityName)
		// Delete just needs doctype and name
		params := map[string]interface{}{
			"doctype": queryIntent.DocType,
			"name":    queryIntent.EntityName,
		}
		paramsJSON, _ := json.Marshal(params)
		toolsCalled = append(toolsCalled, "delete_document")
		result, err = s.executeTool(ctx, "delete_document", paramsJSON)
		// Skip to response building
		goto buildResponse
	}
	
	// Special handling for aggregate queries
	if queryIntent.Action == "aggregate" {
		slog.Info("Processing aggregation query")
		params, err := s.extractAggregationParams(ctx, chatRequest.Message, queryIntent.DocType)
		if err != nil {
			slog.Warn("Failed to extract aggregation params", "error", err)
			err = fmt.Errorf("failed to process aggregation query: %w", err)
		} else {
			toolsCalled = append(toolsCalled, "aggregate_documents")
			result, err = s.executeTool(ctx, "aggregate_documents", params)
		}
		// Skip to response building
		goto buildResponse
	}

	// Special handling for report queries
	if queryIntent.Action == "report" {
		slog.Info("Processing report query")
		params, err := s.extractReportParams(ctx, chatRequest.Message)
		if err != nil {
			slog.Warn("Failed to extract report params", "error", err)
			err = fmt.Errorf("failed to process report query: %w", err)
		} else {
			toolsCalled = append(toolsCalled, "run_report")
			result, err = s.executeTool(ctx, "run_report", params)
		}
		// Skip to response building
		goto buildResponse
	}

	// Check if entity_name looks like an exact ID (e.g., PROJ-0001, CUST-123)
	// If yes, use direct get_document instead of searching
	if queryIntent.EntityName != "" {
		isExactID := regexp.MustCompile(`^[A-Z]{3,5}-\d{4,6}$`).MatchString(queryIntent.EntityName)
		if isExactID {
			slog.Info("Entity is exact ID, skipping search and using direct get", "id", queryIntent.EntityName)
			queryIntent.RequiresSearch = false
			queryIntent.Tool = "get_document"
		}
	}
	
	// Handle the query based on extracted intent
	if queryIntent.EntityName != "" && queryIntent.RequiresSearch {
		// Multi-step: Search for the entity first, then execute the intended tool
		slog.Info("Multi-step query: searching for entity first", "entity", queryIntent.EntityName)
		
		// Step 1: Search for the entity
		searchResult, searchErr := s.searchForEntity(ctx, queryIntent.DocType, queryIntent.EntityName)
		if searchErr != nil {
			err = fmt.Errorf("failed to find %s '%s': %w", queryIntent.DocType, queryIntent.EntityName, searchErr)
		} else if searchResult.EntityName == "" {
			err = fmt.Errorf("no %s found matching '%s'", queryIntent.DocType, queryIntent.EntityName)
		} else {
			// Step 2: Execute the intended tool with the found entity
			toolsCalled = append(toolsCalled, "search_documents", queryIntent.Tool)
			result, err = s.executeToolWithEntity(ctx, queryIntent.Tool, queryIntent.DocType, searchResult.EntityName)
		}
	} else if queryIntent.EntityName != "" {
		// Single-step with entity: Use direct get_document
		slog.Info("Single-step query with entity", "entity", queryIntent.EntityName)
		toolsCalled = append(toolsCalled, queryIntent.Tool)
		result, err = s.executeToolWithEntity(ctx, queryIntent.Tool, queryIntent.DocType, queryIntent.EntityName)
	} else {
		// Single-step: Execute tool directly (list, etc.)
		toolsCalled = append(toolsCalled, queryIntent.Tool)
		result, err = s.executeTool(ctx, queryIntent.Tool, queryIntent.Params)
	}

buildResponse:
	// Build response in expected format
	response := map[string]interface{}{
		"timestamp":   time.Now().Format(time.RFC3339),
		"tools_called": toolsCalled,
	}

	if err != nil {
		response["response"] = fmt.Sprintf("Error processing query: %v", err)
		response["data_quality"] = "error"
		response["data_size"] = 0
		response["is_valid_data"] = false
		slog.Error("Chat query error", "error", err)
	} else if result != nil {
		// Extract text from MCP response
		var responseText strings.Builder
		for _, content := range result.Content {
			if content.Type == "text" {
				responseText.WriteString(content.Text)
				responseText.WriteString("\n")
			}
		}
		
		responseStr := responseText.String()
		
		// Use LLM to format the response nicely if available
		formattedResponse := responseStr
		if s.llmClient != nil {
			formatted, err := s.formatResponseWithLLM(ctx, chatRequest.Message, responseStr)
			if err != nil {
				slog.Warn("Failed to format response with LLM, using raw data", "error", err)
			} else {
				formattedResponse = formatted
			}
		}
		
		response["response"] = formattedResponse
		response["data_size"] = len(formattedResponse)
		response["is_valid_data"] = len(formattedResponse) > 0
		
		// Determine data quality based on response size
		if len(formattedResponse) > 1000 {
			response["data_quality"] = "high"
		} else if len(formattedResponse) > 100 {
			response["data_quality"] = "medium"
		} else {
			response["data_quality"] = "low"
		}
	} else {
		response["response"] = "I understand you're asking about ERPNext data. Please be more specific about what information you need."
		response["data_quality"] = "low"
		response["data_size"] = 0
		response["is_valid_data"] = false
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode chat response", "error", err)
	}
	slog.Info("Chat response sent", "data_size", response["data_size"])
}

// formatResponseWithLLM uses the LLM to format raw data into a user-friendly response
func (s *MCPServer) formatResponseWithLLM(ctx context.Context, userQuery string, rawData string) (string, error) {
	prompt := fmt.Sprintf(`You are a data formatter that converts JSON data into user-requested formats.

CRITICAL RULES:
1. NEVER MAKE UP OR INVENT DATA - Only format what's actually in the raw data
2. If raw data contains an error message, show it clearly to the user
3. If raw data is empty or has no results, say so explicitly
4. DO NOT add placeholder data like "Item 1", "Item 2", etc.
5. Only format the ACTUAL data provided below

User's Question: "%s"
Raw Data: %s

FORMAT DETECTION:
- "table format" / "as a table" / "in table" → Use markdown table with | separators
- "list" / "bullet points" → Use bullet points (-)
- "summary" / "summarize" → Provide a brief summary
- If no format specified → Use the most appropriate format

MARKDOWN TABLE FORMAT (when user asks for table):
| Column1 | Column2 | Column3 |
|---------|---------|---------|
| Value1  | Value2  | Value3  |

BULLET LIST FORMAT (when user asks for list):
- Item 1
- Item 2
- Item 3

EXAMPLES:

Example 1 - With Real Data:
User: "show companies in table format"
Data: {"data":[{"name":"VK","company_name":"VK Corp"}],"total_count":1}
Response:
Here are the companies in table format:

| Name | Company Name |
|------|--------------|
| VK   | VK Corp      |

Total: 1 company

Example 2 - Empty Data:
User: "list all customers"
Data: {"data":[],"total_count":0}
Response:
No customers found in the system.

Example 3 - Error in Data:
User: "show warehouses"
Data: Error processing query: doctype is required
Response:
I encountered an error: The query couldn't be processed because the document type is required. Please try rephrasing your question.

NOW FORMAT THE USER'S DATA:
- Look at the Raw Data above
- If it contains actual data, format it according to user's request
- If it's an error message, explain the error clearly
- If it's empty, say no results were found
- NEVER invent placeholder data`, userQuery, rawData)

	formatted, err := s.llmClient.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to call LLM for formatting: %w", err)
	}
	
	return formatted, nil
}

// extractAggregationParams uses LLM to extract aggregation parameters from query
func (s *MCPServer) extractAggregationParams(ctx context.Context, query string, doctype string) (json.RawMessage, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	prompt := fmt.Sprintf(`Extract aggregation parameters from this query.

Query: "%s"
DocType: "%s"

CRITICAL: You MUST use the provided DocType "%s" exactly as given. DO NOT change it or make up a new one.

Determine:
1. doctype: MUST be "%s" (the provided doctype - DO NOT CHANGE THIS)
2. fields: What fields to select/aggregate (e.g., ["customer", "SUM(grand_total) as total_revenue"])
3. group_by: Field to group by (e.g., "customer")
4. order_by: How to sort (e.g., "total_revenue desc")
5. limit: Top N results (e.g., 5)
6. filters: Any filters to apply (e.g., {"status": "Paid"})

Common patterns for Sales Invoice:
- "top 5 customers by revenue" → doctype="Sales Invoice", limit=5, group_by="customer", order_by="SUM(grand_total) desc", fields=["customer", "SUM(grand_total) as total_revenue"]
- "total sales by customer" → doctype="Sales Invoice", group_by="customer", fields=["customer", "SUM(grand_total) as total"]
- "which items sold most" → doctype="Sales Order Item", group_by="item_code", order_by="SUM(qty) desc", fields=["item_code", "SUM(qty) as quantity_sold"]

Respond with JSON only:
{
  "doctype": "%s",
  "fields": ["...", "..."],
  "group_by": "...",
  "order_by": "...",
  "limit": 5,
  "filters": {}
}`, query, doctype, doctype, doctype, doctype)

	response, err := s.llmClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to extract aggregation params: %w", err)
	}

	// Clean response: remove markdown code blocks if present
	cleanedResponse := strings.TrimSpace(response)
	cleanedResponse = strings.TrimPrefix(cleanedResponse, "```json")
	cleanedResponse = strings.TrimPrefix(cleanedResponse, "```")
	cleanedResponse = strings.TrimSuffix(cleanedResponse, "```")
	cleanedResponse = strings.TrimSpace(cleanedResponse)

	// Validate JSON
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(cleanedResponse), &params); err != nil {
		slog.Warn("Failed to parse aggregation params JSON", "response", cleanedResponse, "error", err)
		return nil, fmt.Errorf("invalid aggregation params JSON: %w", err)
	}

	// FORCE the doctype to be the one provided (prevent hallucination)
	if doctype != "" {
		params["doctype"] = doctype
	} else if params["doctype"] == nil || params["doctype"] == "" {
		return nil, fmt.Errorf("doctype is required for aggregation")
	}

	return json.Marshal(params)
}

// extractReportParams uses LLM to extract report parameters from query
func (s *MCPServer) extractReportParams(ctx context.Context, query string) (json.RawMessage, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	prompt := fmt.Sprintf(`Extract report parameters from this query.

Query: "%s"

Determine:
1. report_name: The exact ERPNext report name (e.g., "Sales Analytics", "Purchase Register", "Customer Ledger Summary")
2. filters: Any filters mentioned (e.g., {"company": "XYZ Corp", "from_date": "2024-01-01"})

Common ERPNext reports:
- Sales Analytics, Sales Register, Sales Order Analysis
- Purchase Register, Purchase Analytics
- Customer Ledger Summary, Supplier Ledger Summary
- Stock Balance, Stock Ledger
- Profit and Loss Statement, Balance Sheet
- General Ledger

Respond with JSON only:
{
  "report_name": "...",
  "filters": {}
}`, query)

	response, err := s.llmClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to extract report params: %w", err)
	}

	// Clean response: remove markdown code blocks if present
	cleanedResponse := strings.TrimSpace(response)
	cleanedResponse = strings.TrimPrefix(cleanedResponse, "```json")
	cleanedResponse = strings.TrimPrefix(cleanedResponse, "```")
	cleanedResponse = strings.TrimSuffix(cleanedResponse, "```")
	cleanedResponse = strings.TrimSpace(cleanedResponse)

	// Validate JSON
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(cleanedResponse), &params); err != nil {
		slog.Warn("Failed to parse report params JSON", "response", cleanedResponse, "error", err)
		return nil, fmt.Errorf("invalid report params JSON: %w", err)
	}

	return json.Marshal(params)
}

// extractCreateParams uses LLM to extract field values for document creation
func (s *MCPServer) extractCreateParams(ctx context.Context, query string, doctype string) (json.RawMessage, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	prompt := fmt.Sprintf(`Extract field values from this document creation query.

Query: "%s"
DocType: "%s"

CRITICAL: Extract ONLY the field values mentioned in the query. Do NOT invent or assume values not explicitly stated.

Common ERPNext DocType fields:
- Project: project_name (required), status, expected_start_date, expected_end_date, priority
- Customer: customer_name (required), customer_type, customer_group, territory
- Item: item_code (required), item_name, item_group, stock_uom
- Task: subject (required), project, status, priority, exp_start_date, exp_end_date
- User: email (required), first_name, last_name, enabled
- Company: company_name (required), abbr, default_currency

Extract values into a "data" object with field names as keys.

Examples:

Query: "create a project named Website Redesign"
Response: {
  "doctype": "Project",
  "data": {
    "project_name": "Website Redesign"
  }
}

Query: "add a new customer called Acme Corp with type Company"
Response: {
  "doctype": "Customer",
  "data": {
    "customer_name": "Acme Corp",
    "customer_type": "Company"
  }
}

Query: "create task Review PR with priority High for project PROJ-0001"
Response: {
  "doctype": "Task",
  "data": {
    "subject": "Review PR",
    "priority": "High",
    "project": "PROJ-0001"
  }
}

Respond with JSON only:
{
  "doctype": "%s",
  "data": {
    "field_name": "value"
  }
}`, query, doctype, doctype)

	response, err := s.llmClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to extract create params: %w", err)
	}

	// Clean response: remove markdown code blocks if present
	cleanedResponse := strings.TrimSpace(response)
	cleanedResponse = strings.TrimPrefix(cleanedResponse, "```json")
	cleanedResponse = strings.TrimPrefix(cleanedResponse, "```")
	cleanedResponse = strings.TrimSuffix(cleanedResponse, "```")
	cleanedResponse = strings.TrimSpace(cleanedResponse)

	// Validate JSON
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(cleanedResponse), &params); err != nil {
		slog.Warn("Failed to parse create params JSON", "response", cleanedResponse, "error", err)
		return nil, fmt.Errorf("invalid create params JSON: %w", err)
	}

	// Ensure doctype is set
	if params["doctype"] == nil || params["doctype"] == "" {
		params["doctype"] = doctype
	}

	// Ensure data field exists
	if params["data"] == nil {
		return nil, fmt.Errorf("no field data extracted from query")
	}

	return json.Marshal(params)
}

// extractUpdateParams uses LLM to extract field values for document updates
func (s *MCPServer) extractUpdateParams(ctx context.Context, query string, doctype string, entityName string) (json.RawMessage, error) {
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	prompt := fmt.Sprintf(`Extract field values from this document update query.

Query: "%s"
DocType: "%s"
Document Name: "%s"

CRITICAL: Extract ONLY the fields to be updated. Do NOT include the document name in the data.

Extract values into a "data" object with field names as keys.

Examples:

Query: "update project PROJ-0001 status to Completed"
Response: {
  "doctype": "Project",
  "name": "PROJ-0001",
  "data": {
    "status": "Completed"
  }
}

Query: "change task TASK-123 priority to High and status to Working"
Response: {
  "doctype": "Task",
  "name": "TASK-123",
  "data": {
    "priority": "High",
    "status": "Working"
  }
}

Query: "set customer CUST-001 territory to North America"
Response: {
  "doctype": "Customer",
  "name": "CUST-001",
  "data": {
    "territory": "North America"
  }
}

Respond with JSON only:
{
  "doctype": "%s",
  "name": "%s",
  "data": {
    "field_name": "new_value"
  }
}`, query, doctype, entityName, doctype, entityName)

	response, err := s.llmClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to extract update params: %w", err)
	}

	// Clean response: remove markdown code blocks if present
	cleanedResponse := strings.TrimSpace(response)
	cleanedResponse = strings.TrimPrefix(cleanedResponse, "```json")
	cleanedResponse = strings.TrimPrefix(cleanedResponse, "```")
	cleanedResponse = strings.TrimSuffix(cleanedResponse, "```")
	cleanedResponse = strings.TrimSpace(cleanedResponse)

	// Validate JSON
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(cleanedResponse), &params); err != nil {
		slog.Warn("Failed to parse update params JSON", "response", cleanedResponse, "error", err)
		return nil, fmt.Errorf("invalid update params JSON: %w", err)
	}

	// Ensure required fields are set
	if params["doctype"] == nil || params["doctype"] == "" {
		params["doctype"] = doctype
	}
	if params["name"] == nil || params["name"] == "" {
		params["name"] = entityName
	}
	if params["data"] == nil {
		return nil, fmt.Errorf("no field data extracted from query")
	}

	return json.Marshal(params)
}

// extractDoctypeFromQuery extracts ERPNext doctype from query using pattern matching
func extractDoctypeFromQuery(queryLower string) string {
	// Map of common terms to ERPNext doctypes
	doctypeMap := map[string]string{
		"user":          "User",
		"users":         "User",
		"customer":      "Customer",
		"customers":     "Customer",
		"company":       "Company",
		"companies":     "Company",
		"item":          "Item",
		"items":         "Item",
		"warehouse":     "Warehouse",
		"warehouses":    "Warehouse",
		"project":       "Project",
		"projects":      "Project",
		"task":          "Task",
		"tasks":         "Task",
		"sales order":   "Sales Order",
		"sales orders":  "Sales Order",
		"sales invoice": "Sales Invoice",
		"sales invoices": "Sales Invoice",
		"purchase order": "Purchase Order",
		"purchase orders": "Purchase Order",
		"supplier":      "Supplier",
		"suppliers":     "Supplier",
		"employee":      "Employee",
		"employees":     "Employee",
	}
	
	// Check for matches in the query
	for term, doctype := range doctypeMap {
		if strings.Contains(queryLower, term) {
			return doctype
		}
	}
	
	return "" // No match found
}

// extractQueryIntent uses AI to extract intent and entities from natural language query
func (s *MCPServer) extractQueryIntent(ctx context.Context, query string) (*QueryIntent, error) {
	// PREPROCESSING: Detect simple list queries before calling LLM
	// This prevents LLM from misclassifying simple "list" queries as "aggregate"
	queryLower := strings.ToLower(query)
	
	// Check for list keywords without aggregation keywords
	hasListKeyword := strings.Contains(queryLower, "list") || 
	                  strings.Contains(queryLower, "show all") || 
	                  strings.Contains(queryLower, "give all") ||
	                  strings.Contains(queryLower, "all ")
	
	hasAggregationKeyword := strings.Contains(queryLower, "top ") ||
	                         strings.Contains(queryLower, "bottom ") ||
	                         strings.Contains(queryLower, "sum") ||
	                         strings.Contains(queryLower, "total") ||
	                         strings.Contains(queryLower, "average") ||
	                         strings.Contains(queryLower, "count") ||
	                         strings.Contains(queryLower, "most") ||
	                         strings.Contains(queryLower, "highest") ||
	                         strings.Contains(queryLower, "lowest")
	
	// If query has "list" but NO aggregation keywords, it's definitely a list query
	if hasListKeyword && !hasAggregationKeyword {
		slog.Info("Preprocessing detected simple list query", "query", query)
		
		// Extract doctype from query using simple pattern matching
		doctype := extractDoctypeFromQuery(queryLower)
		if doctype == "" {
			doctype = "User" // Default fallback
		}
		
		params := map[string]interface{}{
			"doctype": doctype,
		}
		paramsJSON, _ := json.Marshal(params)
		
		return &QueryIntent{
			Action:           "list",
			DocType:          doctype,
			EntityName:       "",
			Tool:             "list_documents",
			Params:           paramsJSON,
			RequiresSearch:   false,
			IsERPNextRelated: true,
			Confidence:       0.95,
		}, nil
	}
	
	// Check if LLM client is available
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available - AI features disabled")
	}
	
	slog.Info("Using LLM provider", "provider", s.llmClient.Provider())
	
	// Construct prompt for AI to extract structured information
	prompt := fmt.Sprintf(`You are an ERPNext query parser. Extract structured information from the user's query and respond ONLY with valid JSON.

CRITICAL RULES (APPLY IN THIS ORDER):
1. If query contains the word "list", "all", "show all", "give list", "user list", etc. → action is ALWAYS "list"
2. If query asks for "top N", "bottom N", "most", "highest", "sum", "total", "average", "count" → action is "aggregate"
3. If query mentions running a "report" by name → action is "report"
4. If query mentions a SPECIFIC entity name/ID → action is "get"
5. "list" and "aggregate" actions ALWAYS have empty entity_name ""

SIMPLE RULE: If you see "list" or "all" in the query → IT IS A LIST ACTION, NOT AGGREGATE!

User Query: "%s"

Extract these fields:
1. is_erpnext_related: CRITICAL - Is this query about ERPNext business data?
   - true: Queries about customers, invoices, users, items, warehouses, reports, sales, etc.
   - false: General questions like "what are you?", "hello", "help", "who made you?", math problems, general knowledge
2. action: what the user wants to do (ONLY if is_erpnext_related is true)
   - "list": when asking for multiple/all documents (simple listing) - DEFAULT for "list", "all", "show all"
   - "aggregate": for queries with top/bottom N, sum, count, average, grouping, rankings (ONLY when explicit math/aggregation keywords)
   - "report": when asking to run a specific ERPNext report by name
   - "get": when asking for ONE specific document by name/ID
   - "search": when searching with criteria
   - "analyze": when asking for analysis/status/metrics of a specific document
   - "create/update/delete": for modifications
3. doctype: the ERPNext document type (User, Customer, Company, Project, Item, Task, Sales Order, Sales Invoice, Warehouse, etc.)
   - MUST be a valid ERPNext DocType
   - Map common terms: "user" → "User", "customer" → "Customer", "invoice" → "Sales Invoice", "warehouse" → "Warehouse", "item" → "Item"
   - NEVER use made-up doctypes like "QueryResponse" or "UserList"
   - Empty if is_erpnext_related is false
4. entity_name: THE EXACT entity name/ID (empty for list/aggregate/report/search!)
   - For aggregate/list/report queries → MUST be empty ""
   - For "get X named Y" or "X with ID Y" → extract Y
   - CRITICAL: If entity includes contextual words like "default", "current", "active", "primary" → use action="list" instead and leave entity_name empty
   - Examples of NON-ENTITIES: "default currency", "current company", "active user", "primary warehouse"
   - These should be list/search queries, NOT get queries
5. requires_search: true if entity_name is NOT an exact ID, false if it's an ID or empty

Respond with JSON only:
{
  "is_erpnext_related": true/false,
  "action": "...",
  "doctype": "...",
  "entity_name": "...",
  "requires_search": true/false,
  "confidence": 0.0-1.0
}

Examples:

Query: "what are you?"
Response: {"is_erpnext_related":false,"action":"","doctype":"","entity_name":"","requires_search":false,"confidence":0.95}

Query: "hello"
Response: {"is_erpnext_related":false,"action":"","doctype":"","entity_name":"","requires_search":false,"confidence":0.95}

Query: "help me"
Response: {"is_erpnext_related":false,"action":"","doctype":"","entity_name":"","requires_search":false,"confidence":0.95}

Query: "what is 2+2?"
Response: {"is_erpnext_related":false,"action":"","doctype":"","entity_name":"","requires_search":false,"confidence":0.95}

Query: "give user list"
Response: {"is_erpnext_related":true,"action":"list","doctype":"User","entity_name":"","requires_search":false,"confidence":0.95}

Query: "user list"
Response: {"is_erpnext_related":true,"action":"list","doctype":"User","entity_name":"","requires_search":false,"confidence":0.95}

Query: "give list of user"
Response: {"is_erpnext_related":true,"action":"list","doctype":"User","entity_name":"","requires_search":false,"confidence":0.95}

Query: "give the list of companies"
Response: {"is_erpnext_related":true,"action":"list","doctype":"Company","entity_name":"","requires_search":false,"confidence":0.95}

Query: "show all customers"
Response: {"is_erpnext_related":true,"action":"list","doctype":"Customer","entity_name":"","requires_search":false,"confidence":0.95}

Query: "give me warehouse list"
Response: {"is_erpnext_related":true,"action":"list","doctype":"Warehouse","entity_name":"","requires_search":false,"confidence":0.95}

Query: "list all items"
Response: {"is_erpnext_related":true,"action":"list","doctype":"Item","entity_name":"","requires_search":false,"confidence":0.95}

Query: "top 5 customers by revenue"
Response: {"is_erpnext_related":true,"action":"aggregate","doctype":"Sales Invoice","entity_name":"","requires_search":false,"confidence":0.95}

Query: "show me total sales by customer"
Response: {"is_erpnext_related":true,"action":"aggregate","doctype":"Sales Invoice","entity_name":"","requires_search":false,"confidence":0.9}

Query: "which items sold the most"
Response: {"is_erpnext_related":true,"action":"aggregate","doctype":"Sales Invoice","entity_name":"","requires_search":false,"confidence":0.9}

Query: "run Sales Analytics report"
Response: {"is_erpnext_related":true,"action":"report","doctype":"","entity_name":"","requires_search":false,"confidence":0.95}

Query: "get user john@example.com"
Response: {"is_erpnext_related":true,"action":"get","doctype":"User","entity_name":"john@example.com","requires_search":false,"confidence":0.9}

Query: "Show me details of project PROJ-0001"
Response: {"is_erpnext_related":true,"action":"get","doctype":"Project","entity_name":"PROJ-0001","requires_search":false,"confidence":0.95}

Query: "What's the status of project titled Website Redesign?"
Response: {"is_erpnext_related":true,"action":"get","doctype":"Project","entity_name":"Website Redesign","requires_search":true,"confidence":0.9}

Query: "create a project named Website Redesign"
Response: {"is_erpnext_related":true,"action":"create","doctype":"Project","entity_name":"","requires_search":false,"confidence":0.95}

Query: "add a new customer called Acme Corp"
Response: {"is_erpnext_related":true,"action":"create","doctype":"Customer","entity_name":"","requires_search":false,"confidence":0.9}

Query: "update project PROJ-0001 status to completed"
Response: {"is_erpnext_related":true,"action":"update","doctype":"Project","entity_name":"PROJ-0001","requires_search":false,"confidence":0.95}

Query: "delete customer CUST-00123"
Response: {"is_erpnext_related":true,"action":"delete","doctype":"Customer","entity_name":"CUST-00123","requires_search":false,"confidence":0.9}

Query: "what's the default currency?"
Response: {"is_erpnext_related":true,"action":"list","doctype":"Company","entity_name":"","requires_search":false,"confidence":0.9}

Query: "give details of the current company"
Response: {"is_erpnext_related":true,"action":"list","doctype":"Company","entity_name":"","requires_search":false,"confidence":0.9}

Query: "show me the active user"
Response: {"is_erpnext_related":true,"action":"list","doctype":"User","entity_name":"","requires_search":false,"confidence":0.9}

Query: "run accounts receivable report"
Response: {"is_erpnext_related":true,"action":"report","doctype":"","entity_name":"","requires_search":false,"confidence":0.95}

Now respond for the user's query:`, query)

	// Call LLM provider
	aiResponse, err := s.llmClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM: %w", err)
	}
	
	// Parse the AI's JSON response
	var aiExtraction struct {
		IsERPNextRelated bool    `json:"is_erpnext_related"`
		Action           string  `json:"action"`
		DocType          string  `json:"doctype"`
		EntityName       string  `json:"entity_name"`
		RequiresSearch   bool    `json:"requires_search"`
		Confidence       float64 `json:"confidence"`
	}
	
	if err := json.Unmarshal([]byte(aiResponse), &aiExtraction); err != nil {
		slog.Warn("Failed to parse AI response as JSON", "response", aiResponse, "error", err)
		return nil, fmt.Errorf("AI response was not valid JSON: %w", err)
	}
	
	// Check if query is ERPNext-related
	if !aiExtraction.IsERPNextRelated {
		slog.Info("Query is not ERPNext-related", "query", query)
		return &QueryIntent{
			Action:         "non_erpnext",
			IsERPNextRelated: false,
			Confidence:     aiExtraction.Confidence,
		}, nil
	}
	
	// Map action to actual MCP tool
	tool := s.mapActionToTool(aiExtraction.Action)
	
	// Create parameters
	params := map[string]interface{}{}
	if aiExtraction.DocType != "" {
		params["doctype"] = aiExtraction.DocType
	}
	if aiExtraction.EntityName != "" && !aiExtraction.RequiresSearch {
		params["name"] = aiExtraction.EntityName
	}
	paramsJSON, _ := json.Marshal(params)
	
	intent := &QueryIntent{
		Action:           aiExtraction.Action,
		DocType:          aiExtraction.DocType,
		EntityName:       aiExtraction.EntityName,
		Tool:             tool,
		Params:           paramsJSON,
		RequiresSearch:   aiExtraction.RequiresSearch,
		IsERPNextRelated: true,
		Confidence:       aiExtraction.Confidence,
	}
	
	slog.Info("AI extracted intent successfully", 
		"provider", s.llmClient.Provider(),
		"confidence", intent.Confidence,
		"tool", intent.Tool,
		"entity", intent.EntityName)
	
	return intent, nil
}

// mapActionToTool maps an action string to the appropriate MCP tool name
func (s *MCPServer) mapActionToTool(action string) string {
	action = strings.ToLower(action)
	
	// Map common actions to tools - now using generic analyze_document!
	actionMap := map[string]string{
		// All analysis actions → generic analyze_document (works with ANY doctype!)
		"get_status":         "analyze_document",
		"status":             "analyze_document",
		"analyze_timeline":   "analyze_document",
		"timeline":           "analyze_document",
		"get_metrics":        "analyze_document",
		"metrics":            "analyze_document",
		"risk":               "analyze_document",
		"risk_assessment":    "analyze_document",
		"generate_report":    "analyze_document",
		"analyze":            "analyze_document",
		"analysis":           "analyze_document",
		
		// Aggregation and reporting
		"aggregate":          "aggregate_documents",
		"report":             "run_report",
		
		// Dashboard/portfolio - can also use analyze_document for specific documents
		"portfolio":          "portfolio_dashboard", // Keep for now (lists all)
		"dashboard":          "portfolio_dashboard",
		
		// CRUD operations - generic by design
		"list":               "list_documents",
		"list_all":           "list_documents",
		"search":             "search_documents",
		"find":               "search_documents",
		"get_document":       "get_document",
		"get":                "get_document",
		"details":            "get_document",
		"create":             "create_document",
		"update":             "update_document",
		"delete":             "delete_document",
		
		// Legacy analytics - keep for backward compat
		"resource":           "resource_utilization_analysis",
		"resource_analysis":  "resource_utilization_analysis",
		"budget":             "budget_variance_analysis",
		"budget_analysis":    "budget_variance_analysis",
	}
	
	if tool, ok := actionMap[action]; ok {
		return tool
	}
	
	// Default to analyze_document for unknown actions (let AI figure it out)
	return "analyze_document"
}

// fallbackQueryRouting provides simple keyword-based routing when AI is unavailable
func (s *MCPServer) fallbackQueryRouting(query string) *QueryIntent {
	lowerQuery := strings.ToLower(query)
	
	intent := &QueryIntent{
		Confidence:     0.5, // Low confidence for fallback
		RequiresSearch: false,
	}
	
	// Extract entity name using the old regex method
	entityName := extractEntityName(query)
	intent.EntityName = entityName
	
	// Simple keyword matching
	if strings.Contains(lowerQuery, "portfolio") || strings.Contains(lowerQuery, "dashboard") {
		intent.Action = "dashboard"
		intent.Tool = "portfolio_dashboard"
		intent.Params = json.RawMessage(`{}`)
		return intent
	}
	
	if entityName != "" && strings.Contains(lowerQuery, "project") {
		intent.DocType = "Project"
		intent.RequiresSearch = true
		
		if strings.Contains(lowerQuery, "status") {
			intent.Action = "get_status"
			intent.Tool = "get_project_status"
		} else if strings.Contains(lowerQuery, "timeline") {
			intent.Action = "timeline"
			intent.Tool = "analyze_project_timeline"
		} else {
			intent.Action = "get_document"
			intent.Tool = "get_document"
		}
		return intent
	}
	
	// Default to list
	intent.Action = "list"
	intent.Tool = "list_documents"
	intent.DocType = "Project"
	params := map[string]interface{}{
		"doctype":   "Project",
		"page_size": 20,
	}
	paramsJSON, _ := json.Marshal(params)
	intent.Params = paramsJSON
	
	return intent
}

// searchForEntity searches ERPNext for an entity and returns the best match
func (s *MCPServer) searchForEntity(ctx context.Context, doctype, searchTerm string) (*SearchResult, error) {
	slog.Info("Searching for entity", "doctype", doctype, "search_term", searchTerm)
	
	// Use the ERPNext search API
	searchReq := types.SearchRequest{
		DocType:  doctype,
		Search:   searchTerm,
		PageSize: 5,
	}
	
	results, err := s.frappeClient.SearchDocuments(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	
	if len(results.Data) == 0 {
		return &SearchResult{}, nil // No results found
	}
	
	// Get the first (best) match
	firstResult := results.Data[0]
	
	// Extract the name field
	var entityName string
	if nameVal, ok := firstResult["name"]; ok {
		if nameStr, ok := nameVal.(string); ok {
			entityName = nameStr
		}
	}
	
	// Try alternative name fields
	if entityName == "" {
		for _, field := range []string{"title", "subject", "customer_name", "item_name", "employee_name"} {
			if val, ok := firstResult[field]; ok {
				if str, ok := val.(string); ok {
					entityName = str
					break
				}
			}
		}
	}
	
	slog.Info("Found entity", "name", entityName, "doctype", doctype)
	
	return &SearchResult{
		EntityName: entityName,
		DocType:    doctype,
		MatchScore: 1.0, // Could implement fuzzy matching score
	}, nil
}

// executeToolWithEntity executes a tool with a specific entity name
func (s *MCPServer) executeToolWithEntity(ctx context.Context, toolName, doctype, entityName string) (*mcp.ToolResponse, error) {
	var params map[string]interface{}
	
	// Build parameters based on the tool
	// New generic tools use doctype + name
	switch toolName {
	case "analyze_document", "get_document":
		params = map[string]interface{}{
			"doctype":         doctype,
			"name":            entityName,
			"include_related": true, // Fetch related docs for richer analysis
		}
	case "get_project_status", "analyze_project_timeline", "calculate_project_metrics", "project_risk_assessment", "generate_project_report":
		// Legacy project-specific tools
		params = map[string]interface{}{
			"project_name": entityName,
		}
	default:
		// Default generic approach
		params = map[string]interface{}{
			"doctype": doctype,
			"name":    entityName,
		}
	}
	
	paramsJSON, _ := json.Marshal(params)
	
	return s.executeTool(ctx, toolName, paramsJSON)
}

// executeTool executes a specific MCP tool
func (s *MCPServer) executeTool(ctx context.Context, toolName string, params json.RawMessage) (*mcp.ToolResponse, error) {
	request := mcp.ToolRequest{
		ID:     fmt.Sprintf("tool-%d", time.Now().UnixNano()),
		Tool:   toolName,
		Params: params,
	}
	
	slog.Info("Executing tool", "tool", toolName, "params", string(params))
	
	switch toolName {
	// Core generic tools (work with ANY doctype)
	case "get_document":
		return s.tools.GetDocument(ctx, request)
	case "list_documents":
		return s.tools.ListDocuments(ctx, request)
	case "create_document":
		return s.tools.CreateDocument(ctx, request)
	case "update_document":
		return s.tools.UpdateDocument(ctx, request)
	case "delete_document":
		return s.tools.DeleteDocument(ctx, request)
	case "search_documents":
		return s.tools.SearchDocuments(ctx, request)
	case "analyze_document":
		return s.tools.AnalyzeDocument(ctx, request)
	
	// Aggregation and reporting tools
	case "aggregate_documents":
		return s.tools.AggregateDocuments(ctx, request)
	case "run_report":
		return s.tools.RunReport(ctx, request)
		
	// Legacy tools (deprecated - use analyze_document instead)
	case "get_project_status":
		return s.tools.GetProjectStatus(ctx, request)
	case "portfolio_dashboard":
		return s.tools.PortfolioDashboard(ctx, request)
	case "analyze_project_timeline":
		return s.tools.AnalyzeProjectTimeline(ctx, request)
	case "calculate_project_metrics":
		return s.tools.CalculateProjectMetrics(ctx, request)
	case "resource_utilization_analysis":
		return s.tools.ResourceUtilizationAnalysis(ctx, request)
	case "budget_variance_analysis":
		return s.tools.BudgetVarianceAnalysis(ctx, request)
	case "project_risk_assessment":
		return s.tools.ProjectRiskAssessment(ctx, request)
	case "generate_project_report":
		return s.tools.GenerateProjectReport(ctx, request)
	case "get_resource_allocation":
		return s.tools.GetResourceAllocation(ctx, request)
	default:
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}
}

// extractEntityName extracts entity names from queries using multiple patterns
func extractEntityName(query string) string {
	// Pattern 1: Text in quotes "Entity Name" or 'Entity Name'
	reQuoted := regexp.MustCompile(`["']([^"']+)["']`)
	if matches := reQuoted.FindStringSubmatch(query); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	
	// Pattern 2: After keywords like "titled", "named", "called"
	keywords := []string{"titled", "named", "called", "title", "name"}
	lowerQuery := strings.ToLower(query)
	
	for _, keyword := range keywords {
		if idx := strings.Index(lowerQuery, keyword); idx != -1 {
			// Get text after the keyword
			after := query[idx+len(keyword):]
			after = strings.TrimSpace(after)
			
			// Remove common prefixes
			after = strings.TrimPrefix(after, ":")
			after = strings.TrimPrefix(after, "is")
			after = strings.TrimSpace(after)
			
			// Take until punctuation or end of string
			rePunctuation := regexp.MustCompile(`^([^,.?!]+)`)
			if matches := rePunctuation.FindStringSubmatch(after); len(matches) > 1 {
				result := strings.TrimSpace(matches[1])
				// Remove quotes if present
				result = strings.Trim(result, `"'`)
				if result != "" {
					return result
				}
			}
		}
	}
	
	// Pattern 3: After "of" or "for" in specific contexts
	// e.g., "status of Project XX" or "details for Customer ABC"
	reOfFor := regexp.MustCompile(`(?:of|for)\s+(?:Project|Customer|Item|Task|Employee)\s+([^,.?!]+)`)
	if matches := reOfFor.FindStringSubmatch(query); len(matches) > 1 {
		result := strings.TrimSpace(matches[1])
		result = strings.Trim(result, `"'`)
		return result
	}
	
	return ""
}

// handleOpenAPI provides OpenAPI specification
func (s *MCPServer) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	slog.Info("/api/v1/openapi.json endpoint called", "method", r.Method, "remote_addr", r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")

	spec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "ERPNext MCP Server API",
			"version":     "1.0.0",
			"description": "API for accessing ERPNext data through MCP protocol",
		},
		"servers": []map[string]string{
			{"url": fmt.Sprintf("http://%s:%d/api/v1", s.config.Server.Host, s.config.Server.Port)},
		},
		"paths": map[string]interface{}{
			"/health": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Health check",
					"description": "Check if the server is healthy",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Server is healthy",
						},
					},
				},
			},
			"/chat": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Natural language query",
					"description": "Send a natural language query about ERPNext data",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message": map[string]string{"type": "string"},
									},
									"required": []string{"message"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Query result",
						},
					},
				},
			},
			"/tools": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "List available tools",
					"description": "Get a list of all available MCP tools",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "List of tools",
						},
					},
				},
			},
		},
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(spec); err != nil {
		slog.Error("Failed to encode OpenAPI spec", "error", err)
	}
	slog.Info("OpenAPI spec sent")
}
