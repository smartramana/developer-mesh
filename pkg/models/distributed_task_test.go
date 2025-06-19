package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoordinationModeIsValid(t *testing.T) {
	tests := []struct {
		name     string
		mode     CoordinationMode
		expected bool
	}{
		{"Parallel mode", CoordinationModeParallel, true},
		{"Sequential mode", CoordinationModeSequential, true},
		{"Pipeline mode", CoordinationModePipeline, true},
		{"MapReduce mode", CoordinationModeMapReduce, true},
		{"LeaderElect mode", CoordinationModeLeaderElect, true},
		{"Invalid mode", CoordinationMode("invalid"), false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.IsValid())
		})
	}
}

func TestCompletionModeIsValid(t *testing.T) {
	tests := []struct {
		name     string
		mode     CompletionMode
		expected bool
	}{
		{"All mode", CompletionModeAll, true},
		{"Any mode", CompletionModeAny, true},
		{"Majority mode", CompletionModeMajority, true},
		{"Threshold mode", CompletionModeThreshold, true},
		{"BestOf mode", CompletionModeBestOf, true},
		{"Invalid mode", CompletionMode("invalid"), false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.IsValid())
		})
	}
}

func TestDistributedTaskSetDefaults(t *testing.T) {
	dt := &DistributedTask{}
	dt.SetDefaults()
	
	assert.Equal(t, CoordinationModeParallel, dt.CoordinationMode)
	assert.Equal(t, CompletionModeAll, dt.CompletionMode)
	assert.Equal(t, TaskPriorityNormal, dt.Priority)
}

