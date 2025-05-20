// Package mocks provides mock implementations for testing the GitHub adapter
// and other components of the MCP Server.
package mocks

import (
	"context"
	"fmt"
	"sync"
)

// Logger is a mock implementation of the observability.Logger interface.
// It tracks log messages for verification in tests.
type Logger struct {
	// Track log messages for assertions in tests
	Messages []LogMessage
	
	// Mutex for thread-safe logging
	mu sync.Mutex
	
	// Control log output to console during tests
	Verbose bool
}

// LogMessage represents a logged message for test verification
type LogMessage struct {
	Level    string                 // Log level (INFO, ERROR, etc.)
	Message  string                 // Log message text
	Metadata map[string]interface{} // Additional log context
}

// NewLogger creates a new mock logger for testing
func NewLogger() *Logger {
	return &Logger{
		Messages: make([]LogMessage, 0),
		Verbose:  false, // Default to quiet mode
	}
}

// NewVerboseLogger creates a mock logger that prints to console
func NewVerboseLogger() *Logger {
	return &Logger{
		Messages: make([]LogMessage, 0),
		Verbose:  true,
	}
}

// Info logs an informational message
func (l *Logger) Info(msg string, metadata map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Create a copy of metadata to avoid later modifications affecting the logged data
	metadataCopy := make(map[string]interface{})
	for k, v := range metadata {
		metadataCopy[k] = v
	}
	
	l.Messages = append(l.Messages, LogMessage{
		Level:    "INFO",
		Message:  msg,
		Metadata: metadataCopy,
	})
	
	if l.Verbose {
		fmt.Printf("[INFO] %s %v\n", msg, metadataCopy)
	}
}

// Error logs an error message
func (l *Logger) Error(msg string, metadata map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Create a copy of metadata
	metadataCopy := make(map[string]interface{})
	for k, v := range metadata {
		metadataCopy[k] = v
	}
	
	l.Messages = append(l.Messages, LogMessage{
		Level:    "ERROR",
		Message:  msg,
		Metadata: metadataCopy,
	})
	
	if l.Verbose {
		fmt.Printf("[ERROR] %s %v\n", msg, metadataCopy)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, metadata map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Create a copy of metadata
	metadataCopy := make(map[string]interface{})
	for k, v := range metadata {
		metadataCopy[k] = v
	}
	
	l.Messages = append(l.Messages, LogMessage{
		Level:    "DEBUG",
		Message:  msg,
		Metadata: metadataCopy,
	})
	
	if l.Verbose {
		fmt.Printf("[DEBUG] %s %v\n", msg, metadataCopy)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, metadata map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Create a copy of metadata
	metadataCopy := make(map[string]interface{})
	for k, v := range metadata {
		metadataCopy[k] = v
	}
	
	l.Messages = append(l.Messages, LogMessage{
		Level:    "WARN",
		Message:  msg,
		Metadata: metadataCopy,
	})
	
	if l.Verbose {
		fmt.Printf("[WARN] %s %v\n", msg, metadataCopy)
	}
}

// WithContext returns a new logger with context information
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// In the mock, we just return the same logger
	// A real implementation would create a new logger with the context
	return l
}

// WithField returns a new logger with an additional field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	// In the mock, we just return the same logger
	// A real implementation would create a new logger with the field
	return l
}

// WithFields returns a new logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	// In the mock, we just return the same logger
	// A real implementation would create a new logger with the fields
	return l
}

// GetMessages returns all log messages of a specific level
func (l *Logger) GetMessages(level string) []LogMessage {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	var result []LogMessage
	for _, msg := range l.Messages {
		if msg.Level == level {
			result = append(result, msg)
		}
	}
	
	return result
}

// HasMessage checks if a specific message was logged
func (l *Logger) HasMessage(level, containsText string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	for _, msg := range l.Messages {
		if msg.Level == level && containsSubstring(msg.Message, containsText) {
			return true
		}
	}
	
	return false
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
