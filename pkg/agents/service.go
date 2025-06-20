package agents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// Service provides agent configuration management
type Service struct {
	repo      Repository
	validator *validator.Validate
	cache     *sync.Map // Simple in-memory cache for active configs
	publisher EventPublisher
	mu        sync.RWMutex
}

// EventPublisher publishes configuration change events
type EventPublisher interface {
	PublishConfigUpdated(ctx context.Context, event ConfigUpdatedEvent) error
	PublishConfigDeleted(ctx context.Context, event ConfigDeletedEvent) error
}

// ConfigUpdatedEvent represents a configuration update event
type ConfigUpdatedEvent struct {
	AgentID   string    `json:"agent_id"`
	ConfigID  uuid.UUID `json:"config_id"`
	Version   int       `json:"version"`
	UpdatedBy string    `json:"updated_by"`
	Timestamp time.Time `json:"timestamp"`
}

// ConfigDeletedEvent represents a configuration deletion event
type ConfigDeletedEvent struct {
	AgentID   string    `json:"agent_id"`
	ConfigID  uuid.UUID `json:"config_id"`
	DeletedBy string    `json:"deleted_by"`
	Timestamp time.Time `json:"timestamp"`
}

// ServiceOption configures the service
type ServiceOption func(*Service)

// WithEventPublisher sets the event publisher
func WithEventPublisher(publisher EventPublisher) ServiceOption {
	return func(s *Service) {
		s.publisher = publisher
	}
}

