package observability

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestHelper functions
func captureOutput(f func()) string {
	// Capture stderr since StandardLogger writes to stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Run the function
	f()

	// Close the writer and restore stderr
	_ = w.Close()
	os.Stderr = oldStderr

	// Read the captured output
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

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
	// NoopLogger shouldn't output anything, so we just verify it doesn't panic
	logger := NewNoopLogger()

	// Log messages that should be ignored
	logger.Debug("Debug message", map[string]interface{}{"key": "value"})
	logger.Info("Info message", map[string]interface{}{"key": "value"})
	logger.Warn("Warn message", map[string]interface{}{"key": "value"})
	logger.Error("Error message", map[string]interface{}{"key": "value"})

	// Test that WithPrefix returns a noop logger
	prefixedLogger := logger.WithPrefix("prefix")
	prefixedLogger.Info("Prefixed message", nil)

	// NoopLogger should never produce output, so if we got here without panics, the test passes
}
