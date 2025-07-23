package interfaces

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository/types"
	"github.com/google/uuid"
)

// WorkflowFilters defines filtering options for workflow queries
type WorkflowFilters struct {
	Type          []string
	IsActive      *bool
	CreatedBy     *string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	Tags          []string
	Limit         int
	Offset        int
	SortBy        string
	SortOrder     types.SortOrder
}

// ExecutionFilters defines filtering options for workflow execution queries
type ExecutionFilters struct {
	Status        []string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	TriggeredBy   *string
	Limit         int
	Offset        int
	SortBy        string
	SortOrder     types.SortOrder
}

// WorkflowStats represents workflow execution statistics
type WorkflowStats struct {
	TotalRuns      int64
	SuccessfulRuns int64
	FailedRuns     int64
	AverageRuntime time.Duration
	P95Runtime     time.Duration
	ByStatus       map[string]int64
}

// WorkflowRepository defines the interface for workflow persistence
type WorkflowRepository interface {
	// Transaction support
	WithTx(tx types.Transaction) WorkflowRepository
	BeginTx(ctx context.Context, opts *types.TxOptions) (types.Transaction, error)

	// Basic CRUD operations
	Create(ctx context.Context, workflow *models.Workflow) error
	Get(ctx context.Context, id uuid.UUID) (*models.Workflow, error)
	Update(ctx context.Context, workflow *models.Workflow) error
	Delete(ctx context.Context, id uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// Query operations
	List(ctx context.Context, tenantID uuid.UUID, filters WorkflowFilters) ([]*models.Workflow, error)
	ListByType(ctx context.Context, workflowType string) ([]*models.Workflow, error)
	GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*models.Workflow, error)
	SearchWorkflows(ctx context.Context, query string, filters WorkflowFilters) ([]*models.Workflow, error)

	// Execution operations
	CreateExecution(ctx context.Context, execution *models.WorkflowExecution) error
	GetExecution(ctx context.Context, executionID uuid.UUID) (*models.WorkflowExecution, error)
	UpdateExecution(ctx context.Context, execution *models.WorkflowExecution) error
	ListExecutions(ctx context.Context, workflowID uuid.UUID, limit int) ([]*models.WorkflowExecution, error)
	GetActiveExecutions(ctx context.Context, workflowID uuid.UUID) ([]*models.WorkflowExecution, error)

	// Step operations
	UpdateStepStatus(ctx context.Context, executionID uuid.UUID, stepID string, status string, output map[string]interface{}) error
	GetStepStatus(ctx context.Context, executionID uuid.UUID, stepID string) (*models.StepStatus, error)

	// Analytics
	GetWorkflowStats(ctx context.Context, workflowID uuid.UUID, period time.Duration) (*WorkflowStats, error)
	GetExecutionTimeline(ctx context.Context, executionID uuid.UUID) ([]*models.ExecutionEvent, error)

	// Maintenance
	ArchiveOldExecutions(ctx context.Context, before time.Time) (int64, error)
	ValidateWorkflowIntegrity(ctx context.Context, workflowID uuid.UUID) error
}
