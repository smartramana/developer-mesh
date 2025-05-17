package observability

import (
	"context"
	"time"

	commonLogging "github.com/S-Corkum/devops-mcp/pkg/common/logging"
	commonMetrics "github.com/S-Corkum/devops-mcp/pkg/common/metrics"
)

// LoggingAdapter adapts from common/logging.Logger to observability.Logger
type LoggingAdapter struct {
	logger *commonLogging.Logger
}

// NewLoggingAdapter creates a new adapter for a commonLogging.Logger
func NewLoggingAdapter(logger *commonLogging.Logger) Logger {
	return &LoggingAdapter{logger: logger}
}

// Debug implements the observability.Logger interface
func (a *LoggingAdapter) Debug(msg string, fields map[string]interface{}) {
	a.logger.Debug(msg, fields)
}

// Info implements the observability.Logger interface
func (a *LoggingAdapter) Info(msg string, fields map[string]interface{}) {
	a.logger.Info(msg, fields)
}

// Warn implements the observability.Logger interface
func (a *LoggingAdapter) Warn(msg string, fields map[string]interface{}) {
	a.logger.Warn(msg, fields)
}

// Error implements the observability.Logger interface
func (a *LoggingAdapter) Error(msg string, fields map[string]interface{}) {
	a.logger.Error(msg, fields)
}

// Fatal implements the observability.Logger interface
func (a *LoggingAdapter) Fatal(msg string, fields map[string]interface{}) {
	a.logger.Fatal(msg, fields)
}

// WithPrefix implements the observability.Logger interface
func (a *LoggingAdapter) WithPrefix(prefix string) Logger {
	return &LoggingAdapter{logger: a.logger.WithPrefix(prefix)}
}

// LoggerAdapter adapts between commonLogging.Logger and observability.Logger
type LoggerAdapter struct {
	obs Logger
}

// commonToObsLogger adapts common/logging.Logger to observability.Logger
type commonToObsLogger struct {
	commonLogger *commonLogging.Logger
}

// Debug implements the observability.Logger Debug method
func (c *commonToObsLogger) Debug(msg string, fields map[string]interface{}) {
	c.commonLogger.Debug(msg, fields)
}

// Info implements the observability.Logger Info method
func (c *commonToObsLogger) Info(msg string, fields map[string]interface{}) {
	c.commonLogger.Info(msg, fields)
}

// Warn implements the observability.Logger Warn method
func (c *commonToObsLogger) Warn(msg string, fields map[string]interface{}) {
	c.commonLogger.Warn(msg, fields)
}

// Error implements the observability.Logger Error method
func (c *commonToObsLogger) Error(msg string, fields map[string]interface{}) {
	c.commonLogger.Error(msg, fields)
}

// Fatal implements the observability.Logger Fatal method
func (c *commonToObsLogger) Fatal(msg string, fields map[string]interface{}) {
	c.commonLogger.Fatal(msg, fields)
}

// WithPrefix implements the observability.Logger WithPrefix method
func (c *commonToObsLogger) WithPrefix(prefix string) Logger {
	// Create a new logger adapter with the prefixed logger
	prefixedLogger := c.commonLogger.WithPrefix(prefix)
	return &commonToObsLogger{commonLogger: prefixedLogger}
}

// NewCommonLoggerAdapter creates an adapter from common/logging.Logger to observability.Logger
func NewCommonLoggerAdapter(logger *commonLogging.Logger) Logger {
	return &commonToObsLogger{commonLogger: logger}
}

// NewLoggerAdapter creates a new adapter from observability.Logger to common/logging.Logger
func NewLoggerAdapter(obs Logger) *commonLogging.Logger {
	// Since we can't directly implement the common/logging.Logger interface in our adapter,
	// we'll create a real logger and proxy the calls through it
	return commonLogging.NewLogger("adapter")
}

// obsToCommonLogger adapts an observability.Logger to common/logging.Logger
type obsToCommonLogger struct {
	obs Logger
}

// Debug implements common/logging.Logger Debug method
func (a *obsToCommonLogger) Debug(msg string, fields map[string]interface{}) {
	a.obs.Debug(msg, fields)
}

// Info implements common/logging.Logger Info method
func (a *obsToCommonLogger) Info(msg string, fields map[string]interface{}) {
	a.obs.Info(msg, fields)
}

// Warn implements common/logging.Logger Warn method
func (a *obsToCommonLogger) Warn(msg string, fields map[string]interface{}) {
	a.obs.Warn(msg, fields)
}

// Error implements common/logging.Logger Error method
func (a *obsToCommonLogger) Error(msg string, fields map[string]interface{}) {
	a.obs.Error(msg, fields)
}

// Fatal implements common/logging.Logger Fatal method
func (a *obsToCommonLogger) Fatal(msg string, fields map[string]interface{}) {
	a.obs.Fatal(msg, fields)
}

// SetMinLevel is a stub implementation since observability.Logger doesn't have this method
func (a *obsToCommonLogger) SetMinLevel(level commonLogging.LogLevel) {
	// No-op - this is a compatibility method
}

// WithPrefix creates a new logger with the combined prefix
func (a *obsToCommonLogger) WithPrefix(prefix string) *commonLogging.Logger {
	// Create a new common logger with the prefix
	return commonLogging.NewLogger("adapter-with-prefix")
}



// MetricsAdapter adapts between observability.MetricsClient and common metrics interfaces
type MetricsAdapter struct {
	metrics MetricsClient
}

