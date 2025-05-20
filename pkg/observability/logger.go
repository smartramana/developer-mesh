// Package observability provides unified logging, metrics, and tracing
// capabilities for the devops-mcp workspace. It follows a consistent
// approach to observability across all services and components.
package observability

import (
	"fmt"
	"log"
	"os"
	"time"
)

// StandardLogger is a logger implementation that uses the standard log package
type StandardLogger struct {
	prefix string
	level  LogLevel
}

// NewStandardLogger creates a new StandardLogger with the given prefix
func NewStandardLogger(prefix string) Logger {
	return &StandardLogger{
		prefix: prefix,
		level:  LogLevelInfo, // Default to INFO level
	}
}

// WithLevel returns a new logger with the specified log level
func (l *StandardLogger) WithLevel(level LogLevel) *StandardLogger {
	return &StandardLogger{
		prefix: l.prefix,
		level:  level,
	}
}

// Debug logs a debug message
func (l *StandardLogger) Debug(msg string, fields map[string]interface{}) {
	if l.levelEnabled(LogLevelDebug) {
		l.log(LogLevelDebug, msg, fields)
	}
}

// Info logs an info message
func (l *StandardLogger) Info(msg string, fields map[string]interface{}) {
	if l.levelEnabled(LogLevelInfo) {
		l.log(LogLevelInfo, msg, fields)
	}
}

// Warn logs a warning message
func (l *StandardLogger) Warn(msg string, fields map[string]interface{}) {
	if l.levelEnabled(LogLevelWarn) {
		l.log(LogLevelWarn, msg, fields)
	}
}

// Error logs an error message
func (l *StandardLogger) Error(msg string, fields map[string]interface{}) {
	l.log(LogLevelError, msg, fields)
}

// Fatal logs a fatal message and exits
func (l *StandardLogger) Fatal(msg string, fields map[string]interface{}) {
	l.log(LogLevelFatal, msg, fields)
	os.Exit(1)
}

// WithPrefix returns a new logger with the given prefix
func (l *StandardLogger) WithPrefix(prefix string) Logger {
	return &StandardLogger{
		prefix: prefix,
		level:  l.level,
	}
}

// formatFields formats fields as a string
func (l *StandardLogger) formatFields(fields map[string]interface{}) string {
	if len(fields) == 0 {
		return ""
	}
	
	// Format the fields as key=value pairs
	result := ""
	for k, v := range fields {
		result += fmt.Sprintf(" %s=%v", k, v)
	}
	return result
}

// levelEnabled checks if the given log level is enabled
func (l *StandardLogger) levelEnabled(level LogLevel) bool {
	// Define log level hierarchy
	levelHierarchy := map[LogLevel]int{
		LogLevelDebug: 0,
		LogLevelInfo:  1,
		LogLevelWarn:  2,
		LogLevelError: 3,
		LogLevelFatal: 4,
	}
	
	// Check if the current level is equal to or greater than the minimum level
	return levelHierarchy[level] >= levelHierarchy[l.level]
}

// log logs a message with the given level
func (l *StandardLogger) log(level LogLevel, msg string, fields map[string]interface{}) {
	// Get current timestamp
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	
	// Create log prefix with timestamp, level, and logger prefix
	logPrefix := fmt.Sprintf("%s [%s] [%s]", timestamp, level, l.prefix)
	
	// Format the fields
	fieldsStr := l.formatFields(fields)
	
	// Log the message
	log.Printf("%s %s%s", logPrefix, msg, fieldsStr)
	
	// Exit if fatal
	if level == LogLevelFatal {
		os.Exit(1)
	}
}

// Debugf logs a formatted debug message
func (l *StandardLogger) Debugf(format string, args ...interface{}) {
	if l.levelEnabled(LogLevelDebug) {
		l.log(LogLevelDebug, fmt.Sprintf(format, args...), nil)
	}
}

// Infof logs a formatted info message
func (l *StandardLogger) Infof(format string, args ...interface{}) {
	if l.levelEnabled(LogLevelInfo) {
		l.log(LogLevelInfo, fmt.Sprintf(format, args...), nil)
	}
}

// Warnf logs a formatted warning message
func (l *StandardLogger) Warnf(format string, args ...interface{}) {
	if l.levelEnabled(LogLevelWarn) {
		l.log(LogLevelWarn, fmt.Sprintf(format, args...), nil)
	}
}

// Errorf logs a formatted error message
func (l *StandardLogger) Errorf(format string, args ...interface{}) {
	l.log(LogLevelError, fmt.Sprintf(format, args...), nil)
}

// Fatalf logs a formatted fatal message and exits
func (l *StandardLogger) Fatalf(format string, args ...interface{}) {
	// Fatal always logs regardless of level
	l.log(LogLevelFatal, fmt.Sprintf(format, args...), nil)
}

// NoopLogger is a logger that does nothing
type NoopLogger struct{}

// Debug implements Logger.Debug
func (l *NoopLogger) Debug(msg string, fields map[string]interface{}) {}

// Info implements Logger.Info
func (l *NoopLogger) Info(msg string, fields map[string]interface{}) {}

// Warn implements Logger.Warn
func (l *NoopLogger) Warn(msg string, fields map[string]interface{}) {}

// Error implements Logger.Error
func (l *NoopLogger) Error(msg string, fields map[string]interface{}) {}

// Fatal implements Logger.Fatal
func (l *NoopLogger) Fatal(msg string, fields map[string]interface{}) {}

// WithPrefix implements Logger.WithPrefix
func (l *NoopLogger) WithPrefix(prefix string) Logger {
	return l
}

// NewNoopLogger creates a new NoopLogger
func NewNoopLogger() Logger {
	return &NoopLogger{}
}
