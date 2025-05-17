package cache

// MetricsClient is a temporary stub replacement for the observability.MetricsClient
// This will be used until the observability package dependencies are resolved
type MetricsClient struct{}

// NewMetricsClient creates a new metrics client
func NewMetricsClient() *MetricsClient {
	return &MetricsClient{}
}

// IncrementCounter increments a counter metric
func (m *MetricsClient) IncrementCounter(name string, value float64, tags map[string]string) {
	// Stub implementation - no-op for now
}

// RecordHistogram records a histogram metric
func (m *MetricsClient) RecordHistogram(name string, value float64, tags map[string]string) {
	// Stub implementation - no-op for now
}

// RecordGauge records a gauge metric
func (m *MetricsClient) RecordGauge(name string, value float64, tags map[string]string) {
	// Stub implementation - no-op for now
}

// RecordCacheOperation records a cache operation metric
func (m *MetricsClient) RecordCacheOperation(operation string, hit bool, latencySeconds float64) {
	// Stub implementation - no-op for now
}
