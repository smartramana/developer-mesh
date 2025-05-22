// Package observability provides unified observability functionality for the MCP system.
package observability

import (
	"time"
)

// LegacyMetricsAdapter adapts the observability.MetricsClient to legacy metrics interfaces
// This allows existing code using either internal/metrics or pkg/common/metrics to work with
// the new observability package without modifications.
type LegacyMetricsAdapter struct {
	metrics MetricsClient
}

// Define local interface types to avoid circular imports

// LegacyClient represents the common metrics client interface used by both internal and pkg clients
type LegacyClient interface {
	RecordEvent(source, eventType string)
	RecordLatency(operation string, duration time.Duration)
	RecordCounter(name string, value float64, labels map[string]string)
	RecordGauge(name string, value float64, labels map[string]string)
	Close() error
}

// LegacyConfig represents the legacy metrics configuration format
type LegacyConfig struct {
	Enabled      bool
	Type         string
	Endpoint     string
	PushGateway  string
	PushInterval time.Duration
}

// NewLegacyMetricsAdapter creates a new adapter for the MetricsClient
// This can be used as a replacement for both internal/metrics.Client and pkg/common/metrics.Client
func NewLegacyMetricsAdapter(metrics MetricsClient) *LegacyMetricsAdapter {
	return &LegacyMetricsAdapter{metrics: metrics}
}

// NewInternalMetricsClient creates an adapter that implements the legacy metrics Client interface
func NewInternalMetricsClient(metrics MetricsClient) LegacyClient {
	return &LegacyMetricsAdapter{metrics: metrics}
}

// NewCommonMetricsClient creates an adapter that implements the legacy metrics Client interface
func NewCommonMetricsClient(metrics MetricsClient) LegacyClient {
	return &LegacyMetricsAdapter{metrics: metrics}
}

// RecordEvent records an event metric
func (a *LegacyMetricsAdapter) RecordEvent(source, eventType string) {
	a.metrics.RecordEvent(source, eventType)
}

// RecordLatency records a latency metric
func (a *LegacyMetricsAdapter) RecordLatency(operation string, duration time.Duration) {
	a.metrics.RecordLatency(operation, duration)
}

// RecordCounter increments a counter metric
func (a *LegacyMetricsAdapter) RecordCounter(name string, value float64, labels map[string]string) {
	a.metrics.RecordCounter(name, value, labels)
}

// RecordGauge sets a gauge metric
func (a *LegacyMetricsAdapter) RecordGauge(name string, value float64, labels map[string]string) {
	a.metrics.RecordGauge(name, value, labels)
}

// Close closes the metrics client
func (a *LegacyMetricsAdapter) Close() error {
	return a.metrics.Close()
}

// CreateLegacyConfig creates a legacy Config object from the observability.MetricsConfig
func CreateLegacyConfig(cfg MetricsConfig) LegacyConfig {
	return LegacyConfig{
		Enabled:      cfg.Enabled,
		Type:         cfg.Type,
		Endpoint:     cfg.Endpoint,
		PushGateway:  cfg.PushGateway,
		PushInterval: cfg.PushInterval,
	}
}

// ConvertFromLegacyConfig converts a legacy config to observability.MetricsConfig
func ConvertFromLegacyConfig(cfg LegacyConfig) MetricsConfig {
	return MetricsConfig{
		Enabled:      cfg.Enabled,
		Type:         cfg.Type,
		Endpoint:     cfg.Endpoint,
		PushGateway:  cfg.PushGateway,
		PushInterval: cfg.PushInterval,
	}
}

// NewClientFromLegacyConfig creates a new observability.MetricsClient from a legacy config
func NewClientFromLegacyConfig(cfg LegacyConfig) MetricsClient {
	obsConfig := ConvertFromLegacyConfig(cfg)
	return NewMetricsClientWithOptions(MetricsOptions{
		Enabled: obsConfig.Enabled,
		Labels:  map[string]string{},
	})
}

