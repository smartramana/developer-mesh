// Package observability provides unified observability functionality for the MCP system.
package observability

import (
	"time"

	commonMetrics "github.com/S-Corkum/devops-mcp/pkg/observability"
	internalMetrics "github.com/S-Corkum/devops-mcp/pkg/observability"
)

// LegacyMetricsAdapter adapts the observability.MetricsClient to the legacy metrics.Client interface
// This allows existing code using either internal/metrics or pkg/common/metrics to work with
// the new observability package without modifications.
type LegacyMetricsAdapter struct {
	metrics MetricsClient
}

// NewLegacyMetricsAdapter creates a new adapter for the MetricsClient
// This can be used as a replacement for both internal/metrics.Client and pkg/common/metrics.Client
func NewLegacyMetricsAdapter(metrics MetricsClient) *LegacyMetricsAdapter {
	return &LegacyMetricsAdapter{metrics: metrics}
}

// NewInternalMetricsClient creates an adapter that implements internal/metrics.Client
func NewInternalMetricsClient(metrics MetricsClient) internalMetrics.Client {
	return &LegacyMetricsAdapter{metrics: metrics}
}

// NewCommonMetricsClient creates an adapter that implements pkg/common/metrics.Client
func NewCommonMetricsClient(metrics MetricsClient) commonMetrics.Client {
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

// CreateConfigAdapter creates a Config object from the observability.MetricsConfig
// for backward compatibility with internal/metrics
func CreateConfigAdapter(cfg MetricsConfig) internalMetrics.Config {
	return internalMetrics.Config{
		Enabled:      cfg.Enabled,
		Type:         cfg.Type,
		Endpoint:     cfg.Endpoint,
		PushGateway:  cfg.PushGateway,
		PushInterval: cfg.PushInterval,
	}
}

// CreateCommonConfigAdapter creates a metrics.Config object from the observability.MetricsConfig
// for backward compatibility with pkg/common/metrics
func CreateCommonConfigAdapter(cfg MetricsConfig) commonMetrics.Config {
	return commonMetrics.Config{
		Enabled:      cfg.Enabled,
		Type:         cfg.Type,
		Endpoint:     cfg.Endpoint,
		PushGateway:  cfg.PushGateway,
		PushInterval: cfg.PushInterval,
	}
}

// ConvertFromInternalConfig converts an internal/metrics.Config to observability.MetricsConfig
func ConvertFromInternalConfig(cfg internalMetrics.Config) MetricsConfig {
	return MetricsConfig{
		Enabled:      cfg.Enabled,
		Type:         cfg.Type,
		Endpoint:     cfg.Endpoint,
		PushGateway:  cfg.PushGateway,
		PushInterval: cfg.PushInterval,
	}
}

// ConvertFromCommonConfig converts a pkg/common/metrics.Config to observability.MetricsConfig
func ConvertFromCommonConfig(cfg commonMetrics.Config) MetricsConfig {
	return MetricsConfig{
		Enabled:      cfg.Enabled,
		Type:         cfg.Type,
		Endpoint:     cfg.Endpoint,
		PushGateway:  cfg.PushGateway,
		PushInterval: cfg.PushInterval,
	}
}

// Factory functions that mimic the original constructors

// NewClientFromInternal creates a new observability.MetricsClient from an internal/metrics.Config
func NewClientFromInternal(cfg internalMetrics.Config) MetricsClient {
	obsConfig := ConvertFromInternalConfig(cfg)
	return NewMetricsClientWithOptions(MetricsOptions{
		Enabled: obsConfig.Enabled,
		Labels:  map[string]string{},
	})
}

// NewClientFromCommon creates a new observability.MetricsClient from a pkg/common/metrics.Config
func NewClientFromCommon(cfg commonMetrics.Config) MetricsClient {
	obsConfig := ConvertFromCommonConfig(cfg)
	return NewMetricsClientWithOptions(MetricsOptions{
		Enabled: obsConfig.Enabled,
		Labels:  map[string]string{},
	})
}
