package client

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Logger is an interface that mimics the observability.Logger interface
// but can be used safely in tests without importing internal packages
type Logger interface {
	Info(msg string, fields map[string]interface{})
	Infof(format string, args ...interface{})
	Warn(msg string, fields map[string]interface{})
	Warnf(format string, args ...interface{})
	Error(msg string, fields map[string]interface{})
	Errorf(format string, args ...interface{})
	Debug(msg string, fields map[string]interface{})
	Debugf(format string, args ...interface{})
	Fatal(msg string, fields map[string]interface{})
	Fatalf(format string, args ...interface{})
	With(fields map[string]interface{}) Logger
	WithPrefix(prefix string) Logger
}

// TestLogger is a simple logger implementation for tests
type TestLogger struct {
	mu     sync.Mutex
	logs   []LogEntry
	fields map[string]interface{}
	prefix string
}

// LogEntry represents a single log entry
type LogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

// NewTestLogger creates a new test logger
func NewTestLogger() *TestLogger {
	return &TestLogger{
		logs:   make([]LogEntry, 0),
		fields: make(map[string]interface{}),
	}
}

// Info logs an info message
func (l *TestLogger) Info(msg string, fields map[string]interface{}) {
	l.log("INFO", msg, fields)
}

// Infof logs an info message with formatting
func (l *TestLogger) Infof(format string, args ...interface{}) {
	l.log("INFO", fmt.Sprintf(format, args...), nil)
}

// Warn logs a warning message
func (l *TestLogger) Warn(msg string, fields map[string]interface{}) {
	l.log("WARN", msg, fields)
}

// Warnf logs a warning message with formatting
func (l *TestLogger) Warnf(format string, args ...interface{}) {
	l.log("WARN", fmt.Sprintf(format, args...), nil)
}

// Error logs an error message
func (l *TestLogger) Error(msg string, fields map[string]interface{}) {
	l.log("ERROR", msg, fields)
}

// Errorf logs an error message with formatting
func (l *TestLogger) Errorf(format string, args ...interface{}) {
	l.log("ERROR", fmt.Sprintf(format, args...), nil)
}

// Debug logs a debug message
func (l *TestLogger) Debug(msg string, fields map[string]interface{}) {
	l.log("DEBUG", msg, fields)
}

// Debugf logs a debug message with formatting
func (l *TestLogger) Debugf(format string, args ...interface{}) {
	l.log("DEBUG", fmt.Sprintf(format, args...), nil)
}

// Fatal logs a fatal message and exits
func (l *TestLogger) Fatal(msg string, fields map[string]interface{}) {
	l.log("FATAL", msg, fields)
	// In a test logger, we don't actually exit
}

// Fatalf logs a fatal message with formatting and exits
func (l *TestLogger) Fatalf(format string, args ...interface{}) {
	l.log("FATAL", fmt.Sprintf(format, args...), nil)
	// In a test logger, we don't actually exit
}

// With returns a new logger with the given fields
func (l *TestLogger) With(fields map[string]interface{}) Logger {
	newLogger := NewTestLogger()
	newLogger.prefix = l.prefix
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// WithPrefix returns a new logger with the given prefix
func (l *TestLogger) WithPrefix(prefix string) Logger {
	newLogger := NewTestLogger()
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	// Combine prefixes if there's already one
	if l.prefix != "" {
		newLogger.prefix = l.prefix + "." + prefix
	} else {
		newLogger.prefix = prefix
	}
	return newLogger
}

// log adds a log entry to the log list and also prints to stderr
func (l *TestLogger) log(level, msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Merge fields
	mergedFields := make(map[string]interface{})
	for k, v := range l.fields {
		mergedFields[k] = v
	}
	for k, v := range fields {
		mergedFields[k] = v
	}

	// Add to logs
	l.logs = append(l.logs, LogEntry{
		Level:   level,
		Message: msg,
		Fields:  mergedFields,
	})

	// Print to stderr
	fieldsStr := ""
	if len(mergedFields) > 0 {
		parts := make([]string, 0, len(mergedFields))
		for k, v := range mergedFields {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		fieldsStr = " " + strings.Join(parts, " ")
	}
	
	// Include prefix if available
	prefixStr := ""
	if l.prefix != "" {
		prefixStr = l.prefix + ": "
	}
	
	fmt.Fprintf(os.Stderr, "[TEST-%s] %s%s%s\n", level, prefixStr, msg, fieldsStr)
}

// GetLogs returns all logs recorded by this logger
func (l *TestLogger) GetLogs() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Return a copy to avoid race conditions
	result := make([]LogEntry, len(l.logs))
	copy(result, l.logs)
	return result
}

// FilterLogs returns logs that match the given level
func (l *TestLogger) FilterLogs(level string) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	var result []LogEntry
	for _, entry := range l.logs {
		if entry.Level == level {
			result = append(result, entry)
		}
	}
	return result
}

// Reset clears all logs
func (l *TestLogger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = make([]LogEntry, 0)
}
