package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Workflow represents a multi-agent workflow definition
type Workflow struct {
	ID          uuid.UUID     `json:"id" db:"id"`
	TenantID    uuid.UUID     `json:"tenant_id" db:"tenant_id"`
	Name        string        `json:"name" db:"name"`
	Type        WorkflowType  `json:"type" db:"type"`
	Version     int           `json:"version" db:"version"`
	CreatedBy   string        `json:"created_by" db:"created_by"`
	Agents      JSONMap       `json:"agents" db:"agents"`
	Steps       JSONMap       `json:"steps" db:"steps"`
	Config      JSONMap       `json:"config" db:"config"`
	Description string        `json:"description,omitempty" db:"description"`
	Tags        pq.StringArray `json:"tags,omitempty" db:"tags"`
	IsActive    bool          `json:"is_active" db:"is_active"`
	CreatedAt   time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at" db:"updated_at"`
	DeletedAt   *time.Time    `json:"deleted_at,omitempty" db:"deleted_at"`
}

// WorkflowType represents the execution strategy of a workflow
type WorkflowType string

const (
	WorkflowTypeSequential    WorkflowType = "sequential"
	WorkflowTypeParallel      WorkflowType = "parallel"
	WorkflowTypeConditional   WorkflowType = "conditional"
	WorkflowTypeCollaborative WorkflowType = "collaborative"
)

// WorkflowExecution represents a running or completed workflow instance
type WorkflowExecution struct {
	ID            uuid.UUID        `json:"id" db:"id"`
	WorkflowID    uuid.UUID        `json:"workflow_id" db:"workflow_id"`
	TenantID      uuid.UUID        `json:"tenant_id" db:"tenant_id"`
	Status        WorkflowStatus   `json:"status" db:"status"`
	Context       JSONMap          `json:"context" db:"context"`
	State         JSONMap          `json:"state" db:"state"`
	InitiatedBy   string           `json:"initiated_by" db:"initiated_by"`
	Error         string           `json:"error,omitempty" db:"error"`
	StartedAt     time.Time        `json:"started_at" db:"started_at"`
	CompletedAt   *time.Time       `json:"completed_at,omitempty" db:"completed_at"`
	UpdatedAt     time.Time        `json:"updated_at" db:"updated_at"`
	
	// Runtime data
	Workflow      *Workflow        `json:"workflow,omitempty" db:"-"`
	StepStatuses  map[string]*StepStatus `json:"step_statuses,omitempty" db:"-"`
	CurrentStepID string           `json:"current_step_id,omitempty" db:"-"`
}

// WorkflowStatus represents the state of a workflow execution
type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"
	WorkflowStatusRunning   WorkflowStatus = "running"
	WorkflowStatusPaused    WorkflowStatus = "paused"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
	WorkflowStatusCancelled WorkflowStatus = "cancelled"
	WorkflowStatusTimeout   WorkflowStatus = "timeout"
)

// StepStatus represents the execution state of a workflow step
type StepStatus struct {
	StepID      string                 `json:"step_id"`
	Status      string                 `json:"status"`
	AgentID     string                 `json:"agent_id"`
	Input       map[string]interface{} `json:"input,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	RetryCount  int                    `json:"retry_count"`
}

// ExecutionEvent represents an event in the workflow execution timeline
type ExecutionEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	EventType   string                 `json:"event_type"`
	StepID      string                 `json:"step_id,omitempty"`
	AgentID     string                 `json:"agent_id,omitempty"`
	Description string                 `json:"description"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// Helper methods

// Duration returns the execution duration
func (e *WorkflowExecution) Duration() time.Duration {
	if e.CompletedAt == nil {
		return time.Since(e.StartedAt)
	}
	return e.CompletedAt.Sub(e.StartedAt)
}

// IsTerminal returns true if the workflow is in a terminal state
func (e *WorkflowExecution) IsTerminal() bool {
	switch e.Status {
	case WorkflowStatusCompleted, WorkflowStatusFailed, WorkflowStatusCancelled, WorkflowStatusTimeout:
		return true
	default:
		return false
	}
}

// StepExecution represents the execution of a workflow step
type StepExecution struct {
	ExecutionID  uuid.UUID              `json:"execution_id" db:"execution_id"`
	StepName     string                 `json:"step_name" db:"step_name"`
	StartedAt    time.Time              `json:"started_at" db:"started_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty" db:"completed_at"`
	Status       string                 `json:"status" db:"status"`
	Result       JSONMap                `json:"result,omitempty" db:"result"`
	Error        *string                `json:"error,omitempty" db:"error"`
	RetryCount   int                    `json:"retry_count" db:"retry_count"`
}