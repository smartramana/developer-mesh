package models

import (
	"time"

	"github.com/google/uuid"
)

// WorkflowVersion represents a version of a workflow
type WorkflowVersion struct {
	ID         uuid.UUID              `json:"id" db:"id"`
	WorkflowID uuid.UUID              `json:"workflow_id" db:"workflow_id"`
	Version    int                    `json:"version" db:"version"`
	Changes    string                 `json:"changes" db:"changes"`
	Definition map[string]interface{} `json:"definition" db:"definition"`
	CreatedBy  string                 `json:"created_by" db:"created_by"`
	CreatedAt  time.Time              `json:"created_at" db:"created_at"`
}

// WorkflowExecutionRequest represents a request to execute a workflow
type WorkflowExecutionRequest struct {
	WorkflowID  uuid.UUID              `json:"workflow_id"`
	Input       map[string]interface{} `json:"input"`
	Context     map[string]interface{} `json:"context"`
	TriggeredBy string                 `json:"triggered_by"`
	Priority    string                 `json:"priority"`
}

// CollaborativeWorkflow represents a workflow designed for multi-agent collaboration
type CollaborativeWorkflow struct {
	*Workflow
	RequiredAgents   []string               `json:"required_agents"`
	AgentRoles       map[string]string      `json:"agent_roles"`
	CoordinationMode string                 `json:"coordination_mode"` // parallel, sequential, consensus
	VotingStrategy   string                 `json:"voting_strategy"`   // majority, unanimous, weighted
	SyncPoints       []string               `json:"sync_points"`       // Step IDs where agents must synchronize
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Action      string                 `json:"action"`
	AgentID     string                 `json:"agent_id"`
	Input       map[string]interface{} `json:"input"`
	Config      map[string]interface{} `json:"config"`
	Timeout     time.Duration          `json:"timeout"`
	Retries     int                    `json:"retries"`
	ContinueOnError bool               `json:"continue_on_error"`
	Dependencies []string              `json:"dependencies"`
}


// WorkflowMetrics represents metrics for a workflow
type WorkflowMetrics struct {
	WorkflowID        uuid.UUID              `json:"workflow_id"`
	TotalExecutions   int64                  `json:"total_executions"`
	SuccessfulRuns    int64                  `json:"successful_runs"`
	FailedRuns        int64                  `json:"failed_runs"`
	AverageRunTime    time.Duration          `json:"average_run_time"`
	MedianRunTime     time.Duration          `json:"median_run_time"`
	P95RunTime        time.Duration          `json:"p95_run_time"`
	StepMetrics       map[string]StepMetrics `json:"step_metrics"`
}

// StepMetrics represents metrics for a workflow step
type StepMetrics struct {
	SuccessRate   float64       `json:"success_rate"`
	AverageTime   time.Duration `json:"average_time"`
	FailureReasons map[string]int `json:"failure_reasons"`
}

// ExecutionTrace represents a trace of workflow execution
type ExecutionTrace struct {
	ExecutionID uuid.UUID     `json:"execution_id"`
	Steps       []StepTrace   `json:"steps"`
	Timeline    []TimelineEvent `json:"timeline"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// StepTrace represents a trace of a step execution
type StepTrace struct {
	StepID      string                 `json:"step_id"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     time.Time              `json:"end_time"`
	Duration    time.Duration          `json:"duration"`
	Status      string                 `json:"status"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
	Error       string                 `json:"error,omitempty"`
}

// TimelineEvent represents an event in the execution timeline
type TimelineEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	Type        string                 `json:"type"`
	StepID      string                 `json:"step_id,omitempty"`
	Description string                 `json:"description"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// WorkflowInsights represents insights about workflow performance
type WorkflowInsights struct {
	WorkflowID           uuid.UUID                    `json:"workflow_id"`
	Period               time.Duration                `json:"period"`
	BottleneckSteps      []string                     `json:"bottleneck_steps"`
	FailurePredictors    map[string]float64           `json:"failure_predictors"`
	OptimizationSuggestions []OptimizationSuggestion   `json:"optimization_suggestions"`
	TrendAnalysis        map[string]TrendData         `json:"trend_analysis"`
}

// OptimizationSuggestion represents a suggestion for workflow optimization
type OptimizationSuggestion struct {
	Type        string  `json:"type"`
	StepID      string  `json:"step_id,omitempty"`
	Description string  `json:"description"`
	Impact      float64 `json:"impact"` // Estimated improvement percentage
}

// TrendData represents trend information
type TrendData struct {
	Current  float64 `json:"current"`
	Previous float64 `json:"previous"`
	Change   float64 `json:"change_percentage"`
	Trend    string  `json:"trend"` // increasing, decreasing, stable
}

// WorkflowTemplate represents a reusable workflow template
type WorkflowTemplate struct {
	ID          uuid.UUID              `json:"id" db:"id"`
	Name        string                 `json:"name" db:"name"`
	Description string                 `json:"description" db:"description"`
	Category    string                 `json:"category" db:"category"`
	Definition  map[string]interface{} `json:"definition" db:"definition"`
	Parameters  []TemplateParameter    `json:"parameters" db:"parameters"`
	CreatedBy   string                 `json:"created_by" db:"created_by"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
}

// TemplateParameter represents a parameter in a workflow template
type TemplateParameter struct {
	Name         string      `json:"name"`
	Type         string      `json:"type"`
	Description  string      `json:"description"`
	Required     bool        `json:"required"`
	DefaultValue interface{} `json:"default_value,omitempty"`
}