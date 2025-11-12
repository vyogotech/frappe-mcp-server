package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/server"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create Frappe client
	frappeClient, err := frappe.NewClient(cfg.ERPNext)
	if err != nil {
		slog.Error("Failed to create ERPNext client", "error", err)
		os.Exit(1)
	}

	// Create MCP server
	mcpServer, err := server.NewMCPServer(cfg, frappeClient)
	if err != nil {
		slog.Error("Failed to create MCP server", "error", err)
		os.Exit(1)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		slog.Info("Received shutdown signal, gracefully shutting down...")
		cancel()
	}()

	// Start server
	slog.Info("Starting ERPNext MCP Server", "version", "1.0.0")
	if err := mcpServer.Run(ctx); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}

	slog.Info("Server shutdown complete")
}
