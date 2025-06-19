package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockMetricsClient for testing
type MockMetricsClient struct {
	counters   map[string]float64
	gauges     map[string]float64
	histograms map[string]float64
	timings    map[string]time.Duration
}

func NewMockMetricsClient() *MockMetricsClient {
	return &MockMetricsClient{
		counters:   make(map[string]float64),
		gauges:     make(map[string]float64),
		histograms: make(map[string]float64),
		timings:    make(map[string]time.Duration),
	}
}

func (m *MockMetricsClient) IncrementCounter(name string, value float64, tags map[string]string) {
	m.counters[name] += value
}

func (m *MockMetricsClient) RecordGauge(name string, value float64, tags map[string]string) {
	m.gauges[name] = value
}

func (m *MockMetricsClient) RecordHistogram(name string, value float64, tags map[string]string) {
	m.histograms[name] = value
}

func (m *MockMetricsClient) RecordTiming(name string, duration time.Duration, tags map[string]string) {
	m.timings[name] = duration
}

func TestAgentStatus_Validate(t *testing.T) {
	tests := []struct {
		name    string
		status  AgentStatus
		wantErr bool
	}{
		{"valid active", AgentStatusActive, false},
		{"valid inactive", AgentStatusInactive, false},
		{"valid maintenance", AgentStatusMaintenance, false},
		{"valid draining", AgentStatusDraining, false},
		{"valid error", AgentStatusError, false},
		{"valid offline", AgentStatusOffline, false},
		{"valid starting", AgentStatusStarting, false},
		{"valid stopping", AgentStatusStopping, false},
		{"invalid status", AgentStatus("invalid"), true},
		{"empty status", AgentStatus(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.status.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAgentStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		from     AgentStatus
		to       AgentStatus
		canTrans bool
	}{
		// Valid transitions
		{"offline to starting", AgentStatusOffline, AgentStatusStarting, true},
		{"starting to active", AgentStatusStarting, AgentStatusActive, true},
		{"starting to error", AgentStatusStarting, AgentStatusError, true},
		{"active to draining", AgentStatusActive, AgentStatusDraining, true},
		{"active to maintenance", AgentStatusActive, AgentStatusMaintenance, true},
		{"active to error", AgentStatusActive, AgentStatusError, true},
		{"active to stopping", AgentStatusActive, AgentStatusStopping, true},
		{"draining to inactive", AgentStatusDraining, AgentStatusInactive, true},
		{"draining to error", AgentStatusDraining, AgentStatusError, true},
		{"inactive to active", AgentStatusInactive, AgentStatusActive, true},
		{"inactive to maintenance", AgentStatusInactive, AgentStatusMaintenance, true},
		{"inactive to stopping", AgentStatusInactive, AgentStatusStopping, true},
		{"maintenance to active", AgentStatusMaintenance, AgentStatusActive, true},
		{"maintenance to inactive", AgentStatusMaintenance, AgentStatusInactive, true},
		{"maintenance to stopping", AgentStatusMaintenance, AgentStatusStopping, true},
		{"error to stopping", AgentStatusError, AgentStatusStopping, true},
		{"error to maintenance", AgentStatusError, AgentStatusMaintenance, true},
		{"stopping to offline", AgentStatusStopping, AgentStatusOffline, true},

		// Invalid transitions
		{"offline to active", AgentStatusOffline, AgentStatusActive, false},
		{"offline to inactive", AgentStatusOffline, AgentStatusInactive, false},
		{"active to offline", AgentStatusActive, AgentStatusOffline, false},
		{"active to starting", AgentStatusActive, AgentStatusStarting, false},
		{"inactive to error", AgentStatusInactive, AgentStatusError, false},
		{"error to active", AgentStatusError, AgentStatusActive, false},
		{"stopping to active", AgentStatusStopping, AgentStatusActive, false},
		{"invalid from status", AgentStatus("invalid"), AgentStatusActive, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.from.CanTransitionTo(tt.to)
			assert.Equal(t, tt.canTrans, result)
		})
	}
}

func TestAgentStatus_TransitionTo(t *testing.T) {
	metrics := NewMockMetricsClient()

	tests := []struct {
		name       string
		from       AgentStatus
		to         AgentStatus
		wantErr    bool
		wantStatus AgentStatus
	}{
		// Valid transitions
		{"offline to starting", AgentStatusOffline, AgentStatusStarting, false, AgentStatusStarting},
		{"starting to active", AgentStatusStarting, AgentStatusActive, false, AgentStatusActive},
		{"active to draining", AgentStatusActive, AgentStatusDraining, false, AgentStatusDraining},

		// Invalid transitions
		{"offline to active", AgentStatusOffline, AgentStatusActive, true, AgentStatusOffline},
		{"active to offline", AgentStatusActive, AgentStatusOffline, true, AgentStatusActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.from.TransitionTo(tt.to, metrics)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.wantStatus, result)
				// Check invalid transition metric
				assert.Greater(t, metrics.counters["agent.status.transition.invalid"], float64(0))
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantStatus, result)
				// Check success transition metric
				assert.Greater(t, metrics.counters["agent.status.transition.success"], float64(0))
			}
		})
	}
}

