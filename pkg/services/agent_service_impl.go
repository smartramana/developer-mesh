package services

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository/agent"
	"github.com/google/uuid"
)

// agentService implements AgentService
type agentService struct {
	BaseService
	repo agent.Repository
}

// NewAgentService creates a new agent service
func NewAgentService(config ServiceConfig, repo agent.Repository) AgentService {
	return &agentService{
		BaseService: NewBaseService(config),
		repo:        repo,
	}
}

// GetAgent retrieves an agent by ID
func (s *agentService) GetAgent(ctx context.Context, agentID string) (*models.Agent, error) {
	ctx, span := s.config.Tracer(ctx, "AgentService.GetAgent")
	defer span.End()

	// Parse agent ID
	id, err := uuid.Parse(agentID)
	if err != nil {
		return nil, ErrInvalidID
	}

	// Get from repository
	agent, err := s.repo.Get(ctx, id.String())
	if err != nil {
		s.config.Logger.Error("Failed to get agent", map[string]interface{}{
			"agent_id": agentID,
			"error":    err.Error(),
		})
		return nil, err
	}

	return agent, nil
}

// GetAvailableAgents retrieves all available agents
func (s *agentService) GetAvailableAgents(ctx context.Context) ([]*models.Agent, error) {
	ctx, span := s.config.Tracer(ctx, "AgentService.GetAvailableAgents")
	defer span.End()

	// Get all active agents
	agents, err := s.repo.GetByStatus(ctx, models.AgentStatusActive)
	if err != nil {
		s.config.Logger.Error("Failed to get available agents", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// Filter by availability
	available := make([]*models.Agent, 0, len(agents))
	for _, agent := range agents {
		if agent.Status == string(models.AgentStatusActive) {
			available = append(available, agent)
		}
	}

	return available, nil
}

// GetAgentCapabilities retrieves agent capabilities
func (s *agentService) GetAgentCapabilities(ctx context.Context, agentID string) ([]string, error) {
	ctx, span := s.config.Tracer(ctx, "AgentService.GetAgentCapabilities")
	defer span.End()

	// Get agent
	agent, err := s.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Return the agent's capabilities directly
	return agent.Capabilities, nil
}

// UpdateAgentStatus updates agent status
func (s *agentService) UpdateAgentStatus(ctx context.Context, agentID string, status string) error {
	ctx, span := s.config.Tracer(ctx, "AgentService.UpdateAgentStatus")
	defer span.End()

	// Parse agent ID
	id, err := uuid.Parse(agentID)
	if err != nil {
		return ErrInvalidID
	}

	// Get agent
	agent, err := s.repo.Get(ctx, id.String())
	if err != nil {
		return err
	}

	// Update status
	agent.Status = status
	agent.UpdatedAt = time.Now()

	// Save to repository
	if err := s.repo.Update(ctx, agent); err != nil {
		s.config.Logger.Error("Failed to update agent status", map[string]interface{}{
			"agent_id": agentID,
			"status":   status,
			"error":    err.Error(),
		})
		return err
	}

	return nil
}

// GetAgentWorkload retrieves agent workload information
func (s *agentService) GetAgentWorkload(ctx context.Context, agentID string) (*models.AgentWorkload, error) {
	ctx, span := s.config.Tracer(ctx, "AgentService.GetAgentWorkload")
	defer span.End()

	// Parse agent ID
	id, err := uuid.Parse(agentID)
	if err != nil {
		return nil, ErrInvalidID
	}

	// Get workload from repository
	workload, err := s.repo.GetWorkload(ctx, id)
	if err != nil {
		s.config.Logger.Error("Failed to get agent workload", map[string]interface{}{
			"agent_id": agentID,
			"error":    err.Error(),
		})
		return nil, err
	}

	return workload, nil
}
