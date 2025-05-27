package observability

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

// TestHelper functions
func captureOutput(f func()) string {
	// Redirect log output to a buffer
	var buf bytes.Buffer
	oldLogger := log.Default()
	log.SetOutput(&buf)

	// Run the function
	f()

	// Restore the logger
	log.SetOutput(os.Stderr)
	log.SetOutput(oldLogger.Writer())

	return buf.String()
}

func TestLogger_LogLevels(t *testing.T) {
	output := captureOutput(func() {
		// Create a logger with DEBUG level
		logger := NewStandardLogger("test-service").(*StandardLogger).WithLevel(LogLevelDebug)

		// Test each log level
		logger.Debug("Debug message", map[string]interface{}{"key": "value"})
		logger.Info("Info message", map[string]interface{}{"key": "value"})
		logger.Warn("Warn message", map[string]interface{}{"key": "value"})
	})

	t.Logf("Log output: %s", output)

	// Verify all levels were logged - check the raw strings that appear in the log
	if !strings.Contains(output, "Debug message") {
		t.Error("Expected Debug message but it was not found in the output")
	}
	if !strings.Contains(output, "Info message") {
		t.Error("Expected Info message but it was not found in the output")
	}
	if !strings.Contains(output, "Warn message") {
		t.Error("Expected Warn message but it was not found in the output")
	}
}

func TestLogger_MinimumLevel(t *testing.T) {
	output := captureOutput(func() {
		// Create a logger with INFO level
		logger := NewStandardLogger("test-service").(*StandardLogger).WithLevel(LogLevelInfo)

		// Test each log level - Debug should be filtered out
		logger.Debug("Debug message", map[string]interface{}{"key": "value"})
		logger.Info("Info message", map[string]interface{}{"key": "value"})
	})

	t.Logf("Log output: %s", output)

	// Verify only Info level was logged
	if strings.Contains(output, "Debug message") {
		t.Error("Did not expect Debug message when minimum level is INFO")
	}
	if !strings.Contains(output, "Info message") {
		t.Error("Expected Info message but it was not found in the output")
	}
}

func TestLogger_WithPrefix(t *testing.T) {
	output := captureOutput(func() {
		// Create a logger with a prefix
		logger := NewStandardLogger("parent-service")
		prefixedLogger := logger.WithPrefix("child")

		// Log using the prefixed logger
		prefixedLogger.Info("Prefixed message", nil)
	})

	t.Logf("Log output: %s", output)

	// Verify the prefixed logger works
	if !strings.Contains(output, "Prefixed message") {
		t.Error("Expected message not found in the output")
	}
	if !strings.Contains(output, "child") {
		t.Error("Expected prefix 'child' not found in the output")
	}
}

func TestLogger_StructuredData(t *testing.T) {
	output := captureOutput(func() {
		// Create a logger
		logger := NewStandardLogger("test-service")

		// Log with structured data
		data := map[string]interface{}{
			"string": "value",
			"number": 42,
			"bool":   true,
		}
		logger.Info("Message with data", data)
	})

	t.Logf("Log output: %s", output)

	// Verify the structured data is included in the log message
	if !strings.Contains(output, "Message with data") {
		t.Error("Expected message not found in the output")
	}
	if !strings.Contains(output, "string=value") {
		t.Error("Expected 'string=value' not found in the output")
	}
	if !strings.Contains(output, "number=42") {
		t.Error("Expected 'number=42' not found in the output")
	}
	if !strings.Contains(output, "bool=true") {
		t.Error("Expected 'bool=true' not found in the output")
	}
}

func TestLogger_NoopLogger(t *testing.T) {
	// We'll use a custom buffer here since NoopLogger shouldn't output anything
	var buf bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldOutput)

	// Create a noop logger
	logger := NewNoopLogger()

	// Log messages that should be ignored
	logger.Debug("Debug message", map[string]interface{}{"key": "value"})
	logger.Info("Info message", map[string]interface{}{"key": "value"})
	logger.Warn("Warn message", map[string]interface{}{"key": "value"})
	logger.Error("Error message", map[string]interface{}{"key": "value"})

	// Test that WithPrefix returns a noop logger
	prefixedLogger := logger.WithPrefix("prefix")
	prefixedLogger.Info("Prefixed message", nil)

	// Verify no output was produced from NoopLogger operations
	output := buf.String()
	if output != "" {
		t.Errorf("Expected no output from NoopLogger, but got: %s", output)
	}
}
