// Package agent provides interfaces and implementations for agent entities
package agent

import (
	"context"
	
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// Repository defines operations for managing agent entities
type Repository interface {
	// Core repository methods
	Create(ctx context.Context, agent *models.Agent) error
	Get(ctx context.Context, id string) (*models.Agent, error)
	List(ctx context.Context, filter map[string]interface{}) ([]*models.Agent, error)
	Update(ctx context.Context, agent *models.Agent) error
	Delete(ctx context.Context, id string) error
	
	// API-specific methods
	CreateAgent(ctx context.Context, agent *models.Agent) error
	GetAgentByID(ctx context.Context, id string, tenantID string) (*models.Agent, error)
	ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error)
	UpdateAgent(ctx context.Context, agent *models.Agent) error
	DeleteAgent(ctx context.Context, id string) error
}

// FilterFromTenantID creates a filter map from a tenant ID
func FilterFromTenantID(tenantID string) map[string]interface{} {
	return map[string]interface{}{
		"tenant_id": tenantID,
	}
}

// FilterFromIDs creates a filter map from tenant ID and agent ID
func FilterFromIDs(tenantID, id string) map[string]interface{} {
	return map[string]interface{}{
		"tenant_id": tenantID,
		"id":        id,
	}
}
