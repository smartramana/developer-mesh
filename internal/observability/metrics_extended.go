package observability

import (
	"time"
)

// RecordDuration records a duration metric
func (m *MetricsClient) RecordDuration(name string, duration time.Duration) {
	if !m.enabled {
		return
	}
	
	// Convert duration to seconds for consistent units
	durationSeconds := duration.Seconds()
	
	// Record both a histogram and a timer
	m.RecordHistogram(name, durationSeconds, map[string]string{})
}

// IncrementCounter increments a counter metric by a given value
func (m *MetricsClient) IncrementCounter(name string, value float64) {
	if !m.enabled {
		return
	}
	
	// Call RecordCounter with empty labels
	m.RecordCounter(name, value, map[string]string{})
}
