package observability

import (
	"time"

	commonLogging "github.com/S-Corkum/devops-mcp/pkg/common/logging"
	commonMetrics "github.com/S-Corkum/devops-mcp/pkg/observability"
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
	// In a real implementation, we would create a type that implements commonLogging.Logger
	// and forwards calls to our observer logger, but for now we return a new logger
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

// WithPrefix implements common/logging.Logger WithPrefix method
func (a *obsToCommonLogger) WithPrefix(prefix string) *commonLogging.Logger {
	// Get a new logger with the prefix
	prefixedLogger := a.obs.WithPrefix(prefix)
	
	// Create a new adapter
	return NewLoggerAdapter(prefixedLogger)
}

// SetMinLevel is a stub implementation since observability.Logger doesn't have this method
func (a *obsToCommonLogger) SetMinLevel(level commonLogging.LogLevel) {
	// This is a no-op as the observability.Logger interface doesn't have a SetMinLevel method
}

// MetricsAdapter is an adapter from observability.MetricsClient to commonMetrics.Client
type MetricsAdapter struct {
	metrics MetricsClient
}

// NewMetricsAdapter creates a new adapter from observability.MetricsClient to commonMetrics.Client
func NewMetricsAdapter(metrics MetricsClient) commonMetrics.Client {
	return &MetricsAdapter{metrics: metrics}
}

// IncrementCounter implements the commonMetrics.Client interface
func (a *MetricsAdapter) IncrementCounter(name string, value float64, tags map[string]string) {
	a.metrics.IncrementCounter(name, value, tags)
}

// RecordCounter implements the commonMetrics.Client interface
func (a *MetricsAdapter) RecordCounter(name string, value float64, tags map[string]string) {
	a.metrics.RecordCounter(name, value, tags)
}

// RecordEvent implements the commonMetrics.Client interface
func (a *MetricsAdapter) RecordEvent(source, eventType string) {
	a.metrics.RecordEvent(source, eventType)
}

// RecordGauge implements the commonMetrics.Client interface
func (a *MetricsAdapter) RecordGauge(name string, value float64, tags map[string]string) {
	a.metrics.RecordGauge(name, value, tags)
}

// RecordHistogram implements the commonMetrics.Client interface
func (a *MetricsAdapter) RecordHistogram(name string, value float64, tags map[string]string) {
	a.metrics.RecordHistogram(name, value, tags)
}

// RecordLatency implements the commonMetrics.Client interface
func (a *MetricsAdapter) RecordLatency(operation string, duration time.Duration) {
	a.metrics.RecordLatency(operation, duration)
}

// RecordTimer implements the commonMetrics.Client interface
func (a *MetricsAdapter) RecordTimer(name string, duration time.Duration, tags map[string]string) {
	a.metrics.RecordTimer(name, duration, tags)
}

// RecordDuration implements the commonMetrics.Client interface
func (a *MetricsAdapter) RecordDuration(name string, d time.Duration) {
	a.metrics.RecordDuration(name, d)
}

// RecordOperation implements the commonMetrics.Client interface
func (a *MetricsAdapter) RecordOperation(operation string, success bool, d time.Duration) {
	// Convert to what our metrics client expects
	durationSeconds := d.Seconds()
	component := "common" // Use a default component name
	
	// Use empty labels as default
	labels := map[string]string{}
	
	a.metrics.RecordOperation(component, operation, success, durationSeconds, labels)
}

// Close implements the commonMetrics.Client interface
func (a *MetricsAdapter) Close() error {
	return a.metrics.Close()
}

// LoggingMetricsAdapter is an adapter that uses the Logger interface to log metrics
type LoggingMetricsAdapter struct {
	logger  Logger
	metrics interface{} // Could be Logger or another MetricsClient
}

// NewLoggingMetricsAdapter creates a new adapter
func NewLoggingMetricsAdapter(logger Logger) MetricsClient {
	return &LoggingMetricsAdapter{logger: logger, metrics: logger}
}

// NewLoggingMetricsAdapterWithMetrics creates a new adapter with both logger and metrics client
func NewLoggingMetricsAdapterWithMetrics(logger Logger, metrics interface{}) MetricsClient {
	return &LoggingMetricsAdapter{logger: logger, metrics: metrics}
}

// IncrementCounter increments a counter metric by a given value
func (a *LoggingMetricsAdapter) IncrementCounter(name string, value float64, labels map[string]string) {
	// If the underlying metrics client supports this operation, use it
	if counter, ok := a.metrics.(interface{ IncrementCounter(string, float64, map[string]string) }); ok {
		counter.IncrementCounter(name, value, labels)
		return
	}
	
	// Otherwise, log the operation
	logTags := map[string]interface{}{
		"metric": name,
		"value":  value,
	}
	
	// Add all labels to log tags
	for k, v := range labels {
		logTags[k] = v
	}
	
	a.logger.Debug("Incrementing counter", logTags)
}

// RecordDuration records a duration metric
func (a *LoggingMetricsAdapter) RecordDuration(name string, duration time.Duration) {
	// If the underlying metrics client supports this operation, use it
	if recorder, ok := a.metrics.(interface{ RecordDuration(string, time.Duration) }); ok {
		recorder.RecordDuration(name, duration)
		return
	}
	
	// Otherwise, log the operation
	logTags := map[string]interface{}{
		"metric":   name,
		"duration": duration.String(),
	}
	
	a.logger.Debug("Recording duration", logTags)
}

// RecordEvent records an event metric
func (a *LoggingMetricsAdapter) RecordEvent(source, eventType string) {
	// If the underlying metrics client supports this operation, use it
	if recorder, ok := a.metrics.(interface{ RecordEvent(string, string) }); ok {
		recorder.RecordEvent(source, eventType)
		return
	}
	
	// Otherwise, log the operation
	logTags := map[string]interface{}{
		"source":     source,
		"event_type": eventType,
	}
	
	a.logger.Debug("Recording event", logTags)
}

// RecordLatency records a latency metric
func (a *LoggingMetricsAdapter) RecordLatency(operation string, duration time.Duration) {
	// If the underlying metrics client supports this operation, use it
	if recorder, ok := a.metrics.(interface{ RecordLatency(string, time.Duration) }); ok {
		recorder.RecordLatency(operation, duration)
		return
	}
	
	// Otherwise, log the operation
	logTags := map[string]interface{}{
		"operation": operation,
		"latency":   duration.String(),
	}
	
	a.logger.Debug("Recording latency", logTags)
}

// RecordCounter records a counter metric
func (a *LoggingMetricsAdapter) RecordCounter(name string, value float64, labels map[string]string) {
	// If the underlying metrics client supports this operation, use it
	if recorder, ok := a.metrics.(interface{ RecordCounter(string, float64, map[string]string) }); ok {
		recorder.RecordCounter(name, value, labels)
		return
	}
	
	// Otherwise, log the operation
	logTags := map[string]interface{}{
		"metric": name,
		"value":  value,
	}
	
	// Add all labels to log tags
	for k, v := range labels {
		logTags[k] = v
	}
	
	a.logger.Debug("Recording counter", logTags)
}

// RecordGauge records a gauge metric
func (a *LoggingMetricsAdapter) RecordGauge(name string, value float64, labels map[string]string) {
	// If the underlying metrics client supports this operation, use it
	if recorder, ok := a.metrics.(interface{ RecordGauge(string, float64, map[string]string) }); ok {
		recorder.RecordGauge(name, value, labels)
		return
	}
	
	// Otherwise, log the operation
	logTags := map[string]interface{}{
		"metric": name,
		"value":  value,
	}
	
	// Add all labels to log tags
	for k, v := range labels {
		logTags[k] = v
	}
	
	a.logger.Debug("Recording gauge", logTags)
}

// RecordHistogram records a histogram metric
func (a *LoggingMetricsAdapter) RecordHistogram(name string, value float64, labels map[string]string) {
	// If the underlying metrics client supports this operation, use it
	if recorder, ok := a.metrics.(interface{ RecordHistogram(string, float64, map[string]string) }); ok {
		recorder.RecordHistogram(name, value, labels)
		return
	}
	
	// Otherwise, log the operation
	logTags := map[string]interface{}{
		"metric": name,
		"value":  value,
	}
	
	// Add all labels to log tags
	for k, v := range labels {
		logTags[k] = v
	}
	
	a.logger.Debug("Recording histogram", logTags)
}

// RecordTimer records a timer metric
func (a *LoggingMetricsAdapter) RecordTimer(name string, duration time.Duration, labels map[string]string) {
	// If the underlying metrics client supports this operation, use it
	if recorder, ok := a.metrics.(interface{ RecordTimer(string, time.Duration, map[string]string) }); ok {
		recorder.RecordTimer(name, duration, labels)
		return
	}
	
	// Otherwise, log the operation
	logTags := map[string]interface{}{
		"metric":   name,
		"duration": duration.String(),
	}
	
	// Add all labels to log tags
	for k, v := range labels {
		logTags[k] = v
	}
	
	a.logger.Debug("Recording timer", logTags)
}

// RecordOperation records operation metrics for adapters and other components
func (a *LoggingMetricsAdapter) RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string) {
	// If the underlying metrics client supports this operation, use it
	if recorder, ok := a.metrics.(interface {
		RecordOperation(string, string, bool, float64, map[string]string)
	}); ok {
		recorder.RecordOperation(component, operation, success, durationSeconds, labels)
		return
	}
	
	// Otherwise, log the operation
	logTags := map[string]interface{}{
		"component": component,
		"operation": operation,
		"success":   success,
		"duration":  durationSeconds,
	}
	
	// Copy labels to log tags, but convert to interface{}
	for k, v := range labels {
		logTags[k] = v
	}
	
	// Log the operation
	if success {
		a.logger.Info(component+"."+operation+" completed", logTags)
	} else {
		a.logger.Error(component+"."+operation+" failed", logTags)
	}
}