// NewService creates a new agent configuration service
func NewService(repo Repository, opts ...ServiceOption) *Service {
	s := &Service{
		repo:      repo,
		validator: validator.New(),
		cache:     &sync.Map{},
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// CreateConfig creates a new agent configuration
func (s *Service) CreateConfig(ctx context.Context, config *AgentConfig) error {
	// Validate configuration
	if err := s.validator.Struct(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Check if agent already has a configuration
	existing, err := s.repo.GetConfig(ctx, config.AgentID)
	if err == nil && existing != nil {
		return fmt.Errorf("agent %s already has an active configuration", config.AgentID)
	}

	// Create configuration
	if err := s.repo.CreateConfig(ctx, config); err != nil {
		return fmt.Errorf("failed to create configuration: %w", err)
	}

	// Update cache
	s.cache.Store(config.AgentID, config)

	// Publish event
	if s.publisher != nil {
		event := ConfigUpdatedEvent{
			AgentID:   config.AgentID,
			ConfigID:  config.ID,
			Version:   config.Version,
			UpdatedBy: config.CreatedBy,
			Timestamp: time.Now(),
		}
		go func() {
			// In production, this error should be logged
			_ = s.publisher.PublishConfigUpdated(context.Background(), event)
		}()
	}

	return nil
}

// GetConfig retrieves the active configuration for an agent
func (s *Service) GetConfig(ctx context.Context, agentID string) (*AgentConfig, error) {
	// Check cache first
	if cached, ok := s.cache.Load(agentID); ok {
		return cached.(*AgentConfig), nil
	}

	// Get from repository
	config, err := s.repo.GetConfig(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.cache.Store(agentID, config)

	return config, nil
}

// GetConfigByID retrieves a specific configuration by ID
func (s *Service) GetConfigByID(ctx context.Context, id uuid.UUID) (*AgentConfig, error) {
	return s.repo.GetConfigByID(ctx, id)
}

// GetConfigHistory retrieves configuration history for an agent
func (s *Service) GetConfigHistory(ctx context.Context, agentID string, limit int) ([]*AgentConfig, error) {
	return s.repo.GetConfigHistory(ctx, agentID, limit)
}

// UpdateConfig updates an agent's configuration
func (s *Service) UpdateConfig(ctx context.Context, agentID string, update *ConfigUpdateRequest) (*AgentConfig, error) {
	// Validate update request
	if err := s.validator.Struct(update); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Update configuration
	newConfig, err := s.repo.UpdateConfig(ctx, agentID, update)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.cache.Store(agentID, newConfig)

	// Publish event
	if s.publisher != nil {
		event := ConfigUpdatedEvent{
			AgentID:   newConfig.AgentID,
			ConfigID:  newConfig.ID,
			Version:   newConfig.Version,
			UpdatedBy: update.UpdatedBy,
			Timestamp: time.Now(),
		}
		go func() {
			// In production, this error should be logged
			_ = s.publisher.PublishConfigUpdated(context.Background(), event)
		}()
	}

	return newConfig, nil
}

// ListConfigs lists configurations based on filter
func (s *Service) ListConfigs(ctx context.Context, filter ConfigFilter) ([]*AgentConfig, error) {
	return s.repo.ListConfigs(ctx, filter)
}

// DeactivateConfig deactivates an agent's configuration
func (s *Service) DeactivateConfig(ctx context.Context, agentID string, deactivatedBy string) error {
	// Get current config for event
	config, err := s.repo.GetConfig(ctx, agentID)
	if err != nil {
		return err
	}

	// Deactivate
	if err := s.repo.DeactivateConfig(ctx, agentID); err != nil {
		return err
	}

	// Remove from cache
	s.cache.Delete(agentID)

	// Publish event
	if s.publisher != nil {
		event := ConfigDeletedEvent{
			AgentID:   agentID,
			ConfigID:  config.ID,
			DeletedBy: deactivatedBy,
			Timestamp: time.Now(),
		}
		go func() {
			// In production, this error should be logged
			_ = s.publisher.PublishConfigDeleted(context.Background(), event)
		}()
	}

	return nil
}

// DeleteConfig deletes a configuration
func (s *Service) DeleteConfig(ctx context.Context, id uuid.UUID, deletedBy string) error {
	// Get config for cache invalidation
	config, err := s.repo.GetConfigByID(ctx, id)
	if err != nil {
		return err
	}

	// Delete
	if err := s.repo.DeleteConfig(ctx, id); err != nil {
		return err
	}

	// Remove from cache if it was the active config
	if cached, ok := s.cache.Load(config.AgentID); ok {
		if cachedConfig := cached.(*AgentConfig); cachedConfig.ID == id {
			s.cache.Delete(config.AgentID)
		}
	}

	// Publish event
	if s.publisher != nil {
		event := ConfigDeletedEvent{
			AgentID:   config.AgentID,
			ConfigID:  id,
			DeletedBy: deletedBy,
			Timestamp: time.Now(),
		}
		go func() {
			// In production, this error should be logged
			_ = s.publisher.PublishConfigDeleted(context.Background(), event)
		}()
	}

	return nil
}

// GetModelsForAgent returns the preferred models for an agent's task
func (s *Service) GetModelsForAgent(ctx context.Context, agentID string, taskType TaskType) (primary []string, fallback []string, err error) {
	config, err := s.GetConfig(ctx, agentID)
	if err != nil {
		return nil, nil, err
	}

	primary, fallback = config.GetModelsForTask(taskType)
	if len(primary) == 0 {
		return nil, nil, fmt.Errorf("no models configured for task type: %s", taskType)
	}

	return primary, fallback, nil
}

// ValidateModels checks if the specified models are valid for the agent
func (s *Service) ValidateModels(ctx context.Context, agentID string, models []string) error {
	config, err := s.GetConfig(ctx, agentID)
	if err != nil {
		return err
	}

	// Build a set of all allowed models
	allowedModels := make(map[string]bool)
	for _, pref := range config.ModelPreferences {
		for _, model := range pref.PrimaryModels {
			allowedModels[model] = true
		}
		for _, model := range pref.FallbackModels {
			allowedModels[model] = true
		}
	}

	// Check each model
	for _, model := range models {
		if !allowedModels[model] {
			return fmt.Errorf("model %s is not configured for agent %s", model, agentID)
		}
	}

	return nil
}

// RefreshCache refreshes the configuration cache
func (s *Service) RefreshCache(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear current cache
	s.cache = &sync.Map{}

	// Get all active configurations
	isActive := true
	configs, err := s.repo.ListConfigs(ctx, ConfigFilter{
		IsActive: &isActive,
		Limit:    1000, // Reasonable limit
	})
	if err != nil {
		return fmt.Errorf("failed to list configurations: %w", err)
	}

	// Populate cache
	for _, config := range configs {
		s.cache.Store(config.AgentID, config)
	}

	return nil
}

// GetCacheStats returns cache statistics
func (s *Service) GetCacheStats() map[string]interface{} {
	count := 0
	s.cache.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	return map[string]interface{}{
		"cached_configs": count,
	}
}
