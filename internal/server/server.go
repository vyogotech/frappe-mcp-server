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

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/llm"
	"frappe-mcp-server/internal/mcp"
	"frappe-mcp-server/internal/tools"
	"frappe-mcp-server/internal/types"
)

// MCPServer represents the MCP server
type MCPServer struct {
	config       *config.Config
	frappeClient *frappe.Client
	llmClient    llm.Client
	server       *mcp.Server
	httpServer   *http.Server
	tools        *tools.ToolRegistry
}

// QueryIntent represents the extracted intent from a natural language query
type QueryIntent struct {
	Action         string                 // The intended action (get, list, search, analyze, etc.)
	DocType        string                 // ERPNext doctype (Project, Customer, Item, etc.)
	EntityName     string                 // Specific entity name if querying a specific document
	Tool           string                 // The MCP tool to call
	Params         json.RawMessage        // Parameters for the tool
	RequiresSearch bool                   // Whether we need to search for the entity first
	Confidence     float64                // AI confidence in the extraction (0-1)
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
	return s.loggingMiddleware(s.corsMiddleware(handler))
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

	slog.Info("Registered MCP tools", "count", 16, "core", 7, "legacy", 9)
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
		"tool", queryIntent.Tool)

	var result *mcp.ToolResponse
	var toolsCalled []string
	ctx := r.Context()

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
		response["response"] = responseStr
		response["data_size"] = len(responseStr)
		response["is_valid_data"] = len(responseStr) > 0
		
		// Determine data quality based on response size
		if len(responseStr) > 1000 {
			response["data_quality"] = "high"
		} else if len(responseStr) > 100 {
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

// extractQueryIntent uses AI to extract intent and entities from natural language query
func (s *MCPServer) extractQueryIntent(ctx context.Context, query string) (*QueryIntent, error) {
	// Check if LLM client is available
	if s.llmClient == nil {
		return nil, fmt.Errorf("LLM client not available - AI features disabled")
	}
	
	slog.Info("Using LLM provider", "provider", s.llmClient.Provider())
	
	// Construct prompt for AI to extract structured information
	prompt := fmt.Sprintf(`You are an ERPNext query parser. Extract structured information from the user's query and respond ONLY with valid JSON.

CRITICAL: Extract the exact entity name/ID mentioned in the query!

User Query: "%s"

Extract these fields:
1. action: what the user wants to do (get, list, search, analyze, create, update, delete)
2. doctype: the ERPNext document type (Project, Customer, Item, Task, etc.)
3. entity_name: THE EXACT entity name, ID, or title from the query
   - If query mentions "PROJ-0001", extract "PROJ-0001"
   - If query mentions "project titled Website", extract "Website"
   - If query says "all projects" or "list projects", use empty string ""
   - LOOK FOR: IDs (PROJ-0001, CUST-123), names after "called/named/titled", quoted names
4. requires_search: true if entity_name is NOT an exact ID, false if it's an ID or empty

Respond with JSON only:
{
  "action": "...",
  "doctype": "...",
  "entity_name": "...",
  "requires_search": true/false,
  "confidence": 0.0-1.0
}

Examples:
Query: "Show me details of project PROJ-0001"
Response: {"action":"get","doctype":"Project","entity_name":"PROJ-0001","requires_search":false,"confidence":0.95}

Query: "What's the status of project titled Website Redesign?"
Response: {"action":"get","doctype":"Project","entity_name":"Website Redesign","requires_search":true,"confidence":0.9}

Query: "Get project PROJ-0001"
Response: {"action":"get","doctype":"Project","entity_name":"PROJ-0001","requires_search":false,"confidence":0.95}

Query: "List all projects"
Response: {"action":"list","doctype":"Project","entity_name":"","requires_search":false,"confidence":0.9}

Query: "Show me Customer ABC Corp details"
Response: {"action":"get","doctype":"Customer","entity_name":"ABC Corp","requires_search":true,"confidence":0.85}

Query: "How many projects are there?"
Response: {"action":"list","doctype":"Project","entity_name":"","requires_search":false,"confidence":0.8}

Now respond for the user's query:`, query)

	// Call LLM provider
	aiResponse, err := s.llmClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM: %w", err)
	}
	
	// Parse the AI's JSON response
	var aiExtraction struct {
		Action         string  `json:"action"`
		DocType        string  `json:"doctype"`
		EntityName     string  `json:"entity_name"`
		RequiresSearch bool    `json:"requires_search"`
		Confidence     float64 `json:"confidence"`
	}
	
	if err := json.Unmarshal([]byte(aiResponse), &aiExtraction); err != nil {
		slog.Warn("Failed to parse AI response as JSON", "response", aiResponse, "error", err)
		return nil, fmt.Errorf("AI response was not valid JSON: %w", err)
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
		Action:         aiExtraction.Action,
		DocType:        aiExtraction.DocType,
		EntityName:     aiExtraction.EntityName,
		Tool:           tool,
		Params:         paramsJSON,
		RequiresSearch: aiExtraction.RequiresSearch,
		Confidence:     aiExtraction.Confidence,
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
		"report":             "analyze_document",
		"analyze":            "analyze_document",
		"analysis":           "analyze_document",
		
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