func TestDistributedTaskValidate(t *testing.T) {
	tests := []struct {
		name    string
		task    *DistributedTask
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid distributed task",
			task: &DistributedTask{
				ID:               uuid.New(),
				Type:             "data_processing",
				CoordinationMode: CoordinationModeParallel,
				CompletionMode:   CompletionModeAll,
			},
			wantErr: false,
		},
		{
			name: "Missing ID",
			task: &DistributedTask{
				Type:             "data_processing",
				CoordinationMode: CoordinationModeParallel,
				CompletionMode:   CompletionModeAll,
			},
			wantErr: true,
			errMsg:  "distributed task ID is required",
		},
		{
			name: "Missing type",
			task: &DistributedTask{
				ID:               uuid.New(),
				CoordinationMode: CoordinationModeParallel,
				CompletionMode:   CompletionModeAll,
			},
			wantErr: true,
			errMsg:  "distributed task type is required",
		},
		{
			name: "Invalid coordination mode",
			task: &DistributedTask{
				ID:               uuid.New(),
				Type:             "data_processing",
				CoordinationMode: CoordinationMode("invalid"),
				CompletionMode:   CompletionModeAll,
			},
			wantErr: true,
			errMsg:  "invalid coordination mode",
		},
		{
			name: "Invalid completion mode",
			task: &DistributedTask{
				ID:               uuid.New(),
				Type:             "data_processing",
				CoordinationMode: CoordinationModeParallel,
				CompletionMode:   CompletionMode("invalid"),
			},
			wantErr: true,
			errMsg:  "invalid completion mode",
		},
		{
			name: "Threshold mode without threshold",
			task: &DistributedTask{
				ID:                  uuid.New(),
				Type:                "data_processing",
				CoordinationMode:    CoordinationModeParallel,
				CompletionMode:      CompletionModeThreshold,
				CompletionThreshold: 0,
			},
			wantErr: true,
			errMsg:  "completion threshold must be positive",
		},
		{
			name: "Threshold mode with valid threshold",
			task: &DistributedTask{
				ID:                  uuid.New(),
				Type:                "data_processing",
				CoordinationMode:    CoordinationModeParallel,
				CompletionMode:      CompletionModeThreshold,
				CompletionThreshold: 5,
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDistributedTaskCalculateProgress(t *testing.T) {
	tests := []struct {
		name     string
		task     *DistributedTask
		expected float64
	}{
		{
			name: "With progress object",
			task: &DistributedTask{
				Progress: &TaskProgress{
					PercentComplete: 75.5,
				},
			},
			expected: 75.5,
		},
		{
			name: "No subtasks",
			task: &DistributedTask{
				SubtaskIDs: []uuid.UUID{},
			},
			expected: 0.0,
		},
		{
			name: "Half completed",
			task: &DistributedTask{
				SubtaskIDs:       []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()},
				ResultsCollected: 2,
			},
			expected: 50.0,
		},
		{
			name: "All completed",
			task: &DistributedTask{
				SubtaskIDs:       []uuid.UUID{uuid.New(), uuid.New(), uuid.New()},
				ResultsCollected: 3,
			},
			expected: 100.0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progress := tt.task.CalculateProgress()
			assert.Equal(t, tt.expected, progress)
		})
	}
}

func TestDistributedTaskIsComplete(t *testing.T) {
	subtaskIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	
	tests := []struct {
		name             string
		completionMode   CompletionMode
		subtaskIDs       []uuid.UUID
		resultsCollected int
		threshold        int
		expected         bool
	}{
		// CompletionModeAll tests
		{"All mode - none complete", CompletionModeAll, subtaskIDs, 0, 0, false},
		{"All mode - some complete", CompletionModeAll, subtaskIDs, 2, 0, false},
		{"All mode - all complete", CompletionModeAll, subtaskIDs, 4, 0, true},
		
		// CompletionModeAny tests
		{"Any mode - none complete", CompletionModeAny, subtaskIDs, 0, 0, false},
		{"Any mode - one complete", CompletionModeAny, subtaskIDs, 1, 0, true},
		{"Any mode - all complete", CompletionModeAny, subtaskIDs, 4, 0, true},
		
		// CompletionModeMajority tests
		{"Majority mode - none complete", CompletionModeMajority, subtaskIDs, 0, 0, false},
		{"Majority mode - minority complete", CompletionModeMajority, subtaskIDs, 2, 0, false},
		{"Majority mode - majority complete", CompletionModeMajority, subtaskIDs, 3, 0, true},
		
		// CompletionModeThreshold tests
		{"Threshold mode - below threshold", CompletionModeThreshold, subtaskIDs, 1, 2, false},
		{"Threshold mode - at threshold", CompletionModeThreshold, subtaskIDs, 2, 2, true},
		{"Threshold mode - above threshold", CompletionModeThreshold, subtaskIDs, 3, 2, true},
		{"Threshold mode - no threshold set", CompletionModeThreshold, subtaskIDs, 4, 0, true},
		
		// CompletionModeBestOf tests
		{"BestOf mode - below threshold", CompletionModeBestOf, subtaskIDs, 2, 3, false},
		{"BestOf mode - at threshold", CompletionModeBestOf, subtaskIDs, 3, 3, true},
		
		// Empty subtasks
		{"Empty subtasks", CompletionModeAll, []uuid.UUID{}, 0, 0, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dt := &DistributedTask{
				CompletionMode:      tt.completionMode,
				SubtaskIDs:          tt.subtaskIDs,
				ResultsCollected:    tt.resultsCollected,
				CompletionThreshold: tt.threshold,
			}
			assert.Equal(t, tt.expected, dt.IsComplete())
		})
	}
}

func TestDistributedTaskGetEstimatedCompletion(t *testing.T) {
	now := time.Now()
	duration := 2 * time.Hour
	
	tests := []struct {
		name     string
		task     *DistributedTask
		hasTime  bool
	}{
		{
			name: "Not started",
			task: &DistributedTask{
				EstimatedDuration: duration,
			},
			hasTime: false,
		},
		{
			name: "Started with no duration",
			task: &DistributedTask{
				StartedAt: &now,
			},
			hasTime: false,
		},
		{
			name: "Started with duration",
			task: &DistributedTask{
				StartedAt:         &now,
				EstimatedDuration: duration,
			},
			hasTime: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimated := tt.task.GetEstimatedCompletion()
			if tt.hasTime {
				require.NotNil(t, estimated)
				assert.Equal(t, now.Add(duration).Unix(), estimated.Unix())
			} else {
				assert.Nil(t, estimated)
			}
		})
	}
}

