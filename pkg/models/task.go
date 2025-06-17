package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Task represents a unit of work in the multi-agent system
type Task struct {
	// Core fields
	ID         uuid.UUID  `json:"id" db:"id"`
	TenantID   uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	Type       string     `json:"type" db:"type"`
	Status     TaskStatus `json:"status" db:"status"`
	Priority   TaskPriority `json:"priority" db:"priority"`
	
	// Agent relationships
	CreatedBy  string  `json:"created_by" db:"created_by"`
	AssignedTo *string `json:"assigned_to,omitempty" db:"assigned_to"`
	
	// Task hierarchy
	ParentTaskID *uuid.UUID `json:"parent_task_id,omitempty" db:"parent_task_id"`
	
	// Task data
	Title       string  `json:"title" db:"title"`
	Description string  `json:"description,omitempty" db:"description"`
	Parameters  JSONMap `json:"parameters" db:"parameters"`
	Result      JSONMap `json:"result,omitempty" db:"result"`
	Error       string  `json:"error,omitempty" db:"error"`
	
	// Execution control
	MaxRetries     int `json:"max_retries" db:"max_retries"`
	RetryCount     int `json:"retry_count" db:"retry_count"`
	TimeoutSeconds int `json:"timeout_seconds" db:"timeout_seconds"`
	
	// Timestamps
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	AssignedAt  *time.Time `json:"assigned_at,omitempty" db:"assigned_at"`
	StartedAt   *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
	
	// Optimistic locking
	Version int `json:"version" db:"version"`
	
	// Computed fields (not stored)
	Subtasks     []*Task               `json:"subtasks,omitempty" db:"-"`
	Delegations  []*TaskDelegation     `json:"delegations,omitempty" db:"-"`
	Tags         []string              `json:"tags,omitempty" db:"-"`
	Capabilities []string              `json:"capabilities,omitempty" db:"-"`
}

// TaskStatus represents the lifecycle state of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusAssigned   TaskStatus = "assigned"
	TaskStatusAccepted   TaskStatus = "accepted"
	TaskStatusRejected   TaskStatus = "rejected"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
	TaskStatusTimeout    TaskStatus = "timeout"
)

// TaskPriority represents the urgency of a task
type TaskPriority string

const (
	TaskPriorityLow      TaskPriority = "low"
	TaskPriorityNormal   TaskPriority = "normal"
	TaskPriorityHigh     TaskPriority = "high"
	TaskPriorityCritical TaskPriority = "critical"
)

// TaskDelegation represents a task being delegated between agents
type TaskDelegation struct {
	ID             uuid.UUID      `json:"id" db:"id"`
	TaskID         uuid.UUID      `json:"task_id" db:"task_id"`
	TaskCreatedAt  time.Time      `json:"task_created_at" db:"task_created_at"` // For partitioned table FK
	FromAgentID    string         `json:"from_agent_id" db:"from_agent_id"`
	ToAgentID      string         `json:"to_agent_id" db:"to_agent_id"`
	Reason         string         `json:"reason,omitempty" db:"reason"`
	DelegationType DelegationType `json:"delegation_type" db:"delegation_type"`
	Metadata       JSONMap        `json:"metadata" db:"metadata"`
	DelegatedAt    time.Time      `json:"delegated_at" db:"delegated_at"`
}

// DelegationType represents how a task was delegated
type DelegationType string

const (
	DelegationManual      DelegationType = "manual"
	DelegationAutomatic   DelegationType = "automatic"
	DelegationFailover    DelegationType = "failover"
	DelegationLoadBalance DelegationType = "load_balance"
)

// TaskTree represents a hierarchical task structure
type TaskTree struct {
	Root     *Task
	Children map[uuid.UUID][]*Task
	Depth    int
}

// DelegationNode represents a node in the delegation chain
type DelegationNode struct {
	Delegation *TaskDelegation
	Next       *DelegationNode
}

// JSONMap is a type alias for map[string]interface{} that implements sql.Scanner and driver.Valuer
type JSONMap map[string]interface{}

// Value implements driver.Valuer for JSONMap
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

// Scan implements sql.Scanner for JSONMap
func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, (*map[string]interface{})(m))
	case string:
		return json.Unmarshal([]byte(v), (*map[string]interface{})(m))
	default:
		return json.Unmarshal([]byte(v.(string)), (*map[string]interface{})(m))
	}
}

// Helper methods

// GetID returns the task ID (implements AggregateRoot)
func (t *Task) GetID() uuid.UUID {
	return t.ID
}

// GetType returns the aggregate type (implements AggregateRoot)
func (t *Task) GetType() string {
	return "Task"
}

// GetVersion returns the version (implements AggregateRoot)
func (t *Task) GetVersion() int {
	return t.Version
}

// IsTerminal returns true if the task is in a terminal state
func (t *Task) IsTerminal() bool {
	switch t.Status {
	case TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled, TaskStatusTimeout:
		return true
	default:
		return false
	}
}

// CanRetry returns true if the task can be retried
func (t *Task) CanRetry() bool {
	return !t.IsTerminal() && t.RetryCount < t.MaxRetries
}

// Duration returns the task execution duration
func (t *Task) Duration() time.Duration {
	if t.StartedAt == nil || t.CompletedAt == nil {
		return 0
	}
	return t.CompletedAt.Sub(*t.StartedAt)
}

// IsOverdue returns true if the task has exceeded its timeout
func (t *Task) IsOverdue() bool {
	if t.StartedAt == nil || t.IsTerminal() {
		return false
	}
	deadline := t.StartedAt.Add(time.Duration(t.TimeoutSeconds) * time.Second)
	return time.Now().After(deadline)
}