package observability

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// mockLogger for testing
type mockLogger struct {
	logs []logEntry
	mu   sync.Mutex
}

type logEntry struct {
	level  string
	msg    string
	fields map[string]interface{}
}

func newMockLogger() *mockLogger {
	return &mockLogger{
		logs: make([]logEntry, 0),
	}
}

func (l *mockLogger) Debug(msg string, fields map[string]interface{}) { l.log("debug", msg, fields) }
func (l *mockLogger) Info(msg string, fields map[string]interface{})  { l.log("info", msg, fields) }
func (l *mockLogger) Warn(msg string, fields map[string]interface{})  { l.log("warn", msg, fields) }
func (l *mockLogger) Error(msg string, fields map[string]interface{}) { l.log("error", msg, fields) }
func (l *mockLogger) Fatal(msg string, fields map[string]interface{}) { l.log("fatal", msg, fields) }
func (l *mockLogger) Debugf(format string, args ...interface{})       {}
func (l *mockLogger) Infof(format string, args ...interface{})        {}
func (l *mockLogger) Warnf(format string, args ...interface{})        {}
func (l *mockLogger) Errorf(format string, args ...interface{})       {}
func (l *mockLogger) Fatalf(format string, args ...interface{})       {}
func (l *mockLogger) WithPrefix(prefix string) Logger                 { return l }
func (l *mockLogger) With(fields map[string]interface{}) Logger       { return l }

func (l *mockLogger) log(level, msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, logEntry{level: level, msg: msg, fields: fields})
}

// mockMetricsClient for testing
type mockMetricsClient struct{}

func (m *mockMetricsClient) RecordEvent(source, eventType string)                                 {}
func (m *mockMetricsClient) RecordLatency(operation string, duration time.Duration)               {}
func (m *mockMetricsClient) RecordCounter(name string, value float64, labels map[string]string)   {}
func (m *mockMetricsClient) RecordGauge(name string, value float64, labels map[string]string)     {}
func (m *mockMetricsClient) RecordHistogram(name string, value float64, labels map[string]string) {}
func (m *mockMetricsClient) RecordTimer(name string, duration time.Duration, labels map[string]string) {
}
func (m *mockMetricsClient) RecordCacheOperation(operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string) {
}
func (m *mockMetricsClient) RecordAPIOperation(api string, operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) StartTimer(name string, labels map[string]string) func() {
	return func() {}
}
func (m *mockMetricsClient) IncrementCounter(name string, value float64) {}
func (m *mockMetricsClient) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {
}
func (m *mockMetricsClient) RecordDuration(name string, duration time.Duration) {}
func (m *mockMetricsClient) Close() error                                       { return nil }

// mockSpan for testing
type mockSpan struct{}

func (m *mockSpan) End()                                                    {}
func (m *mockSpan) SetAttribute(key string, value interface{})              {}
func (m *mockSpan) AddEvent(name string, attributes map[string]interface{}) {}
func (m *mockSpan) RecordError(err error)                                   {}
func (m *mockSpan) SetStatus(code int, description string)                  {}
func (m *mockSpan) SpanContext() trace.SpanContext                          { return trace.SpanContext{} }
func (m *mockSpan) TracerProvider() trace.TracerProvider                    { return noop.NewTracerProvider() }

func TestObservabilityContext(t *testing.T) {
	t.Run("With and FromContext", func(t *testing.T) {
		ctx := context.Background()

		logger := newMockLogger()
		metrics := &mockMetricsClient{}
		startSpan := func(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, Span) {
			return ctx, &mockSpan{}
		}

		// Add components to context
		ctx = With(ctx, logger, metrics, startSpan)

		// Retrieve components from context
		retrievedLogger, retrievedMetrics, retrievedStartSpan := FromContext(ctx)

		assert.Equal(t, logger, retrievedLogger)
		assert.Equal(t, metrics, retrievedMetrics)
		assert.NotNil(t, retrievedStartSpan)
	})

	t.Run("FromContext with empty context uses defaults", func(t *testing.T) {
		// Save original defaults
		origLogger := DefaultLogger
		origMetrics := DefaultMetricsClient
		origStartSpan := DefaultStartSpan
		defer func() {
			DefaultLogger = origLogger
			DefaultMetricsClient = origMetrics
			DefaultStartSpan = origStartSpan
		}()

		// Set test defaults
		DefaultLogger = newMockLogger()
		DefaultMetricsClient = &mockMetricsClient{}
		DefaultStartSpan = NoopStartSpan

		ctx := context.Background()
		logger, metrics, startSpan := FromContext(ctx)

		assert.Equal(t, DefaultLogger, logger)
		assert.Equal(t, DefaultMetricsClient, metrics)
		assert.NotNil(t, startSpan)
	})

	t.Run("With handles nil values", func(t *testing.T) {
		// Save and set defaults
		origLogger := DefaultLogger
		origMetrics := DefaultMetricsClient
		origStartSpan := DefaultStartSpan
		defer func() {
			DefaultLogger = origLogger
			DefaultMetricsClient = origMetrics
			DefaultStartSpan = origStartSpan
		}()

		DefaultLogger = newMockLogger()
		DefaultMetricsClient = &mockMetricsClient{}
		DefaultStartSpan = NoopStartSpan

		ctx := context.Background()

		// Should not panic with nil values
		ctx = With(ctx, nil, nil, nil)

		// Should return defaults when retrieving
		logger, metrics, startSpan := FromContext(ctx)
		assert.Equal(t, DefaultLogger, logger)
		assert.Equal(t, DefaultMetricsClient, metrics)
		assert.NotNil(t, startSpan)
	})
}

