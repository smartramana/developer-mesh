// Package mocks provides mock implementations for testing
package mocks

import (
	"context"
	"fmt"
)

// Logger is a mock implementation of the observability.Logger interface
type Logger struct {
	// We could add fields to track log messages for assertions in tests
	Messages []LogMessage
}

// LogMessage represents a logged message
type LogMessage struct {
	Level string
	Message string
	Metadata map[string]interface{}
}

// NewLogger creates a new mock logger
func NewLogger() *Logger {
	return &Logger{
		Messages: make([]LogMessage, 0),
	}
}

// Info logs an informational message
func (l *Logger) Info(msg string, metadata map[string]interface{}) {
	l.Messages = append(l.Messages, LogMessage{
		Level: "INFO",
		Message: msg,
		Metadata: metadata,
	})
	fmt.Printf("[INFO] %s %v\n", msg, metadata)
}

// Error logs an error message
func (l *Logger) Error(msg string, metadata map[string]interface{}) {
	l.Messages = append(l.Messages, LogMessage{
		Level: "ERROR",
		Message: msg,
		Metadata: metadata,
	})
	fmt.Printf("[ERROR] %s %v\n", msg, metadata)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, metadata map[string]interface{}) {
	l.Messages = append(l.Messages, LogMessage{
		Level: "DEBUG",
		Message: msg,
		Metadata: metadata,
	})
	fmt.Printf("[DEBUG] %s %v\n", msg, metadata)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, metadata map[string]interface{}) {
	l.Messages = append(l.Messages, LogMessage{
		Level: "WARN",
		Message: msg,
		Metadata: metadata,
	})
	fmt.Printf("[WARN] %s %v\n", msg, metadata)
}

// WithContext returns a new logger with context
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return l
}

// WithField returns a new logger with additional field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l
}

// WithFields returns a new logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	return l
}
