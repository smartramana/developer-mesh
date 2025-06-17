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
	Create(ctx context.Context, workflow *models.Workflow) error
	Get(ctx context.Context, id uuid.UUID) (*models.Workflow, error)
	Update(ctx context.Context, workflow *models.Workflow) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filters interfaces.WorkflowFilters) ([]*models.Workflow, error)

	// Version control
	CreateVersion(ctx context.Context, workflowID uuid.UUID, changes string) (*models.Workflow, error)
	GetVersion(ctx context.Context, workflowID uuid.UUID, version int) (*models.Workflow, error)
	ListVersions(ctx context.Context, workflowID uuid.UUID) ([]*models.WorkflowVersion, error)

	// Execution with monitoring
	Execute(ctx context.Context, workflowID uuid.UUID, input map[string]interface{}) (*models.WorkflowExecution, error)
	ExecuteWithContext(ctx context.Context, execution *models.WorkflowExecutionRequest) (*models.WorkflowExecution, error)
	GetExecution(ctx context.Context, executionID uuid.UUID) (*models.WorkflowExecution, error)
	ListExecutions(ctx context.Context, workflowID uuid.UUID, filters interfaces.ExecutionFilters) ([]*models.WorkflowExecution, error)

	// Execution control
	PauseExecution(ctx context.Context, executionID uuid.UUID) error
	ResumeExecution(ctx context.Context, executionID uuid.UUID) error
	CancelExecution(ctx context.Context, executionID uuid.UUID, reason string) error
	RetryExecution(ctx context.Context, executionID uuid.UUID, fromStep string) error

	// Collaborative workflows
	CreateCollaborative(ctx context.Context, workflow *models.CollaborativeWorkflow) error
	ExecuteCollaborative(ctx context.Context, workflowID uuid.UUID, input map[string]interface{}) (*models.WorkflowExecution, error)

	// Step management
	CompleteStep(ctx context.Context, executionID uuid.UUID, stepID string, output interface{}) error
	FailStep(ctx context.Context, executionID uuid.UUID, stepID string, errorMsg string) error
	GetNextSteps(ctx context.Context, executionID uuid.UUID) ([]*models.WorkflowStep, error)
	GetStepStatus(ctx context.Context, executionID uuid.UUID, stepID string) (*models.StepStatus, error)

	// Analytics
	GetWorkflowMetrics(ctx context.Context, workflowID uuid.UUID) (*models.WorkflowMetrics, error)
	GetExecutionTrace(ctx context.Context, executionID uuid.UUID) (*models.ExecutionTrace, error)
	GetWorkflowInsights(ctx context.Context, workflowID uuid.UUID, period time.Duration) (*models.WorkflowInsights, error)

	// Template management
	CreateTemplate(ctx context.Context, template *models.WorkflowTemplate) error
	GetTemplate(ctx context.Context, templateID uuid.UUID) (*models.WorkflowTemplate, error)
	ListTemplates(ctx context.Context, category string) ([]*models.WorkflowTemplate, error)
	CreateFromTemplate(ctx context.Context, templateID uuid.UUID, params map[string]interface{}) (*models.Workflow, error)
}