package observability

import (
	"fmt"
	"os"
	"time"
)

var DefaultLogger = NewLogger("devops-mcp")

// LogLevel defines log message severity
type LogLevel string

// Log levels
const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
)

// Logger provides structured logging capabilities
type Logger struct {
	serviceName string
	minLevel    LogLevel
}

// NewLogger creates a new structured logger
func NewLogger(serviceName string) *Logger {
	return &Logger{
		serviceName: serviceName,
		minLevel:    LogLevelInfo, // Default minimum level
	}
}

// SetMinLevel sets the minimum log level to display
func (l *Logger) SetMinLevel(level LogLevel) {
	l.minLevel = level
}

// shouldLog checks if the log level is high enough to log
func (l *Logger) shouldLog(level LogLevel) bool {
	switch level {
	case LogLevelDebug:
		return l.minLevel == LogLevelDebug
	case LogLevelInfo:
		return l.minLevel == LogLevelDebug || l.minLevel == LogLevelInfo
	case LogLevelWarn:
		return l.minLevel == LogLevelDebug || l.minLevel == LogLevelInfo || l.minLevel == LogLevelWarn
	case LogLevelError:
		return true // Always log errors
	default:
		return true
	}
}

// Debug logs a debug message with structured data
func (l *Logger) Debug(message string, data map[string]any) {
	if l.shouldLog(LogLevelDebug) {
		l.log(LogLevelDebug, message, data)
	}
}

// Info logs an informational message with structured data
func (l *Logger) Info(message string, data map[string]any) {
	if l.shouldLog(LogLevelInfo) {
		l.log(LogLevelInfo, message, data)
	}
}

// Warn logs a warning message with structured data
func (l *Logger) Warn(message string, data map[string]any) {
	if l.shouldLog(LogLevelWarn) {
		l.log(LogLevelWarn, message, data)
	}
}

// Error logs an error message with structured data
func (l *Logger) Error(message string, data map[string]any) {
	if l.shouldLog(LogLevelError) {
		l.log(LogLevelError, message, data)
	}
}

// WithPrefix creates a new logger with a combined service name
func (l *Logger) WithPrefix(prefix string) *Logger {
	return NewLogger(l.serviceName + "." + prefix)
}

// log handles the actual logging
func (l *Logger) log(level LogLevel, message string, data map[string]any) {
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Format structured data as key=value pairs
	structuredData := ""
	for k, v := range data {
		structuredData += fmt.Sprintf(" %s=%v", k, v)
	}

	// Format log message
	logMsg := fmt.Sprintf("[%s] %s [%s] %s%s\n",
		timestamp,
		level,
		l.serviceName,
		message,
		structuredData,
	)

	// Output log message
	if level == LogLevelError {
		_, _ = os.Stderr.WriteString(logMsg) // Best effort logging
	} else {
		_, _ = os.Stdout.WriteString(logMsg) // Best effort logging
	}
}
