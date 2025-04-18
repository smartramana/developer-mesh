package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Context operation metrics
	contextOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_context_operations_total",
			Help: "Number of context operations by operation type and model ID",
		},
		[]string{"operation", "model_id"},
	)

	contextOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_context_operation_duration_seconds",
			Help:    "Duration of context operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // From 1ms to ~16s
		},
		[]string{"operation", "model_id"},
	)

	contextTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_context_tokens_total",
			Help: "Number of tokens processed in contexts by model ID",
		},
		[]string{"model_id"},
	)

	contextSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_context_size_tokens",
			Help:    "Size of contexts in tokens",
			Buckets: prometheus.ExponentialBuckets(10, 2, 15), // From 10 to ~160k tokens
		},
		[]string{"model_id"},
	)

	// Vector/embedding metrics
	vectorOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_vector_operations_total",
			Help: "Number of vector operations by operation type",
		},
		[]string{"operation"},
	)

	vectorOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_vector_operation_duration_seconds",
			Help:    "Duration of vector operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // From 1ms to ~16s
		},
		[]string{"operation"},
	)

	// Tool operation metrics
	toolOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_tool_operations_total",
			Help: "Number of tool operations by tool name and action",
		},
		[]string{"tool", "action"},
	)

	toolOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_tool_operation_duration_seconds",
			Help:    "Duration of tool operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // From 1ms to ~16s
		},
		[]string{"tool", "action"},
	)

	toolOperationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_tool_operation_errors_total",
			Help: "Number of tool operation errors by tool name and action",
		},
		[]string{"tool", "action", "error_type"},
	)

	// API metrics
	apiRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_api_requests_total",
			Help: "Number of API requests by endpoint and method",
		},
		[]string{"endpoint", "method", "status"},
	)

	apiRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_api_request_duration_seconds",
			Help:    "Duration of API requests in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // From 1ms to ~16s
		},
		[]string{"endpoint", "method"},
	)

	// Cache metrics
	cacheOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_cache_operations_total",
			Help: "Number of cache operations by operation type",
		},
		[]string{"operation", "result"},
	)

	cacheOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_cache_operation_duration_seconds",
			Help:    "Duration of cache operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.0001, 10, 5), // From 0.1ms to 100ms
		},
		[]string{"operation"},
	)
)

// MetricsClient provides methods for recording metrics
type MetricsClient struct {}

// NewMetricsClient creates a new metrics client
func NewMetricsClient() *MetricsClient {
	return &MetricsClient{}
}

// RecordContextOperation records a context operation
func (c *MetricsClient) RecordContextOperation(operation, modelID string, durationSeconds float64, tokenCount int) {
	contextOperations.WithLabelValues(operation, modelID).Inc()
	contextOperationDuration.WithLabelValues(operation, modelID).Observe(durationSeconds)
	if tokenCount > 0 {
		contextTokens.WithLabelValues(modelID).Add(float64(tokenCount))
		contextSize.WithLabelValues(modelID).Observe(float64(tokenCount))
	}
}

// RecordVectorOperation records a vector operation
func (c *MetricsClient) RecordVectorOperation(operation string, durationSeconds float64) {
	vectorOperations.WithLabelValues(operation).Inc()
	vectorOperationDuration.WithLabelValues(operation).Observe(durationSeconds)
}

// RecordToolOperation records a tool operation
func (c *MetricsClient) RecordToolOperation(tool, action string, durationSeconds float64, err error) {
	toolOperations.WithLabelValues(tool, action).Inc()
	toolOperationDuration.WithLabelValues(tool, action).Observe(durationSeconds)
	
	if err != nil {
		errorType := "unknown"
		if err.Error() == "not found" {
			errorType = "not_found"
		} else if err.Error() == "unauthorized" {
			errorType = "unauthorized"
		} else if err.Error() == "invalid_input" {
			errorType = "invalid_input"
		}
		toolOperationErrors.WithLabelValues(tool, action, errorType).Inc()
	}
}

// RecordAPIRequest records an API request
func (c *MetricsClient) RecordAPIRequest(endpoint, method, status string, durationSeconds float64) {
	apiRequests.WithLabelValues(endpoint, method, status).Inc()
	apiRequestDuration.WithLabelValues(endpoint, method).Observe(durationSeconds)
}

// RecordCacheOperation records a cache operation
func (c *MetricsClient) RecordCacheOperation(operation string, hit bool, durationSeconds float64) {
	result := "miss"
	if hit {
		result = "hit"
	}
	cacheOperations.WithLabelValues(operation, result).Inc()
	cacheOperationDuration.WithLabelValues(operation).Observe(durationSeconds)
}
