// Package observability provides unified observability functionality for the MCP system.
// It consolidates logging, metrics, and tracing into a cohesive interface.
package observability

import (
	"context"
	"sync"

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

	// shutdownFuncs stores cleanup functions to be called during shutdown
	shutdownFuncs []func() error
	shutdownMutex sync.Mutex
)

// Additional context keys for storing observability components
const (
	loggerKey    contextKey = "observability_logger"
	metricsKey   contextKey = "observability_metrics"
	startSpanKey contextKey = "observability_startspan"
)

// Initialize initializes the observability system with the given configuration.
// This is the main entry point for configuring all observability components.
func Initialize(cfg Config) error {
	// Set up default logger if not already set
	if DefaultLogger == nil {
		DefaultLogger = NewStandardLogger("developer-mesh")
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

				// Store shutdown function to be called during Shutdown()
				registerShutdownFunc(func() error {
					shutdownFunc()
					return nil
				})
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
	var shutdownErrors []error

	// Close metrics client if it exists
	if DefaultMetricsClient != nil {
		if err := DefaultMetricsClient.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, err)
			if DefaultLogger != nil {
				DefaultLogger.Error("Error shutting down metrics client", map[string]interface{}{"error": err.Error()})
			}
		}
	}

	// Call all registered shutdown functions
	shutdownMutex.Lock()
	funcs := make([]func() error, len(shutdownFuncs))
	copy(funcs, shutdownFuncs)
	shutdownFuncs = nil // Clear the list
	shutdownMutex.Unlock()

	for _, fn := range funcs {
		if err := fn(); err != nil {
			shutdownErrors = append(shutdownErrors, err)
			if DefaultLogger != nil {
				DefaultLogger.Error("Error during observability shutdown", map[string]interface{}{"error": err.Error()})
			}
		}
	}

	// Return the first error if any occurred
	if len(shutdownErrors) > 0 {
		return shutdownErrors[0]
	}
	return nil
}

// With creates a new context that contains the provided observability components
func With(ctx context.Context, logger Logger, metrics MetricsClient, startSpan StartSpanFunc) context.Context {
	if logger != nil {
		ctx = context.WithValue(ctx, loggerKey, logger)
	}
	if metrics != nil {
		ctx = context.WithValue(ctx, metricsKey, metrics)
	}
	if startSpan != nil {
		ctx = context.WithValue(ctx, startSpanKey, startSpan)
	}
	return ctx
}

// FromContext extracts the observability components from the provided context
func FromContext(ctx context.Context) (Logger, MetricsClient, StartSpanFunc) {
	logger := DefaultLogger
	metrics := DefaultMetricsClient
	startSpan := DefaultStartSpan

	if l, ok := ctx.Value(loggerKey).(Logger); ok && l != nil {
		logger = l
	}
	if m, ok := ctx.Value(metricsKey).(MetricsClient); ok && m != nil {
		metrics = m
	}
	if s, ok := ctx.Value(startSpanKey).(StartSpanFunc); ok && s != nil {
		startSpan = s
	}

	return logger, metrics, startSpan
}

// registerShutdownFunc registers a function to be called during shutdown
func registerShutdownFunc(fn func() error) {
	if fn == nil {
		return
	}

	shutdownMutex.Lock()
	defer shutdownMutex.Unlock()
	shutdownFuncs = append(shutdownFuncs, fn)
}
