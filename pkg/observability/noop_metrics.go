package observability

import "time"

// noOpMetricsClient is a no-op implementation of MetricsClient for testing
type noOpMetricsClient struct{}

// NewNoOpMetricsClient creates a new no-op metrics client that does nothing
func NewNoOpMetricsClient() MetricsClient {
	return &noOpMetricsClient{}
}

// RecordEvent is a no-op implementation
func (n *noOpMetricsClient) RecordEvent(source, eventType string) {}

// RecordLatency is a no-op implementation
func (n *noOpMetricsClient) RecordLatency(operation string, duration time.Duration) {}

// RecordCounter is a no-op implementation
func (n *noOpMetricsClient) RecordCounter(name string, value float64, labels map[string]string) {}

// RecordGauge is a no-op implementation
func (n *noOpMetricsClient) RecordGauge(name string, value float64, labels map[string]string) {}

// RecordHistogram is a no-op implementation
func (n *noOpMetricsClient) RecordHistogram(name string, value float64, labels map[string]string) {}

// RecordTimer is a no-op implementation
func (n *noOpMetricsClient) RecordTimer(name string, duration time.Duration, labels map[string]string) {}

// RecordCacheOperation is a no-op implementation
func (n *noOpMetricsClient) RecordCacheOperation(operation string, success bool, durationSeconds float64) {}

// RecordOperation is a no-op implementation
func (n *noOpMetricsClient) RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string) {
}

// RecordAPIOperation is a no-op implementation
func (n *noOpMetricsClient) RecordAPIOperation(api string, operation string, success bool, durationSeconds float64) {
}

// RecordDatabaseOperation is a no-op implementation
func (n *noOpMetricsClient) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {}

// StartTimer is a no-op implementation
func (n *noOpMetricsClient) StartTimer(name string, labels map[string]string) func() {
	return func() {}
}

// IncrementCounter is a no-op implementation
func (n *noOpMetricsClient) IncrementCounter(name string, value float64) {}

// IncrementCounterWithLabels is a no-op implementation
func (n *noOpMetricsClient) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {}

// RecordDuration is a no-op implementation
func (n *noOpMetricsClient) RecordDuration(name string, duration time.Duration) {}

// Close is a no-op implementation
func (n *noOpMetricsClient) Close() error {
	return nil
}