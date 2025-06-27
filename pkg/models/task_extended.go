package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TaskEvent represents an event in task lifecycle
type TaskEvent struct {
	ID        uuid.UUID              `json:"id" db:"id"`
	TaskID    uuid.UUID              `json:"task_id" db:"task_id"`
	Type      string                 `json:"type" db:"type"`
	Timestamp time.Time              `json:"timestamp" db:"timestamp"`
	AgentID   string                 `json:"agent_id" db:"agent_id"`
	Data      map[string]interface{} `json:"data" db:"data"`
}

// DistributedTask represents a task that can be split into subtasks
type DistributedTask struct {
	ID          uuid.UUID         `json:"id"`
	Type        string            `json:"type"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Priority    TaskPriority      `json:"priority"`
	Subtasks    []Subtask         `json:"subtasks"`
	Aggregation AggregationConfig `json:"aggregation"`
	SubtaskIDs  []uuid.UUID       `json:"subtask_ids,omitempty"`

	// Phase 3 additions
	Task                *Task            `json:"task,omitempty" db:"-"`
	CoordinationMode    CoordinationMode `json:"coordination_mode" db:"coordination_mode"`
	CompletionMode      CompletionMode   `json:"completion_mode" db:"completion_mode"`
	CompletionThreshold int              `json:"completion_threshold,omitempty" db:"completion_threshold"`

	// Execution tracking fields
	ExecutionPlan *ExecutionPlan  `json:"execution_plan,omitempty" db:"execution_plan"`
	Partitions    []TaskPartition `json:"partitions,omitempty" db:"-"`
	Progress      *TaskProgress   `json:"progress,omitempty" db:"-"`
	ResourceUsage *ResourceUsage  `json:"resource_usage,omitempty" db:"-"`

	// Timing fields
	StartedAt         *time.Time    `json:"started_at,omitempty" db:"started_at"`
	CompletedAt       *time.Time    `json:"completed_at,omitempty" db:"completed_at"`
	EstimatedDuration time.Duration `json:"estimated_duration,omitempty" db:"estimated_duration"`

	// Results aggregation
	ResultsCollected    int           `json:"results_collected" db:"results_collected"`
	FinalResult         interface{}   `json:"final_result,omitempty" db:"final_result"`
	IntermediateResults []interface{} `json:"intermediate_results,omitempty" db:"-"`
}

// Subtask represents a subtask definition
type Subtask struct {
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// AggregationConfig defines how results should be aggregated
type AggregationConfig struct {
	Method     string `json:"method"`       // combine_results, first_complete, majority_vote
	WaitForAll bool   `json:"wait_for_all"` // Whether to wait for all subtasks
	Timeout    int    `json:"timeout"`      // Timeout in seconds
}

// TaskStats represents task statistics
type TaskStats struct {
	TotalTasks      int64                  `json:"total_tasks"`
	TasksByStatus   map[TaskStatus]int64   `json:"tasks_by_status"`
	TasksByPriority map[TaskPriority]int64 `json:"tasks_by_priority"`
	TasksByType     map[string]int64       `json:"tasks_by_type"`
	AverageTime     float64                `json:"average_time_seconds"`
	SuccessRate     float64                `json:"success_rate"`
}

// AgentPerformance represents agent performance metrics
type AgentPerformance struct {
	AgentID               string                 `json:"agent_id"`
	TasksCompleted        int64                  `json:"tasks_completed"`
	TasksFailed           int64                  `json:"tasks_failed"`
	AverageCompletionTime float64                `json:"average_completion_time"`
	SuccessRate           float64                `json:"success_rate"`
	LoadFactor            float64                `json:"load_factor"`
	SpeedScore            float64                `json:"speed_score"`
	TaskTypeMetrics       map[string]TaskMetrics `json:"task_type_metrics"`
}

// TaskMetrics represents metrics for a specific task type
type TaskMetrics struct {
	Count       int64   `json:"count"`
	SuccessRate float64 `json:"success_rate"`
	AverageTime float64 `json:"average_time"`
	MedianTime  float64 `json:"median_time"`
	P95Time     float64 `json:"p95_time"`
}

// TaskCreatedEvent represents a task creation event
type TaskCreatedEvent struct {
	Task *Task `json:"task"`
}

// SubtaskCreatedEvent represents a subtask creation event
type SubtaskCreatedEvent struct {
	ParentTask *Task `json:"parent_task"`
	Subtask    *Task `json:"subtask"`
}

// TaskDelegatedEvent represents a task delegation event
type TaskDelegatedEvent struct {
	Task          *Task           `json:"task"`
	Delegation    *TaskDelegation `json:"delegation"`
	PreviousAgent string          `json:"previous_agent"`
}

// AgentWorkload represents current workload for an agent
type AgentWorkload struct {
	AgentID       string         `json:"agent_id"`
	ActiveTasks   int            `json:"active_tasks"`
	QueuedTasks   int            `json:"queued_tasks"`
	TasksByType   map[string]int `json:"tasks_by_type"`
	LoadScore     float64        `json:"load_score"`     // 0.0 (idle) to 1.0 (overloaded)
	EstimatedTime int            `json:"estimated_time"` // Estimated time to complete all tasks in seconds
}

// Helper methods for DistributedTask

// SetDefaults sets default values for a distributed task
func (dt *DistributedTask) SetDefaults() {
	if dt.CoordinationMode == "" {
		dt.CoordinationMode = CoordinationModeParallel
	}
	if dt.CompletionMode == "" {
		dt.CompletionMode = CompletionModeAll
	}
	if dt.Priority == "" {
		dt.Priority = TaskPriorityNormal
	}
}

// Validate validates the distributed task
func (dt *DistributedTask) Validate() error {
	if dt.ID == uuid.Nil {
		return fmt.Errorf("distributed task ID is required")
	}
	if dt.Type == "" {
		return fmt.Errorf("distributed task type is required")
	}
	if !dt.CoordinationMode.IsValid() {
		return fmt.Errorf("invalid coordination mode: %s", dt.CoordinationMode)
	}
	if !dt.CompletionMode.IsValid() {
		return fmt.Errorf("invalid completion mode: %s", dt.CompletionMode)
	}
	if dt.CompletionMode == CompletionModeThreshold && dt.CompletionThreshold <= 0 {
		return fmt.Errorf("completion threshold must be positive for threshold mode")
	}
	return nil
}

// CalculateProgress calculates the overall progress of the distributed task
func (dt *DistributedTask) CalculateProgress() float64 {
	if dt.Progress != nil {
		return dt.Progress.PercentComplete
	}

	if len(dt.SubtaskIDs) == 0 {
		return 0.0
	}

	// Calculate based on collected results
	return float64(dt.ResultsCollected) / float64(len(dt.SubtaskIDs)) * 100.0
}

// IsComplete checks if the distributed task is complete based on completion mode
func (dt *DistributedTask) IsComplete() bool {
	totalSubtasks := len(dt.SubtaskIDs)
	if totalSubtasks == 0 {
		return true
	}

	switch dt.CompletionMode {
	case CompletionModeAll:
		return dt.ResultsCollected >= totalSubtasks
	case CompletionModeAny:
		return dt.ResultsCollected > 0
	case CompletionModeMajority:
		return dt.ResultsCollected > totalSubtasks/2
	case CompletionModeThreshold:
		if dt.CompletionThreshold > 0 {
			return dt.ResultsCollected >= dt.CompletionThreshold
		}
		return dt.ResultsCollected >= totalSubtasks
	case CompletionModeBestOf:
		// For best_of mode, check if we have enough results
		return dt.ResultsCollected >= dt.CompletionThreshold
	default:
		return false
	}
}

// GetEstimatedCompletion returns the estimated completion time
func (dt *DistributedTask) GetEstimatedCompletion() *time.Time {
	if dt.StartedAt == nil {
		return nil
	}

	if dt.EstimatedDuration == 0 {
		return nil
	}

	estimatedTime := dt.StartedAt.Add(dt.EstimatedDuration)
	return &estimatedTime
}
