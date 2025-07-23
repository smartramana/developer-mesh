package interfaces

import (
	"context"
	"io"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository/types"
	"github.com/google/uuid"
)

// TaskFilters defines comprehensive filtering options for task queries
type TaskFilters struct {
	Status        []string
	Priority      []string
	Types         []string
	AssignedTo    *string
	CreatedBy     *string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	UpdatedAfter  *time.Time
	ParentTaskID  *uuid.UUID
	HasSubtasks   *bool
	Tags          []string
	Capabilities  []string
	Limit         int
	Offset        int
	Cursor        string // For cursor-based pagination
	SortBy        string
	SortOrder     types.SortOrder
}

// TaskStats represents aggregated task statistics
type TaskStats struct {
	TotalCount        int64
	CompletedCount    int64
	FailedCount       int64
	AverageCompletion time.Duration
	P95Completion     time.Duration
	P99Completion     time.Duration
	ByStatus          map[string]int64
	ByPriority        map[string]int64
	ByAgent           map[string]int64
}

// TaskRepository defines the comprehensive interface for task persistence
type TaskRepository interface {
	// Transaction support
	WithTx(tx types.Transaction) TaskRepository
	BeginTx(ctx context.Context, opts *types.TxOptions) (types.Transaction, error)

	// Basic CRUD operations with optimistic locking
	Create(ctx context.Context, task *models.Task) error
	CreateBatch(ctx context.Context, tasks []*models.Task) error
	Get(ctx context.Context, id uuid.UUID) (*models.Task, error)
	GetBatch(ctx context.Context, ids []uuid.UUID) ([]*models.Task, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (*models.Task, error) // SELECT FOR UPDATE
	Update(ctx context.Context, task *models.Task) error
	UpdateWithVersion(ctx context.Context, task *models.Task, expectedVersion int) error
	Delete(ctx context.Context, id uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// Query operations with cursor pagination
	ListByAgent(ctx context.Context, agentID string, filters types.TaskFilters) (*TaskPage, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID, filters types.TaskFilters) (*TaskPage, error)
	GetSubtasks(ctx context.Context, parentTaskID uuid.UUID) ([]*models.Task, error)
	GetTaskTree(ctx context.Context, rootTaskID uuid.UUID, maxDepth int) (*models.TaskTree, error)
	StreamTasks(ctx context.Context, filters types.TaskFilters) (<-chan *models.Task, <-chan error)

	// Task assignment operations with audit
	AssignToAgent(ctx context.Context, taskID uuid.UUID, agentID string, assignedBy string) error
	UnassignTask(ctx context.Context, taskID uuid.UUID, reason string) error
	UpdateStatus(ctx context.Context, taskID uuid.UUID, status string, metadata map[string]interface{}) error
	BulkUpdateStatus(ctx context.Context, updates []StatusUpdate) error
	IncrementRetryCount(ctx context.Context, taskID uuid.UUID) (int, error)

	// Delegation operations
	CreateDelegation(ctx context.Context, delegation *models.TaskDelegation) error
	GetDelegationHistory(ctx context.Context, taskID uuid.UUID) ([]*models.TaskDelegation, error)
	GetDelegationsToAgent(ctx context.Context, agentID string, since time.Time) ([]*models.TaskDelegation, error)
	GetDelegationChain(ctx context.Context, taskID uuid.UUID) ([]*models.DelegationNode, error)

	// Bulk operations with COPY support
	BulkInsert(ctx context.Context, tasks []*models.Task) error
	BulkUpdate(ctx context.Context, updates []TaskUpdate) error
	BatchUpdateStatus(ctx context.Context, taskIDs []uuid.UUID, status string) error
	ArchiveTasks(ctx context.Context, before time.Time) (int64, error)

	// Execution and scheduling
	GetTasksForExecution(ctx context.Context, agentID string, limit int) ([]*models.Task, error)
	GetOverdueTasks(ctx context.Context, threshold time.Duration) ([]*models.Task, error)
	GetTasksBySchedule(ctx context.Context, schedule string) ([]*models.Task, error)
	LockTaskForExecution(ctx context.Context, taskID uuid.UUID, agentID string, duration time.Duration) error

	// Analytics and reporting
	GetTaskStats(ctx context.Context, tenantID uuid.UUID, period time.Duration) (*TaskStats, error)
	GetAgentWorkload(ctx context.Context, agentIDs []string) (map[string]*AgentWorkload, error)
	GetTaskTimeline(ctx context.Context, taskID uuid.UUID) ([]*TaskEvent, error)
	GenerateTaskReport(ctx context.Context, filters types.TaskFilters, format string) (io.Reader, error)

	// Search operations
	SearchTasks(ctx context.Context, query string, filters types.TaskFilters) (*TaskSearchResult, error)
	GetSimilarTasks(ctx context.Context, taskID uuid.UUID, limit int) ([]*models.Task, error)

	// Maintenance operations
	VacuumTasks(ctx context.Context) error
	RebuildTaskIndexes(ctx context.Context) error
	ValidateTaskIntegrity(ctx context.Context) (*types.IntegrityReport, error)
}

// Supporting types
type TaskPage struct {
	Tasks      []*models.Task
	TotalCount int64
	HasMore    bool
	NextCursor string
}

type StatusUpdate struct {
	TaskID   uuid.UUID
	Status   string
	Metadata map[string]interface{}
}

type TaskUpdate struct {
	TaskID  uuid.UUID
	Updates map[string]interface{}
}

type AgentWorkload struct {
	PendingCount    int
	ActiveCount     int
	CompletedToday  int
	AverageTime     time.Duration
	CurrentCapacity float64
}

type TaskEvent struct {
	Timestamp time.Time
	EventType string
	AgentID   string
	Details   map[string]interface{}
}

type TaskSearchResult struct {
	Tasks      []*models.Task
	TotalCount int64
	Facets     map[string]map[string]int64
	Highlights map[uuid.UUID][]string
}
