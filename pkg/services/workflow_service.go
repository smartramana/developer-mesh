package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
)

// WorkflowService orchestrates multi-agent workflows with saga pattern
type WorkflowService interface {
	// Workflow management
	CreateWorkflow(ctx context.Context, workflow *models.Workflow) error
	GetWorkflow(ctx context.Context, id uuid.UUID) (*models.Workflow, error)
	UpdateWorkflow(ctx context.Context, workflow *models.Workflow) error
	DeleteWorkflow(ctx context.Context, id uuid.UUID) error
	ListWorkflows(ctx context.Context, filters interfaces.WorkflowFilters) ([]*models.Workflow, error)
	SearchWorkflows(ctx context.Context, query string) ([]*models.Workflow, error)

	// Execution management
	CreateExecution(ctx context.Context, execution *models.WorkflowExecution) error
	ExecuteWorkflow(ctx context.Context, workflowID uuid.UUID, input map[string]interface{}, idempotencyKey string) (*models.WorkflowExecution, error)
	ExecuteWorkflowStep(ctx context.Context, executionID uuid.UUID, stepID string) error
	StartWorkflow(ctx context.Context, workflowID uuid.UUID, initiatorID string, input map[string]interface{}) (*models.WorkflowExecution, error)
	GetExecution(ctx context.Context, executionID uuid.UUID) (*models.WorkflowExecution, error)
	ListExecutions(ctx context.Context, workflowID uuid.UUID, filters interfaces.ExecutionFilters) ([]*models.WorkflowExecution, error)
	GetExecutionStatus(ctx context.Context, executionID uuid.UUID) (*models.ExecutionStatus, error)
	GetExecutionTimeline(ctx context.Context, executionID uuid.UUID) ([]*models.ExecutionEvent, error)
	GetExecutionHistory(ctx context.Context, workflowID uuid.UUID) ([]*models.WorkflowExecution, error)

	// Execution control
	UpdateExecution(ctx context.Context, execution *models.WorkflowExecution) error
	PauseExecution(ctx context.Context, executionID uuid.UUID, reason string) error
	ResumeExecution(ctx context.Context, executionID uuid.UUID) error
	CancelExecution(ctx context.Context, executionID uuid.UUID, reason string) error
	RetryExecution(ctx context.Context, executionID uuid.UUID, fromStep string) error
	
	// Step management
	CompleteStep(ctx context.Context, executionID uuid.UUID, stepID string, output map[string]interface{}) error
	FailStep(ctx context.Context, executionID uuid.UUID, stepID string, reason string, details map[string]interface{}) error
	RetryStep(ctx context.Context, executionID uuid.UUID, stepID string) error
	GetCurrentStep(ctx context.Context, executionID uuid.UUID) (*models.StepExecution, error)
	GetPendingSteps(ctx context.Context, executionID uuid.UUID) ([]*models.StepExecution, error)
	GetStepExecution(ctx context.Context, executionID uuid.UUID, stepID string) (*models.StepExecution, error)

	// Approval management
	SubmitApproval(ctx context.Context, executionID uuid.UUID, stepID string, approval *models.ApprovalDecision) error
	GetPendingApprovals(ctx context.Context, approverID string) ([]*models.PendingApproval, error)

	// Template management
	CreateWorkflowTemplate(ctx context.Context, template *models.WorkflowTemplate) error
	GetWorkflowTemplate(ctx context.Context, templateID uuid.UUID) (*models.WorkflowTemplate, error)
	ListWorkflowTemplates(ctx context.Context) ([]*models.WorkflowTemplate, error)
	CreateFromTemplate(ctx context.Context, templateID uuid.UUID, params map[string]interface{}) (*models.Workflow, error)

	// Validation and simulation
	ValidateWorkflow(ctx context.Context, workflow *models.Workflow) error
	SimulateWorkflow(ctx context.Context, workflow *models.Workflow, input map[string]interface{}) (*models.SimulationResult, error)

	// Analytics and reporting
	GetWorkflowStats(ctx context.Context, workflowID uuid.UUID, period time.Duration) (*interfaces.WorkflowStats, error)
	GetWorkflowHistory(ctx context.Context, workflowID uuid.UUID, limit int, offset int) ([]*models.WorkflowExecution, error)
	GetWorkflowMetrics(ctx context.Context, workflowID uuid.UUID) (*models.WorkflowMetrics, error)
	GenerateWorkflowReport(ctx context.Context, filters interfaces.WorkflowFilters, format string) ([]byte, error)

	// Maintenance
	ArchiveCompletedExecutions(ctx context.Context, before time.Time) (int64, error)

	// Advanced features
	CreateBranchingPath(ctx context.Context, executionID uuid.UUID, branchPoint string, conditions map[string]interface{}) error
	MergeBranchingPaths(ctx context.Context, executionID uuid.UUID, branchIDs []string) error
	CreateCompensation(ctx context.Context, executionID uuid.UUID, failedStep string, compensation *models.CompensationAction) error
	ExecuteCompensation(ctx context.Context, executionID uuid.UUID) error
}