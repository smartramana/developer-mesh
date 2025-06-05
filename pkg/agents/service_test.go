package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a mock implementation of Repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateConfig(ctx context.Context, config *AgentConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockRepository) GetConfig(ctx context.Context, agentID string) (*AgentConfig, error) {
	args := m.Called(ctx, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AgentConfig), args.Error(1)
}

func (m *MockRepository) GetConfigByID(ctx context.Context, id uuid.UUID) (*AgentConfig, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AgentConfig), args.Error(1)
}

func (m *MockRepository) GetConfigHistory(ctx context.Context, agentID string, limit int) ([]*AgentConfig, error) {
	args := m.Called(ctx, agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*AgentConfig), args.Error(1)
}

func (m *MockRepository) UpdateConfig(ctx context.Context, agentID string, update *ConfigUpdateRequest) (*AgentConfig, error) {
	args := m.Called(ctx, agentID, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AgentConfig), args.Error(1)
}

func (m *MockRepository) ListConfigs(ctx context.Context, filter ConfigFilter) ([]*AgentConfig, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*AgentConfig), args.Error(1)
}

func (m *MockRepository) DeactivateConfig(ctx context.Context, agentID string) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *MockRepository) DeleteConfig(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockEventPublisher is a mock implementation of EventPublisher
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) PublishConfigUpdated(ctx context.Context, event ConfigUpdatedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishConfigDeleted(ctx context.Context, event ConfigDeletedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func createTestConfig(agentID string) *AgentConfig {
	return &AgentConfig{
		ID:                uuid.New(),
		AgentID:           agentID,
		Version:           1,
		EmbeddingStrategy: StrategyBalanced,
		ModelPreferences: []ModelPreference{
			{
				TaskType:       TaskTypeGeneralQA,
				PrimaryModels:  []string{"text-embedding-3-small"},
				FallbackModels: []string{"text-embedding-ada-002"},
			},
		},
		Constraints: AgentConstraints{
			MaxCostPerMonthUSD: 100.0,
			MaxLatencyP99Ms:    500,
			MinAvailabilitySLA: 0.99,
			RateLimits: RateLimitConfig{
				RequestsPerMinute: 100,
			},
		},
		FallbackBehavior: FallbackConfig{
			MaxRetries:      3,
			InitialDelayMs:  100,
			ExponentialBase: 2.0,
		},
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: "test-user",
	}
}

func TestService_CreateConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("successful creation", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockPublisher := new(MockEventPublisher)
		service := NewService(mockRepo, WithEventPublisher(mockPublisher))

		config := createTestConfig("agent-001")
		
		// Mock expectations
		mockRepo.On("GetConfig", ctx, "agent-001").Return(nil, errors.New("not found"))
		mockRepo.On("CreateConfig", ctx, config).Return(nil)
		mockPublisher.On("PublishConfigUpdated", mock.Anything, mock.AnythingOfType("ConfigUpdatedEvent")).Return(nil)

		err := service.CreateConfig(ctx, config)
		assert.NoError(t, err)

		// Verify cache was updated
		cached, err := service.GetConfig(ctx, "agent-001")
		assert.NoError(t, err)
		assert.Equal(t, config, cached)

		mockRepo.AssertExpectations(t)
		// Publisher is called async, so we don't check immediately
		time.Sleep(100 * time.Millisecond)
		mockPublisher.AssertExpectations(t)
	})

	t.Run("agent already has config", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		existing := createTestConfig("agent-001")
		config := createTestConfig("agent-001")

		mockRepo.On("GetConfig", ctx, "agent-001").Return(existing, nil)

		err := service.CreateConfig(ctx, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already has an active configuration")

		mockRepo.AssertExpectations(t)
	})

	t.Run("validation failure", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		config := createTestConfig("")  // Empty agent ID

		err := service.CreateConfig(ctx, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})
}

func TestService_GetConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("from cache", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		config := createTestConfig("agent-001")
		service.cache.Store("agent-001", config)

		// Repository should not be called
		result, err := service.GetConfig(ctx, "agent-001")
		assert.NoError(t, err)
		assert.Equal(t, config, result)

		mockRepo.AssertNotCalled(t, "GetConfig")
	})

	t.Run("from repository", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		config := createTestConfig("agent-001")
		mockRepo.On("GetConfig", ctx, "agent-001").Return(config, nil)

		result, err := service.GetConfig(ctx, "agent-001")
		assert.NoError(t, err)
		assert.Equal(t, config, result)

		// Check cache was updated
		cached, _ := service.cache.Load("agent-001")
		assert.Equal(t, config, cached)

		mockRepo.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		mockRepo.On("GetConfig", ctx, "agent-001").Return(nil, errors.New("not found"))

		_, err := service.GetConfig(ctx, "agent-001")
		assert.Error(t, err)

		mockRepo.AssertExpectations(t)
	})
}

