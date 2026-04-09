package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestInit_NoEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := Init(context.Background())

	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Shutdown must not panic and must return quickly when no exporter is wired up.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	assert.NoError(t, shutdown(shutdownCtx))
}

func TestInit_ValidEndpoint(t *testing.T) {
	// Set endpoint to a non-routable address. otlptracehttp constructs the
	// exporter lazily — it does not dial during New(), so this succeeds and
	// exercises the "real provider installed" branch.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318")
	t.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })

	shutdown, err := Init(context.Background())

	require.NoError(t, err)
	require.NotNil(t, shutdown)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	assert.NoError(t, shutdown(shutdownCtx))
}

func TestInit_ShutdownTimeout(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318")
	t.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })

	shutdown, err := Init(context.Background())
	require.NoError(t, err)

	// Cancelled context: shutdown must return without panic.
	shutdownCtx, cancel := context.WithCancel(context.Background())
	cancel()
	// With a cancelled context, shutdown may return a context error; we only
	// verify that it returns without panicking.
	_ = shutdown(shutdownCtx)
}
