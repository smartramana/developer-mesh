package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Run("NoopClient when disabled", func(t *testing.T) {
		cfg := Config{
			Enabled: false,
			Type:    "prometheus",
		}

		client := NewClient(cfg)
		assert.NotNil(t, client)
		
		// Verify it's a NoopClient
		_, ok := client.(*NoopClient)
		assert.True(t, ok, "Expected NoopClient")
	})
	
	t.Run("PrometheusClient", func(t *testing.T) {
		cfg := Config{
			Enabled:      true,
			Type:         "prometheus",
			PushGateway:  "localhost:9091",
			PushInterval: 10 * time.Second,
		}

		client := NewClient(cfg)
		assert.NotNil(t, client)
		
		// Verify it's a PrometheusClient
		promClient, ok := client.(*PrometheusClient)
		assert.True(t, ok, "Expected PrometheusClient")
		assert.Equal(t, cfg.PushGateway, promClient.config.PushGateway)
		
		// Clean up
		err := client.Close()
		assert.NoError(t, err)
	})

	t.Run("Default to NoopClient", func(t *testing.T) {
		cfg := Config{
			Enabled: true,
			Type:    "unsupported",
		}

		client := NewClient(cfg)
		assert.NotNil(t, client)
		
		// Verify it's a NoopClient
		_, ok := client.(*NoopClient)
		assert.True(t, ok, "Expected NoopClient for unsupported type")
	})
}

func TestPrometheusClient(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		Type:        "prometheus",
		PushGateway: "", // No push gateway for testing
	}

	client := NewPrometheusClient(cfg)
	require.NotNil(t, client)

	t.Run("RecordEvent", func(t *testing.T) {
		// Record some events
		client.RecordEvent("source1", "type1")
		client.RecordEvent("source1", "type1") // Same event twice
		client.RecordEvent("source2", "type2")
		
		// Check that events were recorded
		client.mu.RLock()
		defer client.mu.RUnlock()
		
		assert.Equal(t, 2.0, client.metrics["events_total_source1_type1"])
		assert.Equal(t, 1.0, client.metrics["events_total_source2_type2"])
	})
	
	t.Run("RecordLatency", func(t *testing.T) {
		// Record latency
		client.RecordLatency("operation1", 100*time.Millisecond)
		client.RecordLatency("operation2", 200*time.Millisecond)
		
		// Check that latencies were recorded
		client.mu.RLock()
		defer client.mu.RUnlock()
		
		assert.Equal(t, float64(100), client.metrics["latency_operation1"])
		assert.Equal(t, float64(200), client.metrics["latency_operation2"])
	})
	
	t.Run("RecordCounter", func(t *testing.T) {
		// Record counters
		client.RecordCounter("counter1", 5, map[string]string{"label1": "value1"})
		client.RecordCounter("counter1", 3, map[string]string{"label1": "value1"}) // Add to same counter
		client.RecordCounter("counter2", 10, map[string]string{"label2": "value2"})
		
		// Check that counters were recorded
		client.mu.RLock()
		defer client.mu.RUnlock()
		
		assert.Equal(t, 8.0, client.metrics["counter_counter1"])
		assert.Equal(t, 10.0, client.metrics["counter_counter2"])
	})
	
	t.Run("RecordGauge", func(t *testing.T) {
		// Record gauges
		client.RecordGauge("gauge1", 42, map[string]string{"label1": "value1"})
		client.RecordGauge("gauge2", 84, map[string]string{"label2": "value2"})
		
		// Update a gauge
		client.RecordGauge("gauge1", 50, map[string]string{"label1": "value1"})
		
		// Check that gauges were recorded
		client.mu.RLock()
		defer client.mu.RUnlock()
		
		assert.Equal(t, 50.0, client.metrics["gauge_gauge1"]) // Updated value
		assert.Equal(t, 84.0, client.metrics["gauge_gauge2"])
	})
	
	// Clean up
	err := client.Close()
	assert.NoError(t, err)
}

func TestNoopClient(t *testing.T) {
	client := &NoopClient{}
	
	// Test that methods don't panic
	client.RecordEvent("source", "type")
	client.RecordLatency("operation", 100*time.Millisecond)
	client.RecordCounter("counter", 1, map[string]string{"label": "value"})
	client.RecordGauge("gauge", 42, map[string]string{"label": "value"})
	
	// Test close
	err := client.Close()
	assert.NoError(t, err)
}
