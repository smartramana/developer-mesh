package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
)

// TaskService handles task lifecycle with production features
type TaskService interface {
	// Task lifecycle with idempotency
	Create(ctx context.Context, task *models.Task, idempotencyKey string) error
	CreateBatch(ctx context.Context, tasks []*models.Task) error
	Get(ctx context.Context, id uuid.UUID) (*models.Task, error)
	GetBatch(ctx context.Context, ids []uuid.UUID) ([]*models.Task, error)
	Update(ctx context.Context, task *models.Task) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Task assignment with load balancing
	AssignTask(ctx context.Context, taskID uuid.UUID, agentID string) error
	AutoAssignTask(ctx context.Context, taskID uuid.UUID, strategy AssignmentStrategy) error
	DelegateTask(ctx context.Context, delegation *models.TaskDelegation) error
	AcceptTask(ctx context.Context, taskID uuid.UUID, agentID string) error
	RejectTask(ctx context.Context, taskID uuid.UUID, agentID string, reason string) error

	// Task execution with monitoring
	StartTask(ctx context.Context, taskID uuid.UUID, agentID string) error
	UpdateProgress(ctx context.Context, taskID uuid.UUID, progress int, message string) error
	CompleteTask(ctx context.Context, taskID uuid.UUID, agentID string, result interface{}) error
	FailTask(ctx context.Context, taskID uuid.UUID, agentID string, errorMsg string) error
	RetryTask(ctx context.Context, taskID uuid.UUID) error

	// Advanced querying with caching
	GetAgentTasks(ctx context.Context, agentID string, filters interfaces.TaskFilters) ([]*models.Task, error)
	GetAvailableTasks(ctx context.Context, agentID string, capabilities []string) ([]*models.Task, error)
	SearchTasks(ctx context.Context, query string, filters interfaces.TaskFilters) ([]*models.Task, error)
	GetTaskTimeline(ctx context.Context, taskID uuid.UUID) ([]*models.TaskEvent, error)

	// Distributed task management
	CreateDistributedTask(ctx context.Context, task *models.DistributedTask) error
	SubmitSubtaskResult(ctx context.Context, parentTaskID, subtaskID uuid.UUID, result interface{}) error
	GetTaskTree(ctx context.Context, rootTaskID uuid.UUID) (*models.TaskTree, error)
	CancelTaskTree(ctx context.Context, rootTaskID uuid.UUID, reason string) error

	// Workflow integration
	CreateWorkflowTask(ctx context.Context, workflowID, stepID uuid.UUID, params map[string]interface{}) (*models.Task, error)
	CompleteWorkflowTask(ctx context.Context, taskID uuid.UUID, output interface{}) error

	// Analytics and reporting
	GetTaskStats(ctx context.Context, filters interfaces.TaskFilters) (*models.TaskStats, error)
	GetAgentPerformance(ctx context.Context, agentID string, period time.Duration) (*models.AgentPerformance, error)
	GenerateTaskReport(ctx context.Context, filters interfaces.TaskFilters, format string) ([]byte, error)

	// Maintenance
	ArchiveCompletedTasks(ctx context.Context, before time.Time) (int64, error)
	RebalanceTasks(ctx context.Context) error
}

// AssignmentStrategy defines how tasks are assigned to agents
type AssignmentStrategy interface {
	Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error)
	GetName() string
}

// AgentService provides agent management functionality
type AgentService interface {
	GetAgent(ctx context.Context, agentID string) (*models.Agent, error)
	GetAvailableAgents(ctx context.Context) ([]*models.Agent, error)
	GetAgentCapabilities(ctx context.Context, agentID string) ([]string, error)
	UpdateAgentStatus(ctx context.Context, agentID string, status string) error
	GetAgentWorkload(ctx context.Context, agentID string) (*models.AgentWorkload, error)
}