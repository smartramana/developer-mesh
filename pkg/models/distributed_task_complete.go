package models

import (
	"time"

	"github.com/google/uuid"
)

// CoordinationMode defines how subtasks are coordinated
type CoordinationMode string

const (
	CoordinationModeParallel    CoordinationMode = "parallel"    // All subtasks run in parallel
	CoordinationModeSequential  CoordinationMode = "sequential"  // Subtasks run one after another
	CoordinationModePipeline    CoordinationMode = "pipeline"    // Output of one feeds into next
	CoordinationModeMapReduce   CoordinationMode = "map_reduce"  // Map phase then reduce phase
	CoordinationModeLeaderElect CoordinationMode = "leader_elect" // One agent elected as coordinator
)

// CompletionMode defines when a distributed task is considered complete
type CompletionMode string

const (
	CompletionModeAll       CompletionMode = "all"        // All subtasks must complete
	CompletionModeAny       CompletionMode = "any"        // Any subtask completion completes the task
	CompletionModeMajority  CompletionMode = "majority"   // Majority of subtasks must complete
	CompletionModeThreshold CompletionMode = "threshold"  // Configurable threshold of completions
	CompletionModeBestOf    CompletionMode = "best_of"    // Best result from N attempts
)

// TaskPartition represents a partition of work for a subtask
type TaskPartition struct {
	ID         string                 `json:"id"`
	RangeStart int64                  `json:"range_start,omitempty"`
	RangeEnd   int64                  `json:"range_end,omitempty"`
	DataSet    string                 `json:"data_set,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Weight     float64                `json:"weight"` // Relative weight for load balancing
}

// SyncPoint represents a synchronization point between subtasks
type SyncPoint struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	RequiredTasks []string      `json:"required_tasks"`
	Timeout      time.Duration `json:"timeout"`
	OnTimeout    string        `json:"on_timeout"` // continue, fail, retry
}

// ExecutionPlan represents the execution plan for a distributed task
type ExecutionPlan struct {
	Phases      []ExecutionPhase       `json:"phases"`
	SyncPoints  []SyncPoint            `json:"sync_points,omitempty"`
	Constraints map[string]interface{} `json:"constraints,omitempty"`
	Timeout     time.Duration          `json:"timeout"`
}

// ExecutionPhase represents a phase in the execution plan
type ExecutionPhase struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	TaskIDs  []string `json:"task_ids"`
	Parallel bool     `json:"parallel"`
	MaxRetry int      `json:"max_retry"`
}

// TaskProgress represents progress information for a task
type TaskProgress struct {
	TaskID          uuid.UUID              `json:"task_id"`
	TotalSteps      int                    `json:"total_steps"`
	CompletedSteps  int                    `json:"completed_steps"`
	CurrentStep     string                 `json:"current_step"`
	PercentComplete float64                `json:"percent_complete"`
	EstimatedTimeRemaining time.Duration     `json:"estimated_time_remaining,omitempty"`
	LastUpdated     time.Time              `json:"last_updated"`
	Details         map[string]interface{} `json:"details,omitempty"`
}

// ResourceUsage represents resource usage for a task
type ResourceUsage struct {
	TaskID      uuid.UUID `json:"task_id"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemoryMB    int64     `json:"memory_mb"`
	DiskIOMB    int64     `json:"disk_io_mb"`
	NetworkMB   int64     `json:"network_mb"`
	GPUPercent  float64   `json:"gpu_percent,omitempty"`
	StartTime   time.Time `json:"start_time"`
	LastUpdated time.Time `json:"last_updated"`
}

// ExtendedDistributedTask extends the DistributedTask with production fields
type ExtendedDistributedTask struct {
	DistributedTask
	
	// Embedded task reference
	Task            *Task             `json:"task,omitempty"`
	
	// Coordination fields
	CoordinationMode CoordinationMode  `json:"coordination_mode"`
	CompletionMode   CompletionMode    `json:"completion_mode"`
	CompletionThreshold int            `json:"completion_threshold,omitempty"` // For threshold mode
	
	// Execution tracking
	ExecutionPlan   *ExecutionPlan    `json:"execution_plan,omitempty"`
	Partitions      []TaskPartition   `json:"partitions,omitempty"`
	Progress        *TaskProgress     `json:"progress,omitempty"`
	ResourceUsage   *ResourceUsage    `json:"resource_usage,omitempty"`
	
	// Timing
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
	EstimatedDuration time.Duration   `json:"estimated_duration,omitempty"`
	
	// Results aggregation
	ResultsCollected int              `json:"results_collected"`
	FinalResult     interface{}       `json:"final_result,omitempty"`
	IntermediateResults []interface{} `json:"intermediate_results,omitempty"`
}

// IsCoordinationValid checks if the coordination mode is valid
func (m CoordinationMode) IsValid() bool {
	switch m {
	case CoordinationModeParallel, CoordinationModeSequential, CoordinationModePipeline,
		CoordinationModeMapReduce, CoordinationModeLeaderElect:
		return true
	default:
		return false
	}
}

// IsCompletionValid checks if the completion mode is valid
func (m CompletionMode) IsValid() bool {
	switch m {
	case CompletionModeAll, CompletionModeAny, CompletionModeMajority,
		CompletionModeThreshold, CompletionModeBestOf:
		return true
	default:
		return false
	}
}

// CalculateProgress calculates the overall progress of the distributed task
func (dt *ExtendedDistributedTask) CalculateProgress() float64 {
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
func (dt *ExtendedDistributedTask) IsComplete() bool {
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
func (dt *ExtendedDistributedTask) GetEstimatedCompletion() *time.Time {
	if dt.StartedAt == nil {
		return nil
	}
	
	if dt.EstimatedDuration == 0 {
		return nil
	}
	
	estimatedTime := dt.StartedAt.Add(dt.EstimatedDuration)
	return &estimatedTime
}