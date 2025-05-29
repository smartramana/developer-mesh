package mocks

import (
	"time"

	"github.com/stretchr/testify/mock"
)

// MockMetricsClient is a mock implementation of the MetricsClient interface for testing
type MockMetricsClient struct {
	mock.Mock
}

// RecordEvent mocks the RecordEvent method
func (m *MockMetricsClient) RecordEvent(source, eventType string) {
	m.Called(source, eventType)
}

// RecordLatency mocks the RecordLatency method
func (m *MockMetricsClient) RecordLatency(operation string, duration time.Duration) {
	m.Called(operation, duration)
}

// RecordCounter mocks the RecordCounter method
func (m *MockMetricsClient) RecordCounter(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

// RecordGauge mocks the RecordGauge method
func (m *MockMetricsClient) RecordGauge(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

// Close mocks the Close method
func (m *MockMetricsClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
