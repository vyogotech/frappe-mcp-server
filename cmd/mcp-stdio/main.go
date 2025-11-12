package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/mcp"
	"frappe-mcp-server/internal/tools"
)

// MCPMessage represents an MCP protocol message
type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPServer represents the stdio MCP server
type MCPStdioServer struct {
	server  *mcp.Server
	tools   *tools.ToolRegistry
	scanner *bufio.Scanner
	encoder *json.Encoder
}

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Setup logging to stderr so it doesn't interfere with stdio communication
	log.SetOutput(os.Stderr)

	// Set config file environment variable if not already set
	if os.Getenv("CONFIG_FILE") == "" && *configPath != "" {
		os.Setenv("CONFIG_FILE", *configPath)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create Frappe client
	frappeClient, err := frappe.NewClient(cfg.ERPNext)
	if err != nil {
		log.Fatalf("Failed to create Frappe client: %v", err)
	}

	// Create MCP server
	server := mcp.NewServer("frappe-mcp-server", "1.0.0")

	// Create tool registry
	toolRegistry := tools.NewRegistry(frappeClient)

	// Create stdio server
	stdioServer := &MCPStdioServer{
		server:  server,
		tools:   toolRegistry,
		scanner: bufio.NewScanner(os.Stdin),
		encoder: json.NewEncoder(os.Stdout),
	}

	// Register tools
	if err := stdioServer.registerTools(); err != nil {
		log.Fatalf("Failed to register tools: %v", err)
	}

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		cancel()
	}()

	// Start stdio server
	log.Printf("Starting ERPNext MCP stdio server...")
	if err := stdioServer.Run(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// Run starts the stdio MCP server
func (s *MCPStdioServer) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if !s.scanner.Scan() {
				if err := s.scanner.Err(); err != nil {
					return fmt.Errorf("scanner error: %w", err)
				}
				return nil // EOF
			}

		line := s.scanner.Text()
		if line == "" {
			continue
		}

		if err := s.handleMessage(line); err != nil {
			log.Printf("Error handling message: %v", err)
			// Try to extract ID from the message for proper error response
			var partialMsg struct {
				ID interface{} `json:"id"`
			}
			_ = json.Unmarshal([]byte(line), &partialMsg)
			
			// Use a default ID if we couldn't extract one
			msgID := partialMsg.ID
			if msgID == nil {
				msgID = "unknown"
			}
			
			// Send error response with ID
			errorResp := MCPMessage{
				JSONRPC: "2.0",
				ID:      msgID,
				Error: &MCPError{
					Code:    -32603,
					Message: "Internal error",
					Data:    err.Error(),
				},
			}
			_ = s.encoder.Encode(errorResp)
		}
		}
	}
}

// handleMessage processes incoming MCP messages
func (s *MCPStdioServer) handleMessage(message string) error {
	var msg MCPMessage
	if err := json.Unmarshal([]byte(message), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "initialized":
		// Notification - no response needed
		log.Printf("Client initialized")
		return nil
	case "tools/list":
		return s.handleToolsList(msg)
	case "tools/call":
		return s.handleToolCall(msg)
	case "resources/list":
		return s.handleResourcesList(msg)
	case "ping":
		// Handle ping/heartbeat
		return s.sendPingResponse(msg)
	default:
		// For notifications (no ID), don't send error response
		if msg.ID == nil {
			log.Printf("Ignoring unknown notification: %s", msg.Method)
			return nil
		}
		return s.sendErrorResponse(msg.ID, -32601, "Method not found")
	}
}

// handleInitialize handles the initialize method
func (s *MCPStdioServer) handleInitialize(msg MCPMessage) error {
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{},
				"resources": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "frappe-mcp-server",
				"version": "1.0.0",
			},
		},
	}
	return s.encoder.Encode(response)
}

// handleToolsList handles the tools/list method
func (s *MCPStdioServer) handleToolsList(msg MCPMessage) error {
	tools := []map[string]interface{}{
		{
			"name":        "get_document",
			"description": "Retrieve a single ERPNext document by doctype and name",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"doctype": map[string]interface{}{
						"type":        "string",
						"description": "The ERPNext document type (e.g., Project, Task, Customer)",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "The document name/ID",
					},
				},
				"required": []string{"doctype", "name"},
			},
		},
		{
			"name":        "list_documents",
			"description": "List ERPNext documents with optional filters and pagination",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"doctype": map[string]interface{}{
						"type":        "string",
						"description": "The ERPNext document type",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of documents to return",
						"default":     20,
					},
					"filters": map[string]interface{}{
						"type":        "object",
						"description": "Optional filters to apply",
					},
				},
				"required": []string{"doctype"},
			},
		},
		{
			"name":        "search_documents",
			"description": "Search ERPNext documents using full-text search",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"doctype": map[string]interface{}{
						"type":        "string",
						"description": "The ERPNext document type to search",
					},
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query string",
					},
					"limit": map[string]interface{}{
						"type":    "number",
						"default": 10,
					},
				},
				"required": []string{"doctype", "query"},
			},
		},
		{
			"name":        "get_project_status",
			"description": "Get comprehensive project status including tasks, timeline, and metrics",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the project",
					},
				},
				"required": []string{"project_name"},
			},
		},
		{
			"name":        "portfolio_dashboard",
			"description": "Get portfolio-wide dashboard with project overview and metrics",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status_filter": map[string]interface{}{
						"type":        "string",
						"description": "Filter projects by status (Open, Completed, etc.)",
					},
				},
			},
		},
		{
			"name":        "analyze_project_timeline",
			"description": "Analyze project timeline and identify potential delays or issues",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the project to analyze",
					},
				},
				"required": []string{"project_name"},
			},
		},
		{
			"name":        "calculate_project_metrics",
			"description": "Calculate key project metrics including completion rate, budget utilization, etc.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the project",
					},
				},
				"required": []string{"project_name"},
			},
		},
		{
			"name":        "resource_utilization_analysis",
			"description": "Analyze resource utilization across projects",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"period": map[string]interface{}{
						"type":        "string",
						"description": "Analysis period (e.g., 'last_month', 'current_quarter')",
						"default":     "current_month",
					},
				},
			},
		},
		{
			"name":        "budget_variance_analysis",
			"description": "Analyze budget variance across projects",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"period": map[string]interface{}{
						"type":        "string",
						"description": "Analysis period",
						"default":     "current_quarter",
					},
				},
			},
		},
	}

	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
	return s.encoder.Encode(response)
}

