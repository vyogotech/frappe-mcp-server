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

	// Create tool registry and register the same catalogue the HTTP server publishes.
	toolRegistry := tools.NewRegistry(frappeClient)
	if err := tools.Register(mcpServer, toolRegistry.Catalog(cfg.Tools.KnowledgeBase)); err != nil {
		log.Fatalf("Failed to register tools: %v", err)
	}

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
