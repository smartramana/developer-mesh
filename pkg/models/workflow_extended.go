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
	RequiredAgents   []string          `json:"required_agents"`
	AgentRoles       map[string]string `json:"agent_roles"`
	CoordinationMode string            `json:"coordination_mode"` // parallel, sequential, consensus
	VotingStrategy   string            `json:"voting_strategy"`   // majority, unanimous, weighted
	SyncPoints       []string          `json:"sync_points"`       // Step IDs where agents must synchronize
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
	OnFailure       string                 `json:"on_failure,omitempty"` // fail_workflow, continue, compensate
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
	WorkflowID      uuid.UUID              `json:"workflow_id"`
	TotalExecutions int64                  `json:"total_executions"`
	SuccessfulRuns  int64                  `json:"successful_runs"`
	FailedRuns      int64                  `json:"failed_runs"`
	AverageRunTime  time.Duration          `json:"average_run_time"`
	MedianRunTime   time.Duration          `json:"median_run_time"`
	P95RunTime      time.Duration          `json:"p95_run_time"`
	StepMetrics     map[string]StepMetrics `json:"step_metrics"`
}

// StepMetrics represents metrics for a workflow step
type StepMetrics struct {
	SuccessRate    float64        `json:"success_rate"`
	AverageTime    time.Duration  `json:"average_time"`
	FailureReasons map[string]int `json:"failure_reasons"`
}

// ExecutionTrace represents a trace of workflow execution
type ExecutionTrace struct {
	ExecutionID uuid.UUID              `json:"execution_id"`
	Steps       []StepTrace            `json:"steps"`
	Timeline    []TimelineEvent        `json:"timeline"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// StepTrace represents a trace of a step execution
type StepTrace struct {
	StepID    string                 `json:"step_id"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
	Duration  time.Duration          `json:"duration"`
	Status    string                 `json:"status"`
	Input     map[string]interface{} `json:"input"`
	Output    map[string]interface{} `json:"output"`
	Error     string                 `json:"error,omitempty"`
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
	WorkflowID              uuid.UUID                `json:"workflow_id"`
	Period                  time.Duration            `json:"period"`
	BottleneckSteps         []string                 `json:"bottleneck_steps"`
	FailurePredictors       map[string]float64       `json:"failure_predictors"`
	OptimizationSuggestions []OptimizationSuggestion `json:"optimization_suggestions"`
	TrendAnalysis           map[string]TrendData     `json:"trend_analysis"`
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
	ID         uuid.UUID              `json:"id"`
	Approved   bool                   `json:"approved"`
	ApprovedBy string                 `json:"approved_by"`
	Comments   string                 `json:"comments,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// Add methods to make ApprovalDecision implement AggregateRoot
func (a *ApprovalDecision) GetID() uuid.UUID {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return a.ID
}

func (a *ApprovalDecision) GetType() string {
	return "approval_decision"
}

func (a *ApprovalDecision) GetVersion() int {
	return 1
}

// PendingApproval represents a pending approval request
type PendingApproval struct {
	ExecutionID uuid.UUID              `json:"execution_id"`
	WorkflowID  uuid.UUID              `json:"workflow_id"`
	StepID      string                 `json:"step_id"`
	StepName    string                 `json:"step_name"`
	RequestedAt time.Time              `json:"requested_at"`
	RequiredBy  []string               `json:"required_by"`
	ApprovedBy  []string               `json:"approved_by,omitempty"`
	RejectedBy  []string               `json:"rejected_by,omitempty"`
	DueBy       *time.Time             `json:"due_by,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
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

// Add methods to make WorkflowTemplate implement AggregateRoot
func (t *WorkflowTemplate) GetID() uuid.UUID {
	return t.ID
}

func (t *WorkflowTemplate) GetType() string {
	return "workflow_template"
}

func (t *WorkflowTemplate) GetVersion() int {
	return 1 // Templates don't have versioning yet
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
	// Steps is now directly a WorkflowSteps type
	return []WorkflowStep(w.Steps)
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


// Step status constants
const (
	StepStatusPending          = "pending"
	StepStatusQueued           = "queued"
	StepStatusRunning          = "running"
	StepStatusCompleted        = "completed"
	StepStatusFailed           = "failed"
	StepStatusSkipped          = "skipped"
	StepStatusRetrying         = "retrying"
	StepStatusCancelling       = "cancelling"
	StepStatusCancelled        = "cancelled"
	StepStatusTimeout          = "timeout"
	StepStatusAwaitingApproval = "awaiting_approval"
)

// WorkflowStatus constants for workflow definitions
const (
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusInactive WorkflowStatus = "inactive"
	WorkflowStatusDraft    WorkflowStatus = "draft"
	WorkflowStatusArchived WorkflowStatus = "archived"
)

// Document represents a shared document for collaboration
type Document struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	TenantID    uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	WorkspaceID *uuid.UUID `json:"workspace_id,omitempty" db:"workspace_id"`
	Title       string     `json:"title" db:"title"`
	Type        string     `json:"type" db:"type"`
	Content     string     `json:"content" db:"content"`
	ContentType string     `json:"content_type" db:"content_type"`
	CreatedBy   string     `json:"created_by" db:"created_by"`
	Version     int        `json:"version" db:"version"`
	Permissions JSONMap    `json:"permissions" db:"permissions"`
	Metadata    JSONMap    `json:"metadata" db:"metadata"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// DocumentChange represents a change to a document (for CRDT)
type DocumentChange struct {
	ID         uuid.UUID `json:"id" db:"id"`
	DocumentID uuid.UUID `json:"document_id" db:"document_id"`
	AgentID    string    `json:"agent_id" db:"agent_id"`
	ChangeType string    `json:"change_type" db:"change_type"`
	Position   int       `json:"position" db:"position"`
	Content    string    `json:"content" db:"content"`
	Length     int       `json:"length" db:"length"`
	Metadata   JSONMap   `json:"metadata" db:"metadata"`
	Timestamp  time.Time `json:"timestamp" db:"timestamp"`
}

// Removed StateOperation - it's defined in workspace_extended.go

// Enhanced WorkflowStep for multi-agent support
type MultiAgentWorkflowStep struct {
	ID                   uuid.UUID `json:"id" db:"id"`
	WorkflowID           uuid.UUID `json:"workflow_id" db:"workflow_id"`
	Name                 string    `json:"name" db:"name"`
	Type                 string    `json:"type" db:"type"`
	Order                int       `json:"order" db:"order"`
	Config               JSONMap   `json:"config" db:"config"`
	Dependencies         []string  `json:"dependencies" db:"dependencies"`
	AssignedAgents       []string  `json:"assigned_agents" db:"assigned_agents"`
	RequiredCapabilities []string  `json:"required_capabilities" db:"required_capabilities"`
	TimeoutSeconds       int       `json:"timeout_seconds" db:"timeout_seconds"`
	RetryPolicy          JSONMap   `json:"retry_policy" db:"retry_policy"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
}

// ExecutionContext for workflow executions
type ExecutionContext struct {
	ExecutionID   uuid.UUID              `json:"execution_id"`
	WorkflowID    uuid.UUID              `json:"workflow_id"`
	Status        string                 `json:"status"`
	CurrentStep   string                 `json:"current_step"`
	TotalSteps    int                    `json:"total_steps"`
	StartedAt     time.Time              `json:"started_at"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty"`
	ExecutionTime time.Duration          `json:"execution_time"`
	StepResults   map[string]interface{} `json:"step_results"`
}