// StartTimer starts a timer metric and returns a function to stop it
func (a *LoggingMetricsAdapter) StartTimer(name string, labels map[string]string) func() {
	// If the underlying metrics client supports this operation, use it
	if starter, ok := a.metrics.(interface{ StartTimer(string, map[string]string) func() }); ok {
		return starter.StartTimer(name, labels)
	}
	
	// Otherwise, implement it directly
	startTime := time.Now()
	return func() {
		duration := time.Since(startTime)
		a.RecordTimer(name, duration, labels)
	}
}

// RecordAPIOperation implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) RecordAPIOperation(api string, operation string, success bool, durationSeconds float64) {
	// If the underlying metrics client supports this operation, use it
	if recorder, ok := a.metrics.(interface {
		RecordAPIOperation(string, string, bool, float64)
	}); ok {
		recorder.RecordAPIOperation(api, operation, success, durationSeconds)
		return
	}
	
	// Otherwise, log the operation
	logTags := map[string]interface{}{
		"api":       api,
		"operation": operation,
		"success":   success,
		"duration":  durationSeconds,
	}
	
	// Log the operation
	if success {
		a.logger.Info("API operation completed: "+api+"."+operation, logTags)
	} else {
		a.logger.Error("API operation failed: "+api+"."+operation, logTags)
	}
}

