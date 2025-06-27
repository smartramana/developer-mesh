package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// StepExecutionStatus for step execution state
type StepExecutionStatus string

// Convert step status string constants to StepExecutionStatus
const (
	StepExecutionStatusPending    StepExecutionStatus = StepExecutionStatus(StepStatusPending)
	StepExecutionStatusQueued     StepExecutionStatus = StepExecutionStatus(StepStatusQueued)
	StepExecutionStatusRunning    StepExecutionStatus = StepExecutionStatus(StepStatusRunning)
	StepExecutionStatusCompleted  StepExecutionStatus = StepExecutionStatus(StepStatusCompleted)
	StepExecutionStatusFailed     StepExecutionStatus = StepExecutionStatus(StepStatusFailed)
	StepExecutionStatusSkipped    StepExecutionStatus = StepExecutionStatus(StepStatusSkipped)
	StepExecutionStatusRetrying   StepExecutionStatus = StepExecutionStatus(StepStatusRetrying)
	StepExecutionStatusCancelling StepExecutionStatus = StepExecutionStatus(StepStatusCancelling)
	StepExecutionStatusCancelled  StepExecutionStatus = StepExecutionStatus(StepStatusCancelled)
	StepExecutionStatusTimeout    StepExecutionStatus = StepExecutionStatus(StepStatusTimeout)
)

// Alias the existing WorkflowStatus constants for compatibility
const (
	WorkflowExecutionStatusPending   = WorkflowStatusPending
	WorkflowExecutionStatusRunning   = WorkflowStatusRunning
	WorkflowExecutionStatusPaused    = WorkflowStatusPaused
	WorkflowExecutionStatusCompleted = WorkflowStatusCompleted
	WorkflowExecutionStatusFailed    = WorkflowStatusFailed
	WorkflowExecutionStatusCancelled = WorkflowStatusCancelled
	WorkflowExecutionStatusTimeout   = WorkflowStatusTimeout
)

// Add WorkflowTypeStandard to existing types
const (
	WorkflowTypeStandard     WorkflowType = "standard"
	WorkflowTypeDAG          WorkflowType = "dag"
	WorkflowTypeSaga         WorkflowType = "saga"
	WorkflowTypeStateMachine WorkflowType = "state_machine"
	WorkflowTypeEvent        WorkflowType = "event_driven"
)

// WorkflowTransitions defines valid state transitions
var WorkflowTransitions = map[WorkflowStatus][]WorkflowStatus{
	WorkflowStatusPending:   {WorkflowStatusRunning, WorkflowStatusCancelled},
	WorkflowStatusRunning:   {WorkflowStatusPaused, WorkflowStatusCompleted, WorkflowStatusFailed, WorkflowStatusCancelled, WorkflowStatusTimeout},
	WorkflowStatusPaused:    {WorkflowStatusRunning, WorkflowStatusCancelled, WorkflowStatusTimeout},
	WorkflowStatusCompleted: {},
	WorkflowStatusFailed:    {},
	WorkflowStatusCancelled: {},
	WorkflowStatusTimeout:   {},
}

// StepTransitions defines valid step state transitions
var StepTransitions = map[StepExecutionStatus][]StepExecutionStatus{
	StepExecutionStatusPending:    {StepExecutionStatusQueued, StepExecutionStatusSkipped, StepExecutionStatusCancelling},
	StepExecutionStatusQueued:     {StepExecutionStatusRunning, StepExecutionStatusCancelling},
	StepExecutionStatusRunning:    {StepExecutionStatusCompleted, StepExecutionStatusFailed, StepExecutionStatusRetrying, StepExecutionStatusCancelling, StepExecutionStatusTimeout},
	StepExecutionStatusRetrying:   {StepExecutionStatusRunning, StepExecutionStatusFailed, StepExecutionStatusCancelling},
	StepExecutionStatusCancelling: {StepExecutionStatusCancelled},
	StepExecutionStatusCompleted:  {},
	StepExecutionStatusFailed:     {},
	StepExecutionStatusSkipped:    {},
	StepExecutionStatusCancelled:  {},
	StepExecutionStatusTimeout:    {},
}

// CanTransitionTo checks if workflow status transition is valid
func (s WorkflowStatus) CanTransitionTo(target WorkflowStatus) bool {
	validTargets, exists := WorkflowTransitions[s]
	if !exists {
		return false
	}
	for _, valid := range validTargets {
		if valid == target {
			return true
		}
	}
	return false
}

// IsTerminal returns true if the status is a terminal state
func (s WorkflowStatus) IsTerminal() bool {
	switch s {
	case WorkflowStatusCompleted, WorkflowStatusFailed, WorkflowStatusCancelled, WorkflowStatusTimeout:
		return true
	default:
		return false
	}
}

// CanTransitionTo checks if step status transition is valid
func (s StepExecutionStatus) CanTransitionTo(target StepExecutionStatus) bool {
	validTargets, exists := StepTransitions[s]
	if !exists {
		return false
	}
	for _, valid := range validTargets {
		if valid == target {
			return true
		}
	}
	return false
}

// IsTerminal returns true if the step status is a terminal state
func (s StepExecutionStatus) IsTerminal() bool {
	switch s {
	case StepExecutionStatusCompleted, StepExecutionStatusFailed, StepExecutionStatusSkipped, StepExecutionStatusCancelled, StepExecutionStatusTimeout:
		return true
	default:
		return false
	}
}

// Validate ensures the step status is valid
func (s StepExecutionStatus) Validate() error {
	switch s {
	case StepExecutionStatusPending, StepExecutionStatusQueued, StepExecutionStatusRunning,
		StepExecutionStatusCompleted, StepExecutionStatusFailed, StepExecutionStatusSkipped,
		StepExecutionStatusRetrying, StepExecutionStatusCancelling, StepExecutionStatusCancelled,
		StepExecutionStatusTimeout:
		return nil
	default:
		return fmt.Errorf("invalid step status: %s", s)
	}
}

// TransitionTo performs a validated state transition with metrics
func (s StepExecutionStatus) TransitionTo(target StepExecutionStatus, metrics MetricsClient) (StepExecutionStatus, error) {
	if !s.CanTransitionTo(target) {
		metrics.IncrementCounter("step.status.transition.invalid", 1, map[string]string{
			"from": string(s),
			"to":   string(target),
		})
		return s, fmt.Errorf("invalid transition from %s to %s", s, target)
	}

	metrics.IncrementCounter("step.status.transition.success", 1, map[string]string{
		"from": string(s),
		"to":   string(target),
	})

	return target, nil
}

// WorkflowExecutionMetrics tracks workflow performance
type WorkflowExecutionMetrics struct {
	ExecutionID   uuid.UUID                `json:"execution_id"`
	TotalDuration time.Duration            `json:"total_duration"`
	StepDurations map[string]time.Duration `json:"step_durations"`
	QueueTime     time.Duration            `json:"queue_time"`
	RetryCount    int                      `json:"retry_count"`
	ResourceUsage ResourceMetrics          `json:"resource_usage"`
	CostEstimate  float64                  `json:"cost_estimate"`
}

type ResourceMetrics struct {
	CPUSeconds      float64 `json:"cpu_seconds"`
	MemoryMBSeconds float64 `json:"memory_mb_seconds"`
	NetworkMB       float64 `json:"network_mb"`
	StorageMB       float64 `json:"storage_mb"`
}