func TestDistributedTaskFullExample(t *testing.T) {
	// Create a complete distributed task with all fields
	taskID := uuid.New()
	now := time.Now()
	
	dt := &DistributedTask{
		ID:          taskID,
		Type:        "data_analysis",
		Title:       "Analyze Large Dataset",
		Description: "Process and analyze customer behavior data",
		Priority:    TaskPriorityHigh,
		Subtasks: []Subtask{
			{ID: "subtask-1", AgentID: "agent-1", Description: "Process North region"},
			{ID: "subtask-2", AgentID: "agent-2", Description: "Process South region"},
			{ID: "subtask-3", AgentID: "agent-3", Description: "Process East region"},
			{ID: "subtask-4", AgentID: "agent-4", Description: "Process West region"},
		},
		Aggregation: AggregationConfig{
			Method:     "combine_results",
			WaitForAll: true,
			Timeout:    3600,
		},
		SubtaskIDs: []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()},
		
		// Phase 3 fields
		Task: &Task{
			ID:     taskID,
			Status: TaskStatusInProgress,
		},
		CoordinationMode:    CoordinationModeMapReduce,
		CompletionMode:      CompletionModeAll,
		CompletionThreshold: 0,
		
		ExecutionPlan: &ExecutionPlan{
			Phases: []ExecutionPhase{
				{
					ID:       "map-phase",
					Name:     "Map Phase",
					TaskIDs:  []string{"subtask-1", "subtask-2", "subtask-3", "subtask-4"},
					Parallel: true,
					MaxRetry: 3,
				},
				{
					ID:       "reduce-phase",
					Name:     "Reduce Phase",
					TaskIDs:  []string{"reduce-task"},
					Parallel: false,
					MaxRetry: 2,
				},
			},
			SyncPoints: []SyncPoint{
				{
					ID:            "map-complete",
					Name:          "Map Phase Complete",
					RequiredTasks: []string{"subtask-1", "subtask-2", "subtask-3", "subtask-4"},
					Timeout:       30 * time.Minute,
					OnTimeout:     "fail",
				},
			},
			Timeout: 2 * time.Hour,
		},
		
		Partitions: []TaskPartition{
			{ID: "north", RangeStart: 0, RangeEnd: 1000000, Weight: 0.25},
			{ID: "south", RangeStart: 1000001, RangeEnd: 2000000, Weight: 0.25},
			{ID: "east", RangeStart: 2000001, RangeEnd: 3000000, Weight: 0.25},
			{ID: "west", RangeStart: 3000001, RangeEnd: 4000000, Weight: 0.25},
		},
		
		Progress: &TaskProgress{
			TaskID:          taskID,
			TotalSteps:      4,
			CompletedSteps:  2,
			CurrentStep:     "Processing East region",
			PercentComplete: 50.0,
			EstimatedTimeRemaining: 1 * time.Hour,
			LastUpdated:     now,
		},
		
		ResourceUsage: &ResourceUsage{
			TaskID:      taskID,
			CPUPercent:  75.5,
			MemoryMB:    2048,
			DiskIOMB:    500,
			NetworkMB:   100,
			StartTime:   now.Add(-1 * time.Hour),
			LastUpdated: now,
		},
		
		StartedAt:         &now,
		EstimatedDuration: 2 * time.Hour,
		ResultsCollected:  2,
		IntermediateResults: []interface{}{
			map[string]interface{}{"region": "north", "total": 150000},
			map[string]interface{}{"region": "south", "total": 175000},
		},
	}
	
	// Validate the distributed task
	dt.SetDefaults() // Should not override existing values
	err := dt.Validate()
	require.NoError(t, err)
	
	// Test calculations
	assert.Equal(t, 50.0, dt.CalculateProgress())
	assert.False(t, dt.IsComplete())
	
	estimated := dt.GetEstimatedCompletion()
	require.NotNil(t, estimated)
	assert.Equal(t, now.Add(2*time.Hour).Unix(), estimated.Unix())
	
	// Verify coordination and completion modes remain as set
	assert.Equal(t, CoordinationModeMapReduce, dt.CoordinationMode)
	assert.Equal(t, CompletionModeAll, dt.CompletionMode)
}