// handleToolCall handles tool execution
func (s *MCPStdioServer) handleToolCall(msg MCPMessage) error {
	params, ok := msg.Params.(map[string]interface{})
	if !ok {
		return s.sendErrorResponse(msg.ID, -32602, "Invalid params")
	}

	toolName, ok := params["name"].(string)
	if !ok {
		return s.sendErrorResponse(msg.ID, -32602, "Tool name required")
	}

	arguments, ok := params["arguments"].(map[string]interface{})
	if !ok {
		arguments = make(map[string]interface{})
	}

	// Convert to tool request format
	argsJSON, err := json.Marshal(arguments)
	if err != nil {
		return s.sendErrorResponse(msg.ID, -32603, "Failed to marshal arguments")
	}

	toolRequest := mcp.ToolRequest{
		ID:     fmt.Sprintf("%v", msg.ID),
		Tool:   toolName,
		Params: argsJSON,
	}

	// Execute tool
	ctx := context.Background()
	var result *mcp.ToolResponse
	var execErr error

	switch toolName {
	case "get_document":
		result, execErr = s.tools.GetDocument(ctx, toolRequest)
	case "list_documents":
		result, execErr = s.tools.ListDocuments(ctx, toolRequest)
	case "search_documents":
		result, execErr = s.tools.SearchDocuments(ctx, toolRequest)
	case "get_project_status":
		result, execErr = s.tools.GetProjectStatus(ctx, toolRequest)
	case "portfolio_dashboard":
		result, execErr = s.tools.PortfolioDashboard(ctx, toolRequest)
	case "analyze_project_timeline":
		result, execErr = s.tools.AnalyzeProjectTimeline(ctx, toolRequest)
	case "calculate_project_metrics":
		result, execErr = s.tools.CalculateProjectMetrics(ctx, toolRequest)
	case "resource_utilization_analysis":
		result, execErr = s.tools.ResourceUtilizationAnalysis(ctx, toolRequest)
	case "budget_variance_analysis":
		result, execErr = s.tools.BudgetVarianceAnalysis(ctx, toolRequest)
	default:
		return s.sendErrorResponse(msg.ID, -32601, "Tool not found")
	}

	if execErr != nil {
		return s.sendErrorResponse(msg.ID, -32603, execErr.Error())
	}

	// Convert tool response to MCP format
	mcpResult := map[string]interface{}{
		"content": result.Content,
	}

	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  mcpResult,
	}
	return s.encoder.Encode(response)
}

// handleResourcesList handles the resources/list method
func (s *MCPStdioServer) handleResourcesList(msg MCPMessage) error {
	resources := []map[string]interface{}{
		{
			"uri":         "erpnext://projects",
			"name":        "ERPNext Projects",
			"description": "Access to ERPNext project data",
			"mimeType":    "application/json",
		},
		{
			"uri":         "erpnext://tasks",
			"name":        "ERPNext Tasks",
			"description": "Access to ERPNext task data",
			"mimeType":    "application/json",
		},
		{
			"uri":         "erpnext://customers",
			"name":        "ERPNext Customers",
			"description": "Access to ERPNext customer data",
			"mimeType":    "application/json",
		},
	}

	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"resources": resources,
		},
	}
	return s.encoder.Encode(response)
}

// sendErrorResponse sends an error response
func (s *MCPStdioServer) sendErrorResponse(id interface{}, code int, message string) error {
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}
	return s.encoder.Encode(response)
}

// sendPingResponse sends a pong response
func (s *MCPStdioServer) sendPingResponse(msg MCPMessage) error {
	response := MCPMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"status": "ok",
		},
	}
	return s.encoder.Encode(response)
}

// registerTools registers all available tools
func (s *MCPStdioServer) registerTools() error {
	// Tools are registered by listing them in handleToolsList
	// and implementing them in handleToolCall
	log.Printf("ERPNext MCP tools registered successfully")
	return nil
}
