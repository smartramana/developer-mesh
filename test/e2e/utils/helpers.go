package utils

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Config holds E2E test configuration
type Config struct {
	MCPBaseURL    string
	APIBaseURL    string
	APIKey        string
	TenantID      string
	TestTimeout   time.Duration
	MaxRetries    int
	EnableDebug   bool
	ReportDir     string
	ParallelTests int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	config := &Config{
		MCPBaseURL:    getEnvOrDefault("MCP_BASE_URL", "mcp.dev-mesh.io"),
		APIBaseURL:    getEnvOrDefault("API_BASE_URL", "api.dev-mesh.io"),
		APIKey:        getEnvOrDefault("E2E_API_KEY", ""),
		TenantID:      getEnvOrDefault("E2E_TENANT_ID", "e2e-test-tenant"),
		TestTimeout:   parseDurationOrDefault(getEnvOrDefault("E2E_TEST_TIMEOUT", "5m"), 5*time.Minute),
		MaxRetries:    parseIntOrDefault(getEnvOrDefault("E2E_MAX_RETRIES", "3"), 3),
		EnableDebug:   getEnvOrDefault("E2E_DEBUG", "false") == "true",
		ReportDir:     getEnvOrDefault("E2E_REPORT_DIR", "test-results"),
		ParallelTests: parseIntOrDefault(getEnvOrDefault("E2E_PARALLEL_TESTS", "5"), 5),
	}

	// Generate API key if not provided
	if config.APIKey == "" {
		config.APIKey = fmt.Sprintf("e2e-test-key-%s", uuid.New().String())
	}

	return config
}

// getEnvOrDefault gets environment variable or returns default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseDurationOrDefault parses duration or returns default
func parseDurationOrDefault(value string, defaultDuration time.Duration) time.Duration {
	if d, err := time.ParseDuration(value); err == nil {
		return d
	}
	return defaultDuration
}

// parseIntOrDefault parses int or returns default
func parseIntOrDefault(value string, defaultInt int) int {
	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
		return result
	}
	return defaultInt
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}

			// Exponential backoff
			delay = time.Duration(float64(delay) * config.Multiplier)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}

		if err := fn(); err != nil {
			lastErr = err
			continue
		}

		return nil
	}

	return fmt.Errorf("failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// WaitFor waits for a condition to be true
func WaitFor(ctx context.Context, interval time.Duration, condition func() (bool, error)) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			ok, err := condition()
			if err != nil {
				return err
			}
			if ok {
				return nil
			}
		}
	}
}

// Eventually runs a function until it succeeds or timeout
func Eventually(fn func() error, timeout time.Duration, interval time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return WaitFor(ctx, interval, func() (bool, error) {
		err := fn()
		return err == nil, nil
	})
}

// TestLogger provides logging for tests
type TestLogger struct {
	prefix string
	debug  bool
}

// NewTestLogger creates a new test logger
func NewTestLogger(prefix string, debug bool) *TestLogger {
	return &TestLogger{
		prefix: prefix,
		debug:  debug,
	}
}

// Info logs an info message
func (tl *TestLogger) Info(format string, args ...interface{}) {
	fmt.Printf("[%s] [INFO] %s: %s\n",
		time.Now().Format("15:04:05"),
		tl.prefix,
		fmt.Sprintf(format, args...))
}

// Debug logs a debug message
func (tl *TestLogger) Debug(format string, args ...interface{}) {
	if tl.debug {
		fmt.Printf("[%s] [DEBUG] %s: %s\n",
			time.Now().Format("15:04:05"),
			tl.prefix,
			fmt.Sprintf(format, args...))
	}
}

// Error logs an error message
func (tl *TestLogger) Error(format string, args ...interface{}) {
	fmt.Printf("[%s] [ERROR] %s: %s\n",
		time.Now().Format("15:04:05"),
		tl.prefix,
		fmt.Sprintf(format, args...))
}

// GenerateTestName generates a unique test name
func GenerateTestName(prefix string) string {
	timestamp := time.Now().Format("20060102-150405")
	id := strings.Split(uuid.New().String(), "-")[0]
	return fmt.Sprintf("%s-%s-%s", prefix, timestamp, id)
}

// AssertEventually asserts that a condition becomes true within timeout
func AssertEventually(t TestingT, condition func() bool, timeout time.Duration, interval time.Duration, msgAndArgs ...interface{}) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}

	if len(msgAndArgs) > 0 {
		t.Errorf("Condition not met within %v: %v", timeout, msgAndArgs[0])
	} else {
		t.Errorf("Condition not met within %v", timeout)
	}
}

// TestingT is a minimal interface for test assertions
type TestingT interface {
	Errorf(format string, args ...interface{})
	FailNow()
	Helper()
}

// MeasureTime measures execution time
func MeasureTime(name string, fn func() error) (time.Duration, error) {
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	return duration, err
}

// Parallel runs functions in parallel and collects results
func Parallel(fns ...func() error) []error {
	errors := make([]error, len(fns))
	done := make(chan int, len(fns))

	for i, fn := range fns {
		go func(index int, f func() error) {
			errors[index] = f()
			done <- index
		}(i, fn)
	}

	for i := 0; i < len(fns); i++ {
		<-done
	}

	return errors
}

// CaptureMetrics captures performance metrics during test execution
type MetricsCapture struct {
	StartTime    time.Time
	EndTime      time.Time
	MessagesSent int64
	MessagesRecv int64
	BytesSent    int64
	BytesRecv    int64
	Errors       int64
	Latencies    []time.Duration
}

// NewMetricsCapture creates a new metrics capture
func NewMetricsCapture() *MetricsCapture {
	return &MetricsCapture{
		StartTime: time.Now(),
		Latencies: make([]time.Duration, 0),
	}
}

// RecordLatency records a latency measurement
func (mc *MetricsCapture) RecordLatency(latency time.Duration) {
	mc.Latencies = append(mc.Latencies, latency)
}

// Finalize finalizes the metrics capture
func (mc *MetricsCapture) Finalize() {
	mc.EndTime = time.Now()
}

// Duration returns the total duration
func (mc *MetricsCapture) Duration() time.Duration {
	return mc.EndTime.Sub(mc.StartTime)
}

// AverageLatency calculates average latency
func (mc *MetricsCapture) AverageLatency() time.Duration {
	if len(mc.Latencies) == 0 {
		return 0
	}

	var total time.Duration
	for _, l := range mc.Latencies {
		total += l
	}

	return total / time.Duration(len(mc.Latencies))
}

// P99Latency calculates 99th percentile latency
func (mc *MetricsCapture) P99Latency() time.Duration {
	if len(mc.Latencies) == 0 {
		return 0
	}

	// Simple implementation - in production use proper percentile calculation
	index := int(float64(len(mc.Latencies)) * 0.99)
	if index >= len(mc.Latencies) {
		index = len(mc.Latencies) - 1
	}

	return mc.Latencies[index]
}
