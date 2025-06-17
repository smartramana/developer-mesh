package models

import (
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
	ID          uuid.UUID          `json:"id"`
	Type        string             `json:"type"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Priority    TaskPriority       `json:"priority"`
	Subtasks    []Subtask          `json:"subtasks"`
	Aggregation AggregationConfig  `json:"aggregation"`
	SubtaskIDs  []uuid.UUID        `json:"subtask_ids,omitempty"`
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
	Method     string `json:"method"`      // combine_results, first_complete, majority_vote
	WaitForAll bool   `json:"wait_for_all"` // Whether to wait for all subtasks
	Timeout    int    `json:"timeout"`      // Timeout in seconds
}


// TaskStats represents task statistics
type TaskStats struct {
	TotalTasks      int64                    `json:"total_tasks"`
	TasksByStatus   map[TaskStatus]int64     `json:"tasks_by_status"`
	TasksByPriority map[TaskPriority]int64   `json:"tasks_by_priority"`
	TasksByType     map[string]int64         `json:"tasks_by_type"`
	AverageTime     float64                  `json:"average_time_seconds"`
	SuccessRate     float64                  `json:"success_rate"`
}

// AgentPerformance represents agent performance metrics
type AgentPerformance struct {
	AgentID              string                 `json:"agent_id"`
	TasksCompleted       int64                  `json:"tasks_completed"`
	TasksFailed          int64                  `json:"tasks_failed"`
	AverageCompletionTime float64               `json:"average_completion_time"`
	SuccessRate          float64                `json:"success_rate"`
	LoadFactor           float64                `json:"load_factor"`
	SpeedScore           float64                `json:"speed_score"`
	TaskTypeMetrics      map[string]TaskMetrics `json:"task_type_metrics"`
}

// TaskMetrics represents metrics for a specific task type
type TaskMetrics struct {
	Count              int64   `json:"count"`
	SuccessRate        float64 `json:"success_rate"`
	AverageTime        float64 `json:"average_time"`
	MedianTime         float64 `json:"median_time"`
	P95Time            float64 `json:"p95_time"`
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
	Task          *Task            `json:"task"`
	Delegation    *TaskDelegation  `json:"delegation"`
	PreviousAgent string          `json:"previous_agent"`
}

// AgentWorkload represents current workload for an agent
type AgentWorkload struct {
	AgentID      string         `json:"agent_id"`
	ActiveTasks  int            `json:"active_tasks"`
	QueuedTasks  int            `json:"queued_tasks"`
	TasksByType  map[string]int `json:"tasks_by_type"`
	LoadScore    float64        `json:"load_score"`    // 0.0 (idle) to 1.0 (overloaded)
	EstimatedTime int            `json:"estimated_time"` // Estimated time to complete all tasks in seconds
}