func TestService_UpdateConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("successful update", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockPublisher := new(MockEventPublisher)
		service := NewService(mockRepo, WithEventPublisher(mockPublisher))

		newStrategy := StrategyQuality
		update := &ConfigUpdateRequest{
			EmbeddingStrategy: &newStrategy,
			UpdatedBy:         "updater",
		}

		updated := createTestConfig("agent-001")
		updated.Version = 2
		updated.EmbeddingStrategy = StrategyQuality

		mockRepo.On("UpdateConfig", ctx, "agent-001", update).Return(updated, nil)
		mockPublisher.On("PublishConfigUpdated", mock.Anything, mock.AnythingOfType("ConfigUpdatedEvent")).Return(nil)

		result, err := service.UpdateConfig(ctx, "agent-001", update)
		assert.NoError(t, err)
		assert.Equal(t, updated, result)

		// Check cache was updated
		cached, _ := service.GetConfig(ctx, "agent-001")
		assert.Equal(t, updated, cached)

		mockRepo.AssertExpectations(t)
		time.Sleep(100 * time.Millisecond)
		mockPublisher.AssertExpectations(t)
	})
}

func TestService_GetModelsForAgent(t *testing.T) {
	ctx := context.Background()

	t.Run("found models for task", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		config := createTestConfig("agent-001")
		config.ModelPreferences = []ModelPreference{
			{
				TaskType:       TaskTypeCodeAnalysis,
				PrimaryModels:  []string{"voyage-code-2"},
				FallbackModels: []string{"text-embedding-3-large"},
			},
			{
				TaskType:       TaskTypeGeneralQA,
				PrimaryModels:  []string{"text-embedding-3-small"},
				FallbackModels: []string{"text-embedding-ada-002"},
			},
		}

		service.cache.Store("agent-001", config)

		primary, fallback, err := service.GetModelsForAgent(ctx, "agent-001", TaskTypeCodeAnalysis)
		assert.NoError(t, err)
		assert.Equal(t, []string{"voyage-code-2"}, primary)
		assert.Equal(t, []string{"text-embedding-3-large"}, fallback)
	})

	t.Run("task type not configured", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		config := createTestConfig("agent-001")
		service.cache.Store("agent-001", config)

		_, _, err := service.GetModelsForAgent(ctx, "agent-001", TaskTypeCodeAnalysis)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no models configured")
	})

	t.Run("fallback to general QA", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		config := createTestConfig("agent-001")
		service.cache.Store("agent-001", config)

		primary, fallback, err := service.GetModelsForAgent(ctx, "agent-001", TaskTypeMultilingual)
		assert.NoError(t, err)
		assert.Equal(t, []string{"text-embedding-3-small"}, primary)
		assert.Equal(t, []string{"text-embedding-ada-002"}, fallback)
	})
}

func TestService_ValidateModels(t *testing.T) {
	ctx := context.Background()

	t.Run("all models valid", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		config := createTestConfig("agent-001")
		service.cache.Store("agent-001", config)

		err := service.ValidateModels(ctx, "agent-001", []string{"text-embedding-3-small", "text-embedding-ada-002"})
		assert.NoError(t, err)
	})

	t.Run("invalid model", func(t *testing.T) {
		mockRepo := new(MockRepository)
		service := NewService(mockRepo)

		config := createTestConfig("agent-001")
		service.cache.Store("agent-001", config)

		err := service.ValidateModels(ctx, "agent-001", []string{"text-embedding-3-small", "invalid-model"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid-model is not configured")
	})
}

func TestAgentConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := createTestConfig("agent-001")
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("missing agent ID", func(t *testing.T) {
		config := createTestConfig("")
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "agent_id is required")
	})

	t.Run("no model preferences", func(t *testing.T) {
		config := createTestConfig("agent-001")
		config.ModelPreferences = []ModelPreference{}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one model preference")
	})

	t.Run("duplicate task type", func(t *testing.T) {
		config := createTestConfig("agent-001")
		config.ModelPreferences = []ModelPreference{
			{
				TaskType:      TaskTypeGeneralQA,
				PrimaryModels: []string{"model1"},
			},
			{
				TaskType:      TaskTypeGeneralQA,
				PrimaryModels: []string{"model2"},
			},
		}
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate task type")
	})

	t.Run("invalid constraints", func(t *testing.T) {
		config := createTestConfig("agent-001")
		config.Constraints.MaxCostPerMonthUSD = -10
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be non-negative")
	})
}

func TestAgentConfig_Clone(t *testing.T) {
	original := createTestConfig("agent-001")
	original.Metadata = map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	clone := original.Clone()

	// Verify deep copy
	assert.Equal(t, original.AgentID, clone.AgentID)
	assert.Equal(t, original.ModelPreferences, clone.ModelPreferences)
	
	// Modify clone
	clone.ModelPreferences[0].PrimaryModels[0] = "modified"
	clone.Metadata["key1"] = "modified"

	// Original should be unchanged
	assert.NotEqual(t, original.ModelPreferences[0].PrimaryModels[0], clone.ModelPreferences[0].PrimaryModels[0])
	assert.NotEqual(t, original.Metadata["key1"], clone.Metadata["key1"])
}