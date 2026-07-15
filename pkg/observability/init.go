// Package observability provides OpenTelemetry integration for the agent platform
package observability

import (
	"context"
	"fmt"
	"log"
	"os"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitServiceTracing initializes OTel tracing for a service.
// It reads OTEL_EXPORTER_OTLP_ENDPOINT from the environment.
// If not set, it uses a no-op tracer provider (tracing disabled).
// This is a convenience function for service main.go files.
func InitServiceTracing(serviceName string) (*sdktrace.TracerProvider, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		// No OTel collector configured — use no-op provider
		log.Printf("[OTel] No OTEL_EXPORTER_OTLP_ENDPOINT set, tracing disabled for %s", serviceName)
		return nil, nil
	}

	log.Printf("[OTel] Initializing tracing for %s → %s", serviceName, endpoint)

	cfg := &OTelConfig{
		ServiceName:       serviceName,
		ServiceVersion:    "1.0.0",
		Environment:       os.Getenv("ENVIRONMENT"),
		TraceEnabled:      true,
		TraceExporterType: "otlp-grpc",
		TraceEndpoint:     endpoint,
		TraceSampleRate:   1.0,
		TraceBatchTimeout: 5000,
		MetricsEnabled:    false, // Metrics via Prometheus separately
	}

	if err := InitGlobal(context.Background(), cfg); err != nil {
		return nil, fmt.Errorf("failed to init OTel for %s: %w", serviceName, err)
	}

	mgr := GetGlobal()
	if mgr != nil && mgr.tracerProvider != nil {
		return mgr.tracerProvider, nil
	}

	return nil, nil
}

// ShutdownServiceTracing shuts down the global OTel manager gracefully.
func ShutdownServiceTracing() {
	ctx := context.Background()
	if err := ShutdownGlobal(ctx); err != nil {
		log.Printf("[OTel] Shutdown error: %v", err)
	}
}

// noopTracerProvider is created lazily when needed.
var noopTracerProvider = sdktrace.NewTracerProvider()
