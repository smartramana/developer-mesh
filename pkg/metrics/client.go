package metrics

import (
	"sync"
	"time"
)

// Config holds configuration for the metrics client
type Config struct {
	Enabled      bool          `mapstructure:"enabled"`
	Type         string        `mapstructure:"type"`
	Endpoint     string        `mapstructure:"endpoint"`
	PushGateway  string        `mapstructure:"push_gateway"`
	PushInterval time.Duration `mapstructure:"push_interval"`
}

// Client is the interface for metrics collection
type Client interface {
	// RecordEvent records an event metric
	RecordEvent(source, eventType string)

	// RecordLatency records a latency metric
	RecordLatency(operation string, duration time.Duration)

	// RecordCounter increments a counter metric
	RecordCounter(name string, value float64, labels map[string]string)

	// RecordGauge sets a gauge metric
	RecordGauge(name string, value float64, labels map[string]string)

	// Close closes the metrics client
	Close() error
}

// PrometheusClient implements the metrics client using Prometheus
type PrometheusClient struct {
	config  Config
	metrics map[string]interface{}
	mu      sync.RWMutex
}

// NewClient creates a new metrics client
func NewClient(cfg Config) Client {
	if !cfg.Enabled {
		return &NoopClient{}
	}

	switch cfg.Type {
	case "prometheus":
		return NewPrometheusClient(cfg)
	default:
		// Default to noop client
		return &NoopClient{}
	}
}

// NewPrometheusClient creates a new Prometheus metrics client
func NewPrometheusClient(cfg Config) *PrometheusClient {
	client := &PrometheusClient{
		config:  cfg,
		metrics: make(map[string]interface{}),
	}

	// Start pushing metrics to gateway if configured
	if cfg.PushGateway != "" {
		go client.startPushing(cfg.PushInterval)
	}

	return client
}

// RecordEvent records an event metric
func (c *PrometheusClient) RecordEvent(source, eventType string) {
	// In a real implementation, this would use Prometheus client library
	c.mu.Lock()
	defer c.mu.Unlock()

	key := "events_total_" + source + "_" + eventType
	count, ok := c.metrics[key]
	if !ok {
		c.metrics[key] = 1.0
	} else {
		c.metrics[key] = count.(float64) + 1.0
	}
}

// RecordLatency records a latency metric
func (c *PrometheusClient) RecordLatency(operation string, duration time.Duration) {
	// In a real implementation, this would use Prometheus client library
	c.mu.Lock()
	defer c.mu.Unlock()

	key := "latency_" + operation
	c.metrics[key] = float64(duration.Milliseconds())
}

// RecordCounter increments a counter metric
func (c *PrometheusClient) RecordCounter(name string, value float64, labels map[string]string) {
	// In a real implementation, this would use Prometheus client library
	c.mu.Lock()
	defer c.mu.Unlock()

	key := "counter_" + name
	count, ok := c.metrics[key]
	if !ok {
		c.metrics[key] = value
	} else {
		c.metrics[key] = count.(float64) + value
	}
}

// RecordGauge sets a gauge metric
func (c *PrometheusClient) RecordGauge(name string, value float64, labels map[string]string) {
	// In a real implementation, this would use Prometheus client library
	c.mu.Lock()
	defer c.mu.Unlock()

	key := "gauge_" + name
	c.metrics[key] = value
}

// startPushing starts pushing metrics to the gateway
func (c *PrometheusClient) startPushing(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		<-ticker.C
		c.pushMetrics()
	}
}

// pushMetrics pushes metrics to the gateway
func (c *PrometheusClient) pushMetrics() {
	// In a real implementation, this would use Prometheus client library
	// to push metrics to the push gateway
}

// Close closes the metrics client
func (c *PrometheusClient) Close() error {
	// Perform any cleanup needed
	return nil
}

// NoopClient is a no-op implementation of the metrics client
type NoopClient struct{}

// RecordEvent is a no-op implementation
func (c *NoopClient) RecordEvent(source, eventType string) {}

// RecordLatency is a no-op implementation
func (c *NoopClient) RecordLatency(operation string, duration time.Duration) {}

// RecordCounter is a no-op implementation
func (c *NoopClient) RecordCounter(name string, value float64, labels map[string]string) {}

// RecordGauge is a no-op implementation
func (c *NoopClient) RecordGauge(name string, value float64, labels map[string]string) {}

// Close is a no-op implementation
func (c *NoopClient) Close() error { return nil }
