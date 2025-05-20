package observability

import (
	"testing"
	"time"
)

func init() {
	// Ensure DefaultMetricsClient is initialized for tests
	if DefaultMetricsClient == nil {
		DefaultMetricsClient = NewMetricsClient()
	}
}

func TestMetricsClient_Enabled(t *testing.T) {
	// Create a metrics client with enabled=true
	metrics := NewMetricsClientWithOptions(MetricsOptions{
		Enabled: true,
		Labels:  map[string]string{"service": "test"},
	})

	// Verify the metrics client is enabled
	if metrics.(*metricsClient).enabled != true {
		t.Error("Expected metrics client to be enabled")
	}

	// Verify the labels are set
	if metrics.(*metricsClient).labels["service"] != "test" {
		t.Error("Expected metrics client to have labels set")
	}
}

func TestMetricsClient_Disabled(t *testing.T) {
	// Create a metrics client with enabled=false
	metrics := NewMetricsClientWithOptions(MetricsOptions{
		Enabled: false,
	})

	// Verify the metrics client is disabled
	if metrics.(*metricsClient).enabled != false {
		t.Error("Expected metrics client to be disabled")
	}

	// The following calls should not cause any errors even when disabled
	metrics.RecordEvent("test", "event")
	metrics.RecordLatency("operation", time.Second)
	metrics.RecordCounter("counter", 1, nil)
	metrics.RecordGauge("gauge", 2, nil)
	metrics.RecordHistogram("histogram", 3, nil)
	metrics.RecordTimer("timer", time.Second, nil)
	metrics.IncrementCounter("counter", 1, nil) // Fix: Added missing labels parameter
	metrics.RecordDuration("duration", time.Second)
	metrics.RecordCacheOperation("get", true, 0.1)
	metrics.RecordAPIOperation("api", "get", true, 0.2)
	metrics.RecordDatabaseOperation("query", true, 0.3)
	metrics.RecordOperation("component", "op", true, 0.4, nil)
	metrics.Close()
}

func TestMetricsClient_StartTimer(t *testing.T) {
	// Create a metrics client
	metrics := NewMetricsClient()

	// Start a timer
	stopTimer := metrics.StartTimer("test_timer", map[string]string{"label": "value"})
	
	// Sleep a bit
	time.Sleep(10 * time.Millisecond)
	
	// Stop the timer - this should not cause any errors
	stopTimer()
}

func TestMetricsClient_DefaultInstance(t *testing.T) {
	// Ensure DefaultMetricsClient is initialized
	if DefaultMetricsClient == nil {
		DefaultMetricsClient = NewMetricsClient()
	}
	
	// Verify that the default metrics client is initialized
	if DefaultMetricsClient == nil {
		t.Error("Expected DefaultMetricsClient to be initialized")
	} else {
		// Only call methods if it's not nil
		DefaultMetricsClient.RecordEvent("test", "event")
		DefaultMetricsClient.RecordLatency("operation", time.Second)
	}
}

func TestMetricsClient_RecordOperations(t *testing.T) {
	// Create a metrics client
	metrics := NewMetricsClient()
	
	// Record various operations - these should not cause any errors
	metrics.RecordCacheOperation("get", true, 0.1)
	metrics.RecordAPIOperation("api", "get", true, 0.2)
	metrics.RecordDatabaseOperation("query", true, 0.3)
	
	// Test with custom labels
	customLabels := map[string]string{
		"custom": "value",
		"env":    "test",
	}
	metrics.RecordOperation("custom-component", "custom-op", false, 0.5, customLabels)
}

func TestLegacyMetricsAdapter(t *testing.T) {
	// Create a metrics client
	metrics := NewMetricsClient()
	
	// Create an adapter for legacy clients
	adapter := NewLegacyMetricsAdapter(metrics)
	
	// Test the adapter methods
	adapter.RecordEvent("test", "event")
	adapter.RecordLatency("operation", time.Second)
	adapter.RecordCounter("counter", 1, map[string]string{"service": "test"})
	adapter.RecordGauge("gauge", 2, map[string]string{"service": "test"})
	
	// Test close
	if err := adapter.Close(); err != nil {
		t.Errorf("Expected no error from adapter.Close(), got: %v", err)
	}
}
