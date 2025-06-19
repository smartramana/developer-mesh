// Package agent provides interfaces and implementations for agent entities
package agent

import (
	"context"

	"github.com/google/uuid"
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// Filter defines a filter map for repository operations
// This avoids importing pkg/repository to prevent import cycles
type Filter map[string]any

// FilterFromTenantID creates a filter for tenant ID
func FilterFromTenantID(tenantID string) Filter {
	return Filter{"tenant_id": tenantID}
}

// FilterFromIDs creates a filter for tenant ID and agent ID
func FilterFromIDs(tenantID, id string) Filter {
	return Filter{
		"tenant_id": tenantID,
		"id":        id,
	}
}

// Repository defines operations for managing agent entities
// It follows the generic repository pattern while preserving API-specific methods
type Repository interface {
	// Core repository methods - aligned with generic Repository[T] interface
	Create(ctx context.Context, agent *models.Agent) error
	Get(ctx context.Context, id string) (*models.Agent, error)
	List(ctx context.Context, filter Filter) ([]*models.Agent, error)
	Update(ctx context.Context, agent *models.Agent) error
	Delete(ctx context.Context, id string) error

	// API-specific methods - preserved for backward compatibility
	// These methods delegate to the core methods in the implementation
	CreateAgent(ctx context.Context, agent *models.Agent) error
	GetAgentByID(ctx context.Context, id string, tenantID string) (*models.Agent, error)
	ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error)
	UpdateAgent(ctx context.Context, agent *models.Agent) error
	DeleteAgent(ctx context.Context, id string) error

	// Enhanced repository methods for production
	GetByStatus(ctx context.Context, status models.AgentStatus) ([]*models.Agent, error)
	GetWorkload(ctx context.Context, agentID uuid.UUID) (*models.AgentWorkload, error)
	UpdateWorkload(ctx context.Context, workload *models.AgentWorkload) error
	GetLeastLoadedAgent(ctx context.Context, capability models.AgentCapability) (*models.Agent, error)
}

// These functions are now defined above with the Filter type
