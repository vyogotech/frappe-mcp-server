package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/server"
	"frappe-mcp-server/internal/telemetry"
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

	// Initialize OpenTelemetry tracing (no-op unless OTEL_EXPORTER_OTLP_ENDPOINT is set)
	telemetryShutdown, err := telemetry.Init(context.Background())
	if err != nil {
		slog.Warn("Telemetry init returned error; continuing without tracing", "error", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := telemetryShutdown(shutdownCtx); shutdownErr != nil {
			slog.Warn("Telemetry shutdown failed", "error", shutdownErr)
		}
	}()

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
