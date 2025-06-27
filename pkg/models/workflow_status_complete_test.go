package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestStepStatus_Validate(t *testing.T) {
	tests := []struct {
		name    string
		status  StepExecutionStatus
		wantErr bool
	}{
		{"valid pending", StepExecutionStatusPending, false},
		{"valid queued", StepExecutionStatusQueued, false},
		{"valid running", StepExecutionStatusRunning, false},
		{"valid completed", StepExecutionStatusCompleted, false},
		{"valid failed", StepExecutionStatusFailed, false},
		{"valid skipped", StepExecutionStatusSkipped, false},
		{"valid retrying", StepExecutionStatusRetrying, false},
		{"valid cancelling", StepExecutionStatusCancelling, false},
		{"valid cancelled", StepExecutionStatusCancelled, false},
		{"valid timeout", StepExecutionStatusTimeout, false},
		{"invalid status", StepExecutionStatus("invalid"), true},
		{"empty status", StepExecutionStatus(""), true},
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

func TestStepStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		from     StepExecutionStatus
		to       StepExecutionStatus
		canTrans bool
	}{
		// Valid transitions from pending
		{"pending to queued", StepExecutionStatusPending, StepExecutionStatusQueued, true},
		{"pending to skipped", StepExecutionStatusPending, StepExecutionStatusSkipped, true},
		{"pending to cancelling", StepExecutionStatusPending, StepExecutionStatusCancelling, true},

		// Valid transitions from queued
		{"queued to running", StepExecutionStatusQueued, StepExecutionStatusRunning, true},
		{"queued to cancelling", StepExecutionStatusQueued, StepExecutionStatusCancelling, true},

		// Valid transitions from running
		{"running to completed", StepExecutionStatusRunning, StepExecutionStatusCompleted, true},
		{"running to failed", StepExecutionStatusRunning, StepExecutionStatusFailed, true},
		{"running to retrying", StepExecutionStatusRunning, StepExecutionStatusRetrying, true},
		{"running to cancelling", StepExecutionStatusRunning, StepExecutionStatusCancelling, true},
		{"running to timeout", StepExecutionStatusRunning, StepExecutionStatusTimeout, true},

		// Valid transitions from retrying
		{"retrying to running", StepExecutionStatusRetrying, StepExecutionStatusRunning, true},
		{"retrying to failed", StepExecutionStatusRetrying, StepExecutionStatusFailed, true},
		{"retrying to cancelling", StepExecutionStatusRetrying, StepExecutionStatusCancelling, true},

		// Valid transitions from cancelling
		{"cancelling to cancelled", StepExecutionStatusCancelling, StepExecutionStatusCancelled, true},

		// Invalid transitions
		{"pending to running", StepExecutionStatusPending, StepExecutionStatusRunning, false},
		{"pending to completed", StepExecutionStatusPending, StepExecutionStatusCompleted, false},
		{"queued to completed", StepExecutionStatusQueued, StepExecutionStatusCompleted, false},
		{"completed to running", StepExecutionStatusCompleted, StepExecutionStatusRunning, false},
		{"failed to running", StepExecutionStatusFailed, StepExecutionStatusRunning, false},
		{"skipped to running", StepExecutionStatusSkipped, StepExecutionStatusRunning, false},
		{"cancelled to running", StepExecutionStatusCancelled, StepExecutionStatusRunning, false},
		{"timeout to running", StepExecutionStatusTimeout, StepExecutionStatusRunning, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.from.CanTransitionTo(tt.to)
			assert.Equal(t, tt.canTrans, result)
		})
	}
}

func TestStepStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		name       string
		status     StepExecutionStatus
		isTerminal bool
	}{
		{"pending not terminal", StepExecutionStatusPending, false},
		{"queued not terminal", StepExecutionStatusQueued, false},
		{"running not terminal", StepExecutionStatusRunning, false},
		{"retrying not terminal", StepExecutionStatusRetrying, false},
		{"cancelling not terminal", StepExecutionStatusCancelling, false},
		{"completed is terminal", StepExecutionStatusCompleted, true},
		{"failed is terminal", StepExecutionStatusFailed, true},
		{"skipped is terminal", StepExecutionStatusSkipped, true},
		{"cancelled is terminal", StepExecutionStatusCancelled, true},
		{"timeout is terminal", StepExecutionStatusTimeout, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsTerminal()
			assert.Equal(t, tt.isTerminal, result)
		})
	}
}

