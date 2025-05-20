// Package observability provides unified observability functionality for the MCP system.
// It consolidates logging, metrics, and tracing into a cohesive interface.
package observability

import (
	"context"
	"go.opentelemetry.io/otel/attribute"
)

// Global default instances - these are the main entry points for the package users
var (
	// DefaultLogger is the default logger instance
	DefaultLogger Logger

	// DefaultMetricsClient is the default metrics client instance
	DefaultMetricsClient MetricsClient

	// DefaultStartSpan is the default function for starting new spans
	DefaultStartSpan StartSpanFunc
)

// Initialize initializes the observability system with the given configuration.
// This is the main entry point for configuring all observability components.
func Initialize(cfg Config) error {
	// Set up default logger if not already set
	if DefaultLogger == nil {
		DefaultLogger = NewStandardLogger("devops-mcp")
	}

	// Set up default metrics client if not already set
	if DefaultMetricsClient == nil {
		DefaultMetricsClient = NewMetricsClient()
	}

	// Set up default tracing if not already set
	if DefaultStartSpan == nil {
		if cfg.Tracing.Enabled {
			shutdownFunc, err := InitTracing(cfg.Tracing)
			if err != nil {
				DefaultLogger.Error("Failed to initialize tracing", map[string]interface{}{"error": err.Error()})
				// Fall back to no-op tracing
				DefaultStartSpan = NoopStartSpan
			} else {
				// Create a wrapper to adapt the StartSpan function to match the StartSpanFunc signature
				DefaultStartSpan = func(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, Span) {
					returnCtx, returnSpan := StartSpan(ctx, name)
					if len(attrs) > 0 {
						returnSpan.SetAttribute("attributes", attrs)
					}
					return returnCtx, returnSpan
				}
				
				// TODO: Store shutdown function to be called during Shutdown()
				_ = shutdownFunc
			}
		} else {
			// Tracing not enabled, use no-op implementation
			DefaultStartSpan = NoopStartSpan
		}
	}

	return nil
}

// Shutdown gracefully shuts down all observability components
func ObservabilityShutdown() error {
	// Close metrics client if it exists
	if DefaultMetricsClient != nil {
		if err := DefaultMetricsClient.Close(); err != nil {
			if DefaultLogger != nil {
				DefaultLogger.Error("Error shutting down metrics client", map[string]interface{}{"error": err.Error()})
			}
		}
	}

	// TODO: Call shutdown function for tracing when implemented

	return nil
}

// With creates a new context that contains the provided observability components
func With(ctx context.Context, logger Logger, metrics MetricsClient, startSpan StartSpanFunc) context.Context {
	// TODO: Implement context storage using context values
	return ctx
}

// FromContext extracts the observability components from the provided context
func FromContext(ctx context.Context) (Logger, MetricsClient, StartSpanFunc) {
	// TODO: Implement context retrieval using context values
	return DefaultLogger, DefaultMetricsClient, DefaultStartSpan
}