// RecordCacheOperation implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) RecordCacheOperation(operation string, success bool, durationSeconds float64) {
	// If the underlying metrics client supports this operation, use it
	if recorder, ok := a.metrics.(interface {
		RecordCacheOperation(string, bool, float64)
	}); ok {
		recorder.RecordCacheOperation(operation, success, durationSeconds)
		return
	}
	
	// Otherwise, log the operation
	logTags := map[string]interface{}{
		"operation": operation,
		"success":   success,
		"duration":  durationSeconds,
	}
	
	// Log the operation
	if success {
		a.logger.Info("Cache operation completed: "+operation, logTags)
	} else {
		a.logger.Error("Cache operation failed: "+operation, logTags)
	}
}

// RecordDatabaseOperation implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {
	// If the underlying metrics client supports this operation, use it
	if recorder, ok := a.metrics.(interface {
		RecordDatabaseOperation(string, bool, float64)
	}); ok {
		recorder.RecordDatabaseOperation(operation, success, durationSeconds)
		return
	}
	
	// Otherwise, log the operation
	logTags := map[string]interface{}{
		"operation": operation,
		"success":   success,
		"duration":  durationSeconds,
	}
	
	// Log the operation
	if success {
		a.logger.Info("Database operation completed: "+operation, logTags)
	} else {
		a.logger.Error("Database operation failed: "+operation, logTags)
	}
}

// Close implements the observability.MetricsClient interface
func (a *LoggingMetricsAdapter) Close() error {
	// Forward to appropriate method if available in the underlying metrics client
	if closer, ok := a.metrics.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