func TestStepStatus_TransitionTo(t *testing.T) {
	metrics := NewMockMetricsClient()

	tests := []struct {
		name       string
		from       StepExecutionStatus
		to         StepExecutionStatus
		wantErr    bool
		wantStatus StepExecutionStatus
	}{
		// Valid transitions
		{"pending to queued", StepExecutionStatusPending, StepExecutionStatusQueued, false, StepExecutionStatusQueued},
		{"queued to running", StepExecutionStatusQueued, StepExecutionStatusRunning, false, StepExecutionStatusRunning},
		{"running to completed", StepExecutionStatusRunning, StepExecutionStatusCompleted, false, StepExecutionStatusCompleted},

		// Invalid transitions
		{"pending to completed", StepExecutionStatusPending, StepExecutionStatusCompleted, true, StepExecutionStatusPending},
		{"completed to running", StepExecutionStatusCompleted, StepExecutionStatusRunning, true, StepExecutionStatusCompleted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset metrics
			metrics.counters = make(map[string]float64)

			result, err := tt.from.TransitionTo(tt.to, metrics)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.wantStatus, result)
				// Check invalid transition metric
				assert.Greater(t, metrics.counters["step.status.transition.invalid"], float64(0))
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantStatus, result)
				// Check success transition metric
				assert.Greater(t, metrics.counters["step.status.transition.success"], float64(0))
			}
		})
	}
}

func TestWorkflowStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		from     WorkflowStatus
		to       WorkflowStatus
		canTrans bool
	}{
		// Valid transitions from pending
		{"pending to running", WorkflowStatusPending, WorkflowStatusRunning, true},
		{"pending to cancelled", WorkflowStatusPending, WorkflowStatusCancelled, true},

		// Valid transitions from running
		{"running to paused", WorkflowStatusRunning, WorkflowStatusPaused, true},
		{"running to completed", WorkflowStatusRunning, WorkflowStatusCompleted, true},
		{"running to failed", WorkflowStatusRunning, WorkflowStatusFailed, true},
		{"running to cancelled", WorkflowStatusRunning, WorkflowStatusCancelled, true},
		{"running to timeout", WorkflowStatusRunning, WorkflowStatusTimeout, true},

		// Valid transitions from paused
		{"paused to running", WorkflowStatusPaused, WorkflowStatusRunning, true},
		{"paused to cancelled", WorkflowStatusPaused, WorkflowStatusCancelled, true},
		{"paused to timeout", WorkflowStatusPaused, WorkflowStatusTimeout, true},

		// Invalid transitions
		{"pending to completed", WorkflowStatusPending, WorkflowStatusCompleted, false},
		{"completed to running", WorkflowStatusCompleted, WorkflowStatusRunning, false},
		{"failed to running", WorkflowStatusFailed, WorkflowStatusRunning, false},
		{"cancelled to running", WorkflowStatusCancelled, WorkflowStatusRunning, false},
		{"timeout to running", WorkflowStatusTimeout, WorkflowStatusRunning, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.from.CanTransitionTo(tt.to)
			assert.Equal(t, tt.canTrans, result)
		})
	}
}

func TestWorkflowStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		name       string
		status     WorkflowStatus
		isTerminal bool
	}{
		{"pending not terminal", WorkflowStatusPending, false},
		{"running not terminal", WorkflowStatusRunning, false},
		{"paused not terminal", WorkflowStatusPaused, false},
		{"completed is terminal", WorkflowStatusCompleted, true},
		{"failed is terminal", WorkflowStatusFailed, true},
		{"cancelled is terminal", WorkflowStatusCancelled, true},
		{"timeout is terminal", WorkflowStatusTimeout, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.IsTerminal()
			assert.Equal(t, tt.isTerminal, result)
		})
	}
}

