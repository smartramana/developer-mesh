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
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description,omitempty"`
	Type            string                 `json:"type"`
	Action          string                 `json:"action"`
	AgentID         string                 `json:"agent_id"`
	Input           map[string]interface{} `json:"input"`
	Config          map[string]interface{} `json:"config"`
	Timeout         time.Duration          `json:"timeout"`
	TimeoutSeconds  int                    `json:"timeout_seconds,omitempty"`
	Retries         int                    `json:"retries"`
	RetryPolicy     WorkflowRetryPolicy    `json:"retry_policy,omitempty"`
	ContinueOnError bool                   `json:"continue_on_error"`
	Dependencies    []string               `json:"dependencies"`
}

// WorkflowRetryPolicy defines retry behavior for workflow steps
type WorkflowRetryPolicy struct {
	MaxAttempts int           `json:"max_attempts"`
	BackoffType string        `json:"backoff_type"` // linear, exponential
	InitialWait time.Duration `json:"initial_wait"`
	MaxWait     time.Duration `json:"max_wait"`
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

// ExecutionStatus represents detailed workflow execution status
type ExecutionStatus struct {
	ExecutionID    uuid.UUID              `json:"execution_id"`
	WorkflowID     uuid.UUID              `json:"workflow_id"`
	Status         string                 `json:"status"`
	Progress       int                    `json:"progress"`
	CurrentSteps   []string               `json:"current_steps"`
	CompletedSteps int                    `json:"completed_steps"`
	TotalSteps     int                    `json:"total_steps"`
	StartedAt      time.Time              `json:"started_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	EstimatedEnd   *time.Time             `json:"estimated_end,omitempty"`
	Metrics        map[string]interface{} `json:"metrics,omitempty"`
}

// ApprovalDecision represents an approval decision for a workflow step
type ApprovalDecision struct {
	Approved   bool                   `json:"approved"`
	ApprovedBy string                 `json:"approved_by"`
	Comments   string                 `json:"comments,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// PendingApproval represents a pending approval request
type PendingApproval struct {
	ExecutionID  uuid.UUID              `json:"execution_id"`
	WorkflowID   uuid.UUID              `json:"workflow_id"`
	StepID       string                 `json:"step_id"`
	StepName     string                 `json:"step_name"`
	RequestedAt  time.Time              `json:"requested_at"`
	RequiredBy   []string               `json:"required_by"`
	ApprovedBy   []string               `json:"approved_by,omitempty"`
	RejectedBy   []string               `json:"rejected_by,omitempty"`
	DueBy        *time.Time             `json:"due_by,omitempty"`
	Context      map[string]interface{} `json:"context,omitempty"`
}

// SimulationResult represents the result of a workflow simulation
type SimulationResult struct {
	Success         bool                       `json:"success"`
	ExecutionPath   []string                   `json:"execution_path"`
	EstimatedTime   time.Duration              `json:"estimated_time"`
	ResourceUsage   map[string]interface{}     `json:"resource_usage"`
	PotentialErrors []string                   `json:"potential_errors,omitempty"`
	Warnings        []string                   `json:"warnings,omitempty"`
	StepDetails     map[string]*StepSimulation `json:"step_details"`
}

// StepSimulation represents simulation details for a single step
type StepSimulation struct {
	StepID        string        `json:"step_id"`
	CanExecute    bool          `json:"can_execute"`
	EstimatedTime time.Duration `json:"estimated_time"`
	Requirements  []string      `json:"requirements,omitempty"`
	Issues        []string      `json:"issues,omitempty"`
}

// CompensationAction represents a compensation action for failed workflows
type CompensationAction struct {
	ID          uuid.UUID              `json:"id"`
	ExecutionID uuid.UUID              `json:"execution_id"`
	StepID      string                 `json:"step_id"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Status      string                 `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	ExecutedAt  *time.Time             `json:"executed_at,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
}

// Add methods to make Workflow implement AggregateRoot
func (w *Workflow) GetID() uuid.UUID {
	return w.ID
}

func (w *Workflow) GetType() string {
	return "workflow"
}

func (w *Workflow) GetVersion() int {
	return w.Version
}

// Add methods to make WorkflowExecution implement AggregateRoot
func (e *WorkflowExecution) GetID() uuid.UUID {
	return e.ID
}

func (e *WorkflowExecution) GetType() string {
	return "workflow_execution"
}

func (e *WorkflowExecution) GetVersion() int {
	return 1 // WorkflowExecution doesn't have versioning in the base model
}

// Extend WorkflowExecution with additional fields through helper methods
func (e *WorkflowExecution) GetTriggeredBy() string {
	// Use InitiatedBy field which already exists
	return e.InitiatedBy
}

func (e *WorkflowExecution) GetInput() map[string]interface{} {
	// Input is stored in Context
	if e.Context != nil {
		if input, ok := e.Context["input"].(map[string]interface{}); ok {
			return input
		}
	}
	return nil
}

func (e *WorkflowExecution) SetInput(input map[string]interface{}) {
	if e.Context == nil {
		e.Context = make(JSONMap)
	}
	e.Context["input"] = input
}

func (e *WorkflowExecution) GetSteps() map[string]*StepStatus {
	return e.StepStatuses
}

func (e *WorkflowExecution) GetCreatedAt() time.Time {
	return e.StartedAt
}

// Extend Workflow with additional fields
func (w *Workflow) GetLastExecutedAt() *time.Time {
	if w.Config != nil {
		if lastExec, ok := w.Config["last_executed_at"].(*time.Time); ok {
			return lastExec
		}
	}
	return nil
}

func (w *Workflow) SetLastExecutedAt(t time.Time) {
	if w.Config == nil {
		w.Config = make(JSONMap)
	}
	w.Config["last_executed_at"] = &t
}

// Helper function to get workflow steps as structured objects
func (w *Workflow) GetSteps() []WorkflowStep {
	if w.Steps == nil {
		return nil
	}

	// Convert from JSONMap to []WorkflowStep
	if stepsArray, ok := w.Steps["steps"].([]interface{}); ok {
		result := make([]WorkflowStep, 0, len(stepsArray))
		for _, stepInterface := range stepsArray {
			if stepMap, ok := stepInterface.(map[string]interface{}); ok {
				step := WorkflowStep{
					ID:             getStringField(stepMap, "id"),
					Name:           getStringField(stepMap, "name"),
					Description:    getStringField(stepMap, "description"),
					Type:           getStringField(stepMap, "type"),
					Dependencies:   getStringSliceField(stepMap, "dependencies"),
					TimeoutSeconds: getIntField(stepMap, "timeout_seconds"),
					RetryPolicy:    getRetryPolicy(stepMap),
					Config:         getMapField(stepMap, "config"),
				}
				result = append(result, step)
			}
		}
		return result
	}

	return nil
}

// StepStatus extended methods
func (s *StepStatus) GetName() string {
	// Name might be stored in metadata
	if s.Input != nil {
		if name, ok := s.Input["step_name"].(string); ok {
			return name
		}
	}
	return ""
}

func (s *StepStatus) GetAttempts() int {
	return s.RetryCount
}

func (s *StepStatus) GetCreatedAt() time.Time {
	if s.StartedAt != nil {
		return *s.StartedAt
	}
	return time.Time{}
}

// Helper functions for field extraction
func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getIntField(m map[string]interface{}, key string) int {
	if val, ok := m[key].(int); ok {
		return val
	}
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	return 0
}

func getStringSliceField(m map[string]interface{}, key string) []string {
	if val, ok := m[key].([]interface{}); ok {
		result := make([]string, 0, len(val))
		for _, v := range val {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func getMapField(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key].(map[string]interface{}); ok {
		return val
	}
	return nil
}

func getRetryPolicy(m map[string]interface{}) WorkflowRetryPolicy {
	if retryMap, ok := m["retry_policy"].(map[string]interface{}); ok {
		return WorkflowRetryPolicy{
			MaxAttempts: getIntField(retryMap, "max_attempts"),
			BackoffType: getStringField(retryMap, "backoff_type"),
		}
	}
	return WorkflowRetryPolicy{}
}

// Step status constants
const (
	StepStatusPending   = "pending"
	StepStatusRunning   = "running"
	StepStatusCompleted = "completed"
	StepStatusFailed    = "failed"
	StepStatusSkipped   = "skipped"
)