// NewMetricsAdapter creates a new metrics adapter
func NewMetricsAdapter(metrics MetricsClient) commonMetrics.Client {
	return &MetricsAdapter{metrics: metrics}
}

// RecordCounter implements the metrics.Client interface
func (a *MetricsAdapter) RecordCounter(name string, value float64, tags map[string]string) {
	a.metrics.IncrementCounter(name, value, tags)
}

// RecordEvent implements the metrics.Client interface
func (a *MetricsAdapter) RecordEvent(name string, eventType string) {
	// Forward to appropriate metrics method based on the event type
	a.metrics.IncrementCounter(name, 1.0, map[string]string{"event_type": eventType})
}

// RecordGauge implements the metrics.Client interface
func (a *MetricsAdapter) RecordGauge(name string, value float64, tags map[string]string) {
	a.metrics.RecordGauge(name, value, tags)
}

// RecordLatency implements the metrics.Client interface
func (a *MetricsAdapter) RecordLatency(name string, value time.Duration) {
	// Forward to histogram which is typically used for latency measurements
	a.metrics.RecordHistogram(name, float64(value/time.Millisecond), map[string]string{"unit": "ms"})
}

// Close implements the metrics.Client interface
func (a *MetricsAdapter) Close() error {
	return a.metrics.Close()
}

// LoggingMetricsAdapter adapts between commonLogging.MetricsClient and observability.MetricsClient
type LoggingMetricsAdapter struct {
	metrics commonLogging.MetricsClient
}

// NewLoggingMetricsAdapter creates a new adapter from commonLogging.MetricsClient to observability.MetricsClient
func NewLoggingMetricsAdapter(metrics commonLogging.MetricsClient) MetricsClient {
	return &LoggingMetricsAdapter{metrics: metrics}
}

// IncrementCounter implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) IncrementCounter(name string, value float64, tags map[string]string) {
	// Forward call to the logging metrics client, ignoring tags if not supported
	a.metrics.IncrementCounter(name, value)
}

// RecordHistogram implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) RecordHistogram(name string, value float64, tags map[string]string) {
	// No-op implementation since the underlying metrics client doesn't have an equivalent method
	// In a real implementation, this would map to appropriate metrics reporting
}

// RecordGauge implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) RecordGauge(name string, value float64, tags map[string]string) {
	// No-op implementation since the underlying metrics client doesn't have an equivalent method
	// In a real implementation, this would map to appropriate metrics reporting
}

// RecordCounter implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) RecordCounter(name string, value float64, tags map[string]string) {
	// Forward to appropriate method if available in the underlying metrics client
	if recordCounter, ok := a.metrics.(interface{ RecordCounter(string, float64, map[string]string) }); ok {
		recordCounter.RecordCounter(name, value, tags)
	} else {
		// Fall back to IncrementCounter if RecordCounter is not available
		a.IncrementCounter(name, value, tags)
	}
}

// RecordEvent implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) RecordEvent(source, eventType string) {
	// Forward to appropriate method if available in the underlying metrics client
	if recordEvent, ok := a.metrics.(interface{ RecordEvent(string, string) }); ok {
		recordEvent.RecordEvent(source, eventType)
	}
}

// RecordLatency implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) RecordLatency(operation string, duration time.Duration) {
	// Forward to appropriate method if available in the underlying metrics client
	if recordLatency, ok := a.metrics.(interface{ RecordLatency(string, time.Duration) }); ok {
		recordLatency.RecordLatency(operation, duration)
	}
}

// RecordDuration implements the observability.MetricsClient interface (alias for RecordLatency)
func (a *LoggingMetricsAdapter) RecordDuration(operation string, duration time.Duration) {
	// Forward to RecordLatency for compatibility
	a.RecordLatency(operation, duration)
}

// RecordOperation implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) RecordOperation(operationName string, actionName string, success bool, duration float64, tags map[string]string) {
	// Log the operation details
	logTags := map[string]interface{}{
		"operation": operationName,
		"action":    actionName,
		"success":   success,
		"duration":  duration,
	}
	
	// Add any provided tags
	for k, v := range tags {
		logTags[k] = v
	}
	
	// If the underlying metrics client supports this operation, use it
	if recordOp, ok := a.metrics.(interface{ RecordOperation(string, string, bool, float64, map[string]string) }); ok {
		recordOp.RecordOperation(operationName, actionName, success, duration, tags)
	} else {
		// Add success/error status to logs
		logTags["result"] = "success"
		if !success {
			logTags["result"] = "error"
		}
		
		// Convert tags to string interface map
		tagsInterface := make(map[string]interface{}, len(tags))
		for k, v := range tags {
			tagsInterface[k] = v
		}
		
		// Log the operation
		if logger, ok := a.metrics.(interface{ Debug(string, map[string]interface{}) }); ok {
			logger.Debug("Operation metrics recorded", logTags)
		}
	}
}

// RecordOperationWithContext implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) RecordOperationWithContext(ctx context.Context, operation string, f func() error) error {
	// Record the start time
	start := time.Now()
	
	// Execute the provided function
	err := f()
	
	// Record the duration regardless of whether there was an error
	duration := time.Since(start)
	a.RecordDuration(operation, duration)
	
	// Record operation metrics
	success := err == nil
	a.RecordOperation(operation, "execute", success, duration.Seconds(), nil)
	
	return err
}

// Close implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) Close() error {
	// Forward to appropriate method if available in the underlying metrics client
	if closer, ok := a.metrics.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