func TestWorkloadInfo_Calculate(t *testing.T) {
	tests := []struct {
		name          string
		workload      WorkloadInfo
		expectedScore float64
	}{
		{
			name: "no workload",
			workload: WorkloadInfo{
				ActiveTasks: 0,
				QueuedTasks: 0,
				CPUUsage:    0,
				MemoryUsage: 0,
			},
			expectedScore: 0,
		},
		{
			name: "moderate workload",
			workload: WorkloadInfo{
				ActiveTasks: 3,
				QueuedTasks: 2,
				CPUUsage:    50,
				MemoryUsage: 40,
			},
			expectedScore: 48, // (3*2+2)*10*0.6 + (50+40)/2*0.4 = 48
		},
		{
			name: "high workload",
			workload: WorkloadInfo{
				ActiveTasks: 10,
				QueuedTasks: 5,
				CPUUsage:    90,
				MemoryUsage: 85,
			},
			expectedScore: 95, // min(100, (10*2+5)*10)*0.6 + (90+85)/2*0.4 = 95
		},
		{
			name: "max task load",
			workload: WorkloadInfo{
				ActiveTasks: 50,
				QueuedTasks: 50,
				CPUUsage:    100,
				MemoryUsage: 100,
			},
			expectedScore: 100, // 100*0.6 + 100*0.4 = 100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.workload.Calculate()
			assert.Equal(t, tt.expectedScore, tt.workload.WorkloadScore)
		})
	}
}

func TestHealthStatus_Validate(t *testing.T) {
	validStatuses := []HealthStatus{
		HealthStatusHealthy,
		HealthStatusDegraded,
		HealthStatusUnhealthy,
		HealthStatusUnknown,
	}

	for _, status := range validStatuses {
		t.Run(string(status), func(t *testing.T) {
			// Just ensure constants are defined
			assert.NotEmpty(t, status)
		})
	}
}

func TestAgentCapability_Constants(t *testing.T) {
	capabilities := []AgentCapability{
		AgentCapabilityCompute,
		AgentCapabilityStorage,
		AgentCapabilityNetwork,
		AgentCapabilityOrchestrate,
		AgentCapabilityAnalyze,
		AgentCapabilitySecure,
		AgentCapabilitySpecialized,
	}

	for _, cap := range capabilities {
		t.Run(string(cap), func(t *testing.T) {
			assert.NotEmpty(t, cap)
		})
	}
}

func TestAgentMetrics_Structure(t *testing.T) {
	now := time.Now()
	metrics := AgentMetrics{
		CPUUsage:      75.5,
		MemoryUsage:   80.2,
		DiskUsage:     45.0,
		NetworkIO:     1024.5,
		TasksActive:   5,
		TasksQueued:   3,
		TasksComplete: 100,
		ErrorRate:     0.5,
		ResponseTime:  150.5,
		LastUpdated:   now,
	}

	assert.Equal(t, 75.5, metrics.CPUUsage)
	assert.Equal(t, 80.2, metrics.MemoryUsage)
	assert.Equal(t, 45.0, metrics.DiskUsage)
	assert.Equal(t, 1024.5, metrics.NetworkIO)
	assert.Equal(t, 5, metrics.TasksActive)
	assert.Equal(t, 3, metrics.TasksQueued)
	assert.Equal(t, int64(100), metrics.TasksComplete)
	assert.Equal(t, 0.5, metrics.ErrorRate)
	assert.Equal(t, 150.5, metrics.ResponseTime)
	assert.Equal(t, now, metrics.LastUpdated)
}

func TestAgentHealth_Structure(t *testing.T) {
	now := time.Now()
	health := AgentHealth{
		Status:    HealthStatusHealthy,
		LastCheck: now,
		NextCheck: now.Add(time.Minute),
		Checks: map[string]CheckResult{
			"cpu": {
				Status:      HealthStatusHealthy,
				Message:     "CPU usage normal",
				LastSuccess: now,
				Duration:    time.Millisecond * 50,
			},
			"memory": {
				Status:      HealthStatusDegraded,
				Message:     "Memory usage high",
				LastSuccess: now.Add(-time.Minute),
				Duration:    time.Millisecond * 30,
				Error:       "memory threshold exceeded",
			},
		},
		Message: "Overall system healthy",
	}

	assert.Equal(t, HealthStatusHealthy, health.Status)
	assert.Equal(t, now, health.LastCheck)
	assert.Equal(t, now.Add(time.Minute), health.NextCheck)
	assert.Len(t, health.Checks, 2)
	assert.Equal(t, "Overall system healthy", health.Message)

	// Check individual check results
	cpuCheck := health.Checks["cpu"]
	assert.Equal(t, HealthStatusHealthy, cpuCheck.Status)
	assert.Equal(t, "CPU usage normal", cpuCheck.Message)
	assert.Empty(t, cpuCheck.Error)

	memCheck := health.Checks["memory"]
	assert.Equal(t, HealthStatusDegraded, memCheck.Status)
	assert.Equal(t, "Memory usage high", memCheck.Message)
	assert.Equal(t, "memory threshold exceeded", memCheck.Error)
}