func TestObservabilityShutdown(t *testing.T) {
	t.Run("Shutdown calls registered functions", func(t *testing.T) {
		// Clear any existing shutdown functions
		shutdownMutex.Lock()
		shutdownFuncs = nil
		shutdownMutex.Unlock()

		// Track calls
		var calls []string
		var mu sync.Mutex

		// Register multiple shutdown functions
		registerShutdownFunc(func() error {
			mu.Lock()
			calls = append(calls, "func1")
			mu.Unlock()
			return nil
		})

		registerShutdownFunc(func() error {
			mu.Lock()
			calls = append(calls, "func2")
			mu.Unlock()
			return nil
		})

		// Save and replace metrics client
		origMetrics := DefaultMetricsClient
		DefaultMetricsClient = &mockMetricsClient{}
		defer func() {
			DefaultMetricsClient = origMetrics
		}()

		// Call shutdown
		err := ObservabilityShutdown()
		require.NoError(t, err)

		// Verify all functions were called
		mu.Lock()
		assert.Equal(t, []string{"func1", "func2"}, calls)
		mu.Unlock()

		// Verify shutdown functions were cleared
		shutdownMutex.Lock()
		assert.Empty(t, shutdownFuncs)
		shutdownMutex.Unlock()
	})

	t.Run("Shutdown handles errors", func(t *testing.T) {
		// Clear any existing shutdown functions
		shutdownMutex.Lock()
		shutdownFuncs = nil
		shutdownMutex.Unlock()

		// Register function that returns error
		testErr := errors.New("shutdown error")
		registerShutdownFunc(func() error {
			return testErr
		})

		// Save and replace logger and metrics
		origLogger := DefaultLogger
		origMetrics := DefaultMetricsClient
		DefaultLogger = newMockLogger()
		DefaultMetricsClient = &mockMetricsClient{}
		defer func() {
			DefaultLogger = origLogger
			DefaultMetricsClient = origMetrics
		}()

		// Call shutdown
		err := ObservabilityShutdown()
		assert.Equal(t, testErr, err)

		// Verify error was logged
		logger := DefaultLogger.(*mockLogger)
		assert.Len(t, logger.logs, 1)
		assert.Equal(t, "error", logger.logs[0].level)
		assert.Contains(t, logger.logs[0].msg, "Error during observability shutdown")
	})

	t.Run("registerShutdownFunc handles nil", func(t *testing.T) {
		// Should not panic
		registerShutdownFunc(nil)

		// Verify nothing was added
		shutdownMutex.Lock()
		for _, fn := range shutdownFuncs {
			assert.NotNil(t, fn)
		}
		shutdownMutex.Unlock()
	})
}

func TestInitialize(t *testing.T) {
	t.Run("Initialize sets defaults when nil", func(t *testing.T) {
		// Clear defaults
		DefaultLogger = nil
		DefaultMetricsClient = nil
		DefaultStartSpan = nil

		cfg := Config{
			Tracing: TracingConfig{
				Enabled: false,
			},
		}

		err := Initialize(cfg)
		require.NoError(t, err)

		assert.NotNil(t, DefaultLogger)
		assert.NotNil(t, DefaultMetricsClient)
		assert.NotNil(t, DefaultStartSpan)
	})

	t.Run("Initialize preserves existing components", func(t *testing.T) {
		// Set custom components
		logger := newMockLogger()
		metrics := &mockMetricsClient{}
		startSpan := func(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, Span) {
			return ctx, &mockSpan{}
		}

		DefaultLogger = logger
		DefaultMetricsClient = metrics
		DefaultStartSpan = startSpan

		cfg := Config{
			Tracing: TracingConfig{
				Enabled: false,
			},
		}

		err := Initialize(cfg)
		require.NoError(t, err)

		// Should preserve existing components
		assert.Equal(t, logger, DefaultLogger)
		assert.Equal(t, metrics, DefaultMetricsClient)
		assert.NotNil(t, DefaultStartSpan)
	})

	t.Run("Initialize with tracing disabled", func(t *testing.T) {
		DefaultStartSpan = nil

		cfg := Config{
			Tracing: TracingConfig{
				Enabled: false,
			},
		}

		err := Initialize(cfg)
		require.NoError(t, err)

		// Should use NoopStartSpan
		assert.NotNil(t, DefaultStartSpan)
		ctx, span := DefaultStartSpan(context.Background(), "test")
		assert.NotNil(t, ctx)
		assert.NotNil(t, span)
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("Concurrent shutdown registration", func(t *testing.T) {
		// Clear any existing shutdown functions
		shutdownMutex.Lock()
		shutdownFuncs = nil
		shutdownMutex.Unlock()

		var wg sync.WaitGroup
		var callCount int32

		// Register functions concurrently
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				registerShutdownFunc(func() error {
					atomic.AddInt32(&callCount, 1)
					return nil
				})
			}()
		}

		wg.Wait()

		// Verify all were registered
		shutdownMutex.Lock()
		assert.Len(t, shutdownFuncs, 100)
		shutdownMutex.Unlock()

		// Call shutdown and verify all were called
		err := ObservabilityShutdown()
		require.NoError(t, err)
		assert.Equal(t, int32(100), atomic.LoadInt32(&callCount))
	})

	t.Run("Concurrent context operations", func(t *testing.T) {
		ctx := context.Background()
		logger := newMockLogger()
		metrics := &mockMetricsClient{}
		startSpan := NoopStartSpan

		ctx = With(ctx, logger, metrics, startSpan)

		var wg sync.WaitGroup

		// Read from context concurrently
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				l, m, s := FromContext(ctx)
				assert.NotNil(t, l)
				assert.NotNil(t, m)
				assert.NotNil(t, s)
			}()
		}

		wg.Wait()
	})
}
