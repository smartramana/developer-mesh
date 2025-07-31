package webhook

import (
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ContextLifecycleMetrics provides metrics for context lifecycle operations
type ContextLifecycleMetrics struct {
	metrics observability.MetricsClient
}

// NewContextLifecycleMetrics creates a new metrics instance
func NewContextLifecycleMetrics(metrics observability.MetricsClient) *ContextLifecycleMetrics {
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}
	return &ContextLifecycleMetrics{
		metrics: metrics,
	}
}

// Lock acquisition metrics
func (m *ContextLifecycleMetrics) RecordLockAcquisition(tenantID string, success bool, duration time.Duration) {
	labels := map[string]string{
		"tenant_id": tenantID,
		"success":   fmt.Sprintf("%t", success),
	}

	// Record acquisition time
	m.metrics.RecordHistogram("context.lock.acquisition_time", duration.Seconds(), labels)

	// Increment counters
	if success {
		m.metrics.IncrementCounterWithLabels("context.lock.acquired", 1, labels)
	} else {
		m.metrics.IncrementCounterWithLabels("context.lock.failed", 1, labels)
	}
}

// Record lock hold time
func (m *ContextLifecycleMetrics) RecordLockHoldTime(tenantID string, duration time.Duration) {
	labels := map[string]string{
		"tenant_id": tenantID,
	}
	m.metrics.RecordHistogram("context.lock.hold_time", duration.Seconds(), labels)
}

// Record lock contention
func (m *ContextLifecycleMetrics) RecordLockContention(tenantID string, retries int) {
	labels := map[string]string{
		"tenant_id": tenantID,
	}
	m.metrics.RecordHistogram("context.lock.retry_count", float64(retries), labels)

	if retries > 0 {
		m.metrics.IncrementCounterWithLabels("context.lock.contention", 1, labels)
	}
}

// Batch processing metrics
func (m *ContextLifecycleMetrics) RecordBatchProcessing(transitionType string, batchSize int, duration time.Duration, successCount int, failureCount int) {
	labels := map[string]string{
		"transition_type": transitionType,
		"batch_size":      fmt.Sprintf("%d", batchSize),
	}

	// Record processing time
	m.metrics.RecordHistogram("context.batch.processing_time", duration.Seconds(), labels)

	// Record throughput (contexts per second)
	throughput := float64(successCount) / duration.Seconds()
	m.metrics.RecordGauge("context.batch.throughput", throughput, labels)

	// Record success/failure counts
	m.metrics.IncrementCounterWithLabels("context.batch.success", float64(successCount), labels)
	m.metrics.IncrementCounterWithLabels("context.batch.failure", float64(failureCount), labels)

	// Record batch efficiency
	if batchSize > 0 {
		efficiency := float64(successCount) / float64(batchSize) * 100
		m.metrics.RecordGauge("context.batch.efficiency_percent", efficiency, labels)
	}
}

// State transition metrics
func (m *ContextLifecycleMetrics) RecordStateTransition(fromState, toState ContextState, success bool, duration time.Duration) {
	labels := map[string]string{
		"from_state": string(fromState),
		"to_state":   string(toState),
		"success":    fmt.Sprintf("%t", success),
	}

	// Record transition time
	m.metrics.RecordHistogram("context.transition.duration", duration.Seconds(), labels)

	// Increment counter
	m.metrics.IncrementCounterWithLabels("context.transition.count", 1, labels)
}

// Cold storage metrics
func (m *ContextLifecycleMetrics) RecordColdStorageOperation(operation string, success bool, duration time.Duration, size int64) {
	labels := map[string]string{
		"operation": operation,
		"success":   fmt.Sprintf("%t", success),
	}

	// Record operation time
	m.metrics.RecordHistogram("context.cold_storage.operation_time", duration.Seconds(), labels)

	// Record data size
	if size > 0 {
		m.metrics.RecordHistogram("context.cold_storage.data_size", float64(size), labels)
	}

	// Increment counter
	m.metrics.IncrementCounterWithLabels("context.cold_storage.operations", 1, labels)
}

// Circuit breaker metrics
func (m *ContextLifecycleMetrics) RecordCircuitBreakerState(state string) {
	labels := map[string]string{
		"state": state,
	}
	m.metrics.IncrementCounterWithLabels("context.circuit_breaker.state_change", 1, labels)
}

// Search performance metrics
func (m *ContextLifecycleMetrics) RecordSearchPerformance(criteria string, resultCount int, duration time.Duration) {
	labels := map[string]string{
		"criteria_type": criteria,
	}

	// Record search time
	m.metrics.RecordHistogram("context.search.duration", duration.Seconds(), labels)

	// Record result count
	m.metrics.RecordHistogram("context.search.result_count", float64(resultCount), labels)
}

// Storage tier distribution
func (m *ContextLifecycleMetrics) UpdateStorageTierCounts(hotCount, warmCount, coldCount int64) {
	m.metrics.RecordGauge("context.storage.hot_count", float64(hotCount), nil)
	m.metrics.RecordGauge("context.storage.warm_count", float64(warmCount), nil)
	m.metrics.RecordGauge("context.storage.cold_count", float64(coldCount), nil)

	total := hotCount + warmCount + coldCount
	if total > 0 {
		m.metrics.RecordGauge("context.storage.hot_percent", float64(hotCount)/float64(total)*100, nil)
		m.metrics.RecordGauge("context.storage.warm_percent", float64(warmCount)/float64(total)*100, nil)
		m.metrics.RecordGauge("context.storage.cold_percent", float64(coldCount)/float64(total)*100, nil)
	}
}

// Compression metrics
func (m *ContextLifecycleMetrics) RecordCompressionRatio(ratio float64, originalSize, compressedSize int64) {
	m.metrics.RecordHistogram("context.compression.ratio", ratio, nil)
	m.metrics.RecordHistogram("context.compression.original_size", float64(originalSize), nil)
	m.metrics.RecordHistogram("context.compression.compressed_size", float64(compressedSize), nil)
	m.metrics.IncrementCounterWithLabels("context.compression.bytes_saved", float64(originalSize-compressedSize), nil)
}

// Pipeline execution metrics
func (m *ContextLifecycleMetrics) RecordPipelineExecution(commandCount int, success bool, duration time.Duration) {
	labels := map[string]string{
		"success": fmt.Sprintf("%t", success),
	}

	m.metrics.RecordHistogram("context.pipeline.execution_time", duration.Seconds(), labels)
	m.metrics.RecordHistogram("context.pipeline.command_count", float64(commandCount), labels)
	m.metrics.IncrementCounterWithLabels("context.pipeline.executions", 1, labels)
}
