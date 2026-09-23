package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	gosdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"frappe-mcp-server/internal/buildinfo"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/mcp"
	"frappe-mcp-server/internal/tools"
)

func main() {
	// Parse command line flags.
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Setup logging to stderr so it does not interfere with stdio communication.
	log.SetOutput(os.Stderr)

	// Only a --config given on the command line names a file that must exist; without one the environment may carry
	// every setting.
	given := false
	flag.Visit(func(f *flag.Flag) { given = given || f.Name == "config" })
	if os.Getenv("CONFIG_FILE") == "" && given {
		_ = os.Setenv("CONFIG_FILE", *configPath)
	}

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create Frappe client.
	frappeClient, err := frappe.NewClient(cfg.ERPNext)
	if err != nil {
		log.Fatalf("Failed to create Frappe client: %v", err)
	}

	// Create MCP server (backed by go-sdk).
	mcpServer := mcp.NewServer("frappe-mcp-server", buildinfo.Version())

	// Create tool registry and register all tools.
	toolRegistry := tools.NewRegistry(frappeClient)
	mcpServer.RegisterTool("get_document", toolRegistry.GetDocument)
	mcpServer.RegisterTool("list_documents", toolRegistry.ListDocuments)
	mcpServer.RegisterTool("create_document", toolRegistry.CreateDocument)
	mcpServer.RegisterTool("update_document", toolRegistry.UpdateDocument)
	mcpServer.RegisterTool("delete_document", toolRegistry.DeleteDocument)
	mcpServer.RegisterTool("search_documents", toolRegistry.SearchDocuments)
	mcpServer.RegisterTool("analyze_document", toolRegistry.AnalyzeDocument)
	// Legacy tools kept for backward compatibility.
	mcpServer.RegisterTool("get_project_status", toolRegistry.GetProjectStatus)
	mcpServer.RegisterTool("analyze_project_timeline", toolRegistry.AnalyzeProjectTimeline)
	mcpServer.RegisterTool("get_resource_allocation", toolRegistry.GetResourceAllocation)
	mcpServer.RegisterTool("generate_project_report", toolRegistry.GenerateProjectReport)
	mcpServer.RegisterTool("resource_utilization_analysis", toolRegistry.ResourceUtilizationAnalysis)
	mcpServer.RegisterTool("budget_variance_analysis", toolRegistry.BudgetVarianceAnalysis)

	// Handle graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Run with the go-sdk stdio transport.
	log.Printf("Starting ERPNext MCP stdio server...")
	if err := mcpServer.Run(ctx, &gosdk.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