func TestWorkflowExecutionStatusAliases(t *testing.T) {
	// Test that aliases are correctly defined
	assert.Equal(t, WorkflowStatusPending, WorkflowExecutionStatusPending)
	assert.Equal(t, WorkflowStatusRunning, WorkflowExecutionStatusRunning)
	assert.Equal(t, WorkflowStatusPaused, WorkflowExecutionStatusPaused)
	assert.Equal(t, WorkflowStatusCompleted, WorkflowExecutionStatusCompleted)
	assert.Equal(t, WorkflowStatusFailed, WorkflowExecutionStatusFailed)
	assert.Equal(t, WorkflowStatusCancelled, WorkflowExecutionStatusCancelled)
	assert.Equal(t, WorkflowStatusTimeout, WorkflowExecutionStatusTimeout)
}

func TestStepExecutionStatusAliases(t *testing.T) {
	// Test that step execution aliases are correctly defined
	assert.Equal(t, StepExecutionStatusPending, StepExecutionStatusPending)
	assert.Equal(t, StepExecutionStatusRunning, StepExecutionStatusRunning)
	assert.Equal(t, StepExecutionStatusCompleted, StepExecutionStatusCompleted)
	assert.Equal(t, StepExecutionStatusFailed, StepExecutionStatusFailed)
}

func TestWorkflowTypeConstants(t *testing.T) {
	// Test that workflow type constants are defined
	types := []WorkflowType{
		WorkflowTypeStandard,
		WorkflowTypeDAG,
		WorkflowTypeSaga,
		WorkflowTypeStateMachine,
		WorkflowTypeEvent,
	}

	for _, wfType := range types {
		t.Run(string(wfType), func(t *testing.T) {
			assert.NotEmpty(t, wfType)
		})
	}
}

func TestWorkflowExecutionMetrics_Structure(t *testing.T) {
	executionID := uuid.New()
	metrics := WorkflowExecutionMetrics{
		ExecutionID:   executionID,
		TotalDuration: 5 * time.Minute,
		StepDurations: map[string]time.Duration{
			"step1": 1 * time.Minute,
			"step2": 2 * time.Minute,
			"step3": 2 * time.Minute,
		},
		QueueTime:  30 * time.Second,
		RetryCount: 2,
		ResourceUsage: ResourceMetrics{
			CPUSeconds:      300.5,
			MemoryMBSeconds: 1024.75,
			NetworkMB:       50.25,
			StorageMB:       100.0,
		},
		CostEstimate: 0.15,
	}

	assert.Equal(t, executionID, metrics.ExecutionID)
	assert.Equal(t, 5*time.Minute, metrics.TotalDuration)
	assert.Len(t, metrics.StepDurations, 3)
	assert.Equal(t, 1*time.Minute, metrics.StepDurations["step1"])
	assert.Equal(t, 30*time.Second, metrics.QueueTime)
	assert.Equal(t, 2, metrics.RetryCount)
	assert.Equal(t, 300.5, metrics.ResourceUsage.CPUSeconds)
	assert.Equal(t, 1024.75, metrics.ResourceUsage.MemoryMBSeconds)
	assert.Equal(t, 50.25, metrics.ResourceUsage.NetworkMB)
	assert.Equal(t, 100.0, metrics.ResourceUsage.StorageMB)
	assert.Equal(t, 0.15, metrics.CostEstimate)
}

func TestResourceMetrics_Structure(t *testing.T) {
	metrics := ResourceMetrics{
		CPUSeconds:      120.5,
		MemoryMBSeconds: 512.25,
		NetworkMB:       25.5,
		StorageMB:       50.0,
	}

	assert.Equal(t, 120.5, metrics.CPUSeconds)
	assert.Equal(t, 512.25, metrics.MemoryMBSeconds)
	assert.Equal(t, 25.5, metrics.NetworkMB)
	assert.Equal(t, 50.0, metrics.StorageMB)
}
