package database

import (
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	pkgObs "github.com/S-Corkum/devops-mcp/pkg/observability"
)

// obsLoggerAdapter adapts an internal/observability.Logger to the pkg/observability.Logger interface
type obsLoggerAdapter struct {
	internal *observability.Logger
}

// Debug logs a debug message
func (a *obsLoggerAdapter) Debug(msg string, keyvals map[string]interface{}) {
	a.internal.Debug(msg, keyvals)
}

// Info logs an info message
func (a *obsLoggerAdapter) Info(msg string, keyvals map[string]interface{}) {
	a.internal.Info(msg, keyvals)
}

// Warn logs a warning message
func (a *obsLoggerAdapter) Warn(msg string, keyvals map[string]interface{}) {
	a.internal.Warn(msg, keyvals)
}

// Error logs an error message
func (a *obsLoggerAdapter) Error(msg string, keyvals map[string]interface{}) {
	a.internal.Error(msg, keyvals)
}

// WithFields returns a new logger with the given fields
func (a *obsLoggerAdapter) WithFields(keyvals map[string]interface{}) pkgObs.Logger {
	// Create a new logger with the fields
	internalLogger := a.internal.WithFields(keyvals)
	return &obsLoggerAdapter{internal: internalLogger}
}

// WithPrefix returns a new logger with the given prefix
func (a *obsLoggerAdapter) WithPrefix(prefix string) pkgObs.Logger {
	// Create a new logger with the prefix
	internalLogger := a.internal.WithPrefix(prefix)
	return &obsLoggerAdapter{internal: internalLogger}
}
