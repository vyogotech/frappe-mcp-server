// Package telemetry wires OpenTelemetry tracing for frappe-mcp-server.
//
// Init() is no-op unless OTEL_EXPORTER_OTLP_ENDPOINT is set. When unset, the
// global tracer provider stays at the default no-op provider, so calls to
// tracer.Start() throughout the codebase have effectively zero cost. When set,
// spans are exported via OTLP/HTTP using a batch processor.
package telemetry

import (
	"context"
	"log/slog"
	"os"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const serviceName = "frappe-mcp-server"

// noopShutdown is returned when telemetry is disabled.
func noopShutdown(_ context.Context) error { return nil }

// Init configures the global OpenTelemetry tracer provider.
//
// Returns a shutdown function the caller MUST defer; the function flushes any
// pending spans. If OTEL_EXPORTER_OTLP_ENDPOINT is unset, Init returns a no-op
// shutdown and leaves the global provider untouched.
//
// Init never returns an error that should abort startup. If exporter creation
// fails, Init logs a warning and returns a no-op shutdown so the server can
// still serve requests without telemetry.
func Init(ctx context.Context) (func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		slog.Info("OpenTelemetry disabled", "reason", "OTEL_EXPORTER_OTLP_ENDPOINT not set")
		return noopShutdown, nil
	}

	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		slog.Warn("OpenTelemetry exporter init failed; tracing disabled", "error", err)
		return noopShutdown, nil
	}

	version := "unknown"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		version = info.Main.Version
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("service.version", version),
		),
	)
	if err != nil {
		slog.Warn("OpenTelemetry resource init failed; tracing disabled", "error", err)
		return noopShutdown, nil
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)

	slog.Info("OpenTelemetry enabled", "endpoint", endpoint)

	return func(shutdownCtx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(shutdownCtx, 5*time.Second)
		defer cancel()
		return provider.Shutdown(shutdownCtx)
	}, nil
}
