package embedding

import (
	"context"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/agents"
	"github.com/S-Corkum/devops-mcp/pkg/embedding/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSmartRouter(t *testing.T) {
	config := DefaultRouterConfig()
	providers := map[string]providers.Provider{
		"openai": providers.NewMockProvider("openai"),
		"bedrock": providers.NewMockProvider("bedrock"),
	}

	router := NewSmartRouter(config, providers)

	assert.NotNil(t, router)
	assert.Equal(t, config, router.config)
	assert.Len(t, router.providers, 2)
	assert.Len(t, router.circuitBreakers, 2)
	assert.NotNil(t, router.loadBalancer)
	assert.NotNil(t, router.costOptimizer)
	assert.NotNil(t, router.qualityTracker)
}

func TestSmartRouterSelectProvider(t *testing.T) {
	config := DefaultRouterConfig()
	
	// Create mock providers
	openaiProvider := providers.NewMockProvider("openai")
	bedrockProvider := providers.NewMockProvider("bedrock")
	
	providerMap := map[string]providers.Provider{
		"openai":  openaiProvider,
		"bedrock": bedrockProvider,
	}

	router := NewSmartRouter(config, providerMap)

	t.Run("successful provider selection", func(t *testing.T) {
		agentConfig := &agents.AgentConfig{
			AgentID:           "test-agent",
			EmbeddingStrategy: agents.StrategyBalanced,
			ModelPreferences: []agents.ModelPreference{
				{
					TaskType:      agents.TaskTypeGeneralQA,
					PrimaryModels: []string{"mock-model-small", "mock-model-large"},
					FallbackModels: []string{"mock-model-titan"},
				},
			},
		}

		req := &RoutingRequest{
			AgentConfig: agentConfig,
			TaskType:    agents.TaskTypeGeneralQA,
			RequestID:   "test-123",
		}

		decision, err := router.SelectProvider(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, decision)
		assert.True(t, len(decision.Candidates) > 0)
		assert.Equal(t, "balanced", decision.Strategy)
		
		// Check that candidates are properly scored
		for i := 0; i < len(decision.Candidates)-1; i++ {
			assert.GreaterOrEqual(t, decision.Candidates[i].Score, decision.Candidates[i+1].Score)
		}
	})

	t.Run("no models configured for task type", func(t *testing.T) {
		agentConfig := &agents.AgentConfig{
			AgentID:           "test-agent",
			EmbeddingStrategy: agents.StrategyQuality,
			ModelPreferences:  []agents.ModelPreference{},
		}

		req := &RoutingRequest{
			AgentConfig: agentConfig,
			TaskType:    agents.TaskTypeCodeAnalysis,
			RequestID:   "test-456",
		}

		_, err := router.SelectProvider(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no models configured")
	})

	t.Run("circuit breaker filtering", func(t *testing.T) {
		agentConfig := &agents.AgentConfig{
			AgentID:           "test-agent",
			EmbeddingStrategy: agents.StrategySpeed,
			ModelPreferences: []agents.ModelPreference{
				{
					TaskType:      agents.TaskTypeGeneralQA,
					PrimaryModels: []string{"mock-model-small"},
				},
			},
		}

		// Open all circuit breakers
		for _, cb := range router.circuitBreakers {
			// Force circuit breaker to open state
			for i := 0; i < config.CircuitBreakerConfig.FailureThreshold; i++ {
				cb.RecordFailure()
			}
		}

		req := &RoutingRequest{
			AgentConfig: agentConfig,
			TaskType:    agents.TaskTypeGeneralQA,
			RequestID:   "test-789",
		}

		_, err := router.SelectProvider(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no available providers")
	})
}

func TestSmartRouterScoreCandidates(t *testing.T) {
	config := DefaultRouterConfig()
	
	openaiProvider := providers.NewMockProvider("openai")
	bedrockProvider := providers.NewMockProvider("bedrock")
	
	providerMap := map[string]providers.Provider{
		"openai":  openaiProvider,
		"bedrock": bedrockProvider,
	}

	router := NewSmartRouter(config, providerMap)

	t.Run("score multiple candidates", func(t *testing.T) {
		agentConfig := &agents.AgentConfig{
			AgentID:           "test-agent",
			EmbeddingStrategy: agents.StrategyBalanced,
			ModelPreferences: []agents.ModelPreference{
				{
					TaskType:      agents.TaskTypeGeneralQA,
					PrimaryModels: []string{"mock-model-small", "mock-model-large", "mock-model-titan"},
				},
			},
		}

		req := &RoutingRequest{
			AgentConfig: agentConfig,
			TaskType:    agents.TaskTypeGeneralQA,
			RequestID:   "test-123",
		}

		models := []string{"mock-model-small", "mock-model-large", "mock-model-titan"}
		candidates := router.scoreCandidates(req, models)

		assert.Len(t, candidates, 3)
		
		// Check that all candidates have scores and reasons
		for _, candidate := range candidates {
			assert.NotEmpty(t, candidate.Provider)
			assert.NotEmpty(t, candidate.Model)
			assert.True(t, candidate.Score >= 0 && candidate.Score <= 1, "Score %f should be between 0 and 1", candidate.Score)
			assert.NotEmpty(t, candidate.Reasons)
		}
	})
}

func TestSmartRouterScoreProviderModel(t *testing.T) {
	config := DefaultRouterConfig()
	
	providerMap := map[string]providers.Provider{
		"openai": providers.NewMockProvider("openai"),
	}

	router := NewSmartRouter(config, providerMap)

	t.Run("balanced strategy scoring", func(t *testing.T) {
		agentConfig := &agents.AgentConfig{
			EmbeddingStrategy: agents.StrategyBalanced,
			Constraints: agents.AgentConstraints{
				MaxLatencyP99Ms: 1000,
			},
		}

		req := &RoutingRequest{
			AgentConfig: agentConfig,
			TaskType:    agents.TaskTypeGeneralQA,
		}

		model := providers.ModelInfo{
			Name:            "test-model",
			Dimensions:      1536,
			CostPer1MTokens: 0.02,
			IsActive:        true,
		}

		score, reasons := router.scoreProviderModel(req, "openai", model)
		
		assert.True(t, score > 0 && score <= 1)
		assert.NotEmpty(t, reasons)
	})

	t.Run("quality strategy scoring", func(t *testing.T) {
		agentConfig := &agents.AgentConfig{
			EmbeddingStrategy: agents.StrategyQuality,
		}

		req := &RoutingRequest{
			AgentConfig: agentConfig,
			TaskType:    agents.TaskTypeResearch,
		}

		model := providers.ModelInfo{
			Name:               "high-quality-model",
			Dimensions:         3072,
			CostPer1MTokens:    0.13,
			IsActive:           true,
			SupportedTaskTypes: []string{"research", "general_qa"},
		}

		score, reasons := router.scoreProviderModel(req, "openai", model)
		
		// Quality strategy should favor higher dimension models
		assert.True(t, score > 0.5)
		assert.Contains(t, reasons, "quality priority")
	})

	t.Run("cost strategy scoring", func(t *testing.T) {
		agentConfig := &agents.AgentConfig{
			EmbeddingStrategy: agents.StrategyCost,
		}

		req := &RoutingRequest{
			AgentConfig: agentConfig,
			TaskType:    agents.TaskTypeGeneralQA,
		}

		model := providers.ModelInfo{
			Name:            "cheap-model",
			Dimensions:      768,
			CostPer1MTokens: 0.01,
			IsActive:        true,
		}

		score, reasons := router.scoreProviderModel(req, "openai", model)
		
		// Cost strategy should favor cheaper models
		assert.True(t, score > 0.7)
		assert.Contains(t, reasons, "cost optimized")
	})

	t.Run("speed strategy scoring", func(t *testing.T) {
		agentConfig := &agents.AgentConfig{
			EmbeddingStrategy: agents.StrategySpeed,
		}

		req := &RoutingRequest{
			AgentConfig: agentConfig,
			TaskType:    agents.TaskTypeGeneralQA,
		}

		model := providers.ModelInfo{
			Name:            "fast-model",
			Dimensions:      1536,
			CostPer1MTokens: 0.02,
			IsActive:        true,
		}

		score, reasons := router.scoreProviderModel(req, "openai", model)
		
		assert.True(t, score > 0)
		assert.Contains(t, reasons, "speed priority")
	})

	t.Run("inactive model gets zero score", func(t *testing.T) {
		agentConfig := &agents.AgentConfig{
			EmbeddingStrategy: agents.StrategyBalanced,
		}

		req := &RoutingRequest{
			AgentConfig: agentConfig,
			TaskType:    agents.TaskTypeGeneralQA,
		}

		model := providers.ModelInfo{
			Name:            "inactive-model",
			Dimensions:      1536,
			CostPer1MTokens: 0.02,
			IsActive:        false,
		}

		score, reasons := router.scoreProviderModel(req, "openai", model)
		
		assert.Equal(t, float64(0), score)
		assert.Contains(t, reasons, "inactive model")
	})

	t.Run("deprecated model gets penalty", func(t *testing.T) {
		agentConfig := &agents.AgentConfig{
			EmbeddingStrategy: agents.StrategyBalanced,
		}

		req := &RoutingRequest{
			AgentConfig: agentConfig,
			TaskType:    agents.TaskTypeGeneralQA,
		}

		deprecatedTime := time.Now().Add(-24 * time.Hour)
		model := providers.ModelInfo{
			Name:            "deprecated-model",
			Dimensions:      1536,
			CostPer1MTokens: 0.02,
			IsActive:        true,
			DeprecatedAt:    &deprecatedTime,
		}

		score, reasons := router.scoreProviderModel(req, "openai", model)
		
		// Deprecated models should have reduced score
		assert.True(t, score < 0.5)
		assert.Contains(t, reasons, "deprecated model")
	})
}

func TestSmartRouterCircuitBreakerIntegration(t *testing.T) {
	config := DefaultRouterConfig()
	config.CircuitBreakerConfig.FailureThreshold = 2
	config.CircuitBreakerConfig.Timeout = 100 * time.Millisecond
	
	providerMap := map[string]providers.Provider{
		"openai":  providers.NewMockProvider("openai"),
		"bedrock": providers.NewMockProvider("bedrock"),
	}

	router := NewSmartRouter(config, providerMap)

	agentConfig := &agents.AgentConfig{
		AgentID:           "test-agent",
		EmbeddingStrategy: agents.StrategyBalanced,
		ModelPreferences: []agents.ModelPreference{
			{
				TaskType:      agents.TaskTypeGeneralQA,
				PrimaryModels: []string{"mock-model-small"},
			},
		},
	}

	req := &RoutingRequest{
		AgentConfig: agentConfig,
		TaskType:    agents.TaskTypeGeneralQA,
		RequestID:   "test-123",
	}

	// Initially should work
	decision, err := router.SelectProvider(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, len(decision.Candidates) > 0)

	// Record failures for openai
	openaiCB := router.circuitBreakers["openai"]
	openaiCB.RecordFailure()
	openaiCB.RecordFailure()

	// OpenAI should now be filtered out
	decision, err = router.SelectProvider(context.Background(), req)
	require.NoError(t, err)
	
	// Should only have bedrock candidates
	for _, candidate := range decision.Candidates {
		assert.NotEqual(t, "openai", candidate.Provider)
	}

	// Wait for circuit breaker timeout
	time.Sleep(config.CircuitBreakerConfig.Timeout + 10*time.Millisecond)

	// OpenAI should be available again (half-open)
	decision, err = router.SelectProvider(context.Background(), req)
	require.NoError(t, err)
	
	hasOpenAI := false
	for _, candidate := range decision.Candidates {
		if candidate.Provider == "openai" {
			hasOpenAI = true
			break
		}
	}
	assert.True(t, hasOpenAI, "OpenAI should be available after timeout")
}

// Helper function tests
func TestLoadBalancer(t *testing.T) {
	config := LoadBalancerConfig{
		Strategy: "weighted_round_robin",
	}

	lb := NewLoadBalancer(config)
	assert.NotNil(t, lb)
}

func TestCostOptimizer(t *testing.T) {
	config := CostOptimizerConfig{
		MaxCostPerRequest: 0.01,
	}

	co := NewCostOptimizer(config)
	assert.NotNil(t, co)
}

func TestQualityTracker(t *testing.T) {
	config := QualityConfig{
		MinQualityScore: 0.8,
	}

	qt := NewQualityTracker(config)
	assert.NotNil(t, qt)
}

// Benchmark tests
func BenchmarkSmartRouterSelectProvider(b *testing.B) {
	config := DefaultRouterConfig()
	
	providerMap := map[string]providers.Provider{
		"openai":  providers.NewMockProvider("openai"),
		"bedrock": providers.NewMockProvider("bedrock"),
		"voyage":  providers.NewMockProvider("voyage"),
	}

	router := NewSmartRouter(config, providerMap)

	agentConfig := &agents.AgentConfig{
		AgentID:           "bench-agent",
		EmbeddingStrategy: agents.StrategyBalanced,
		ModelPreferences: []agents.ModelPreference{
			{
				TaskType:       agents.TaskTypeGeneralQA,
				PrimaryModels:  []string{"mock-model-small", "mock-model-large"},
				FallbackModels: []string{"mock-model-titan"},
			},
		},
	}

	req := &RoutingRequest{
		AgentConfig: agentConfig,
		TaskType:    agents.TaskTypeGeneralQA,
		RequestID:   "bench-123",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = router.SelectProvider(ctx, req)
	}
}

func BenchmarkSmartRouterScoreCandidates(b *testing.B) {
	config := DefaultRouterConfig()
	
	providerMap := map[string]providers.Provider{
		"openai":  providers.NewMockProvider("openai"),
		"bedrock": providers.NewMockProvider("bedrock"),
	}

	router := NewSmartRouter(config, providerMap)

	agentConfig := &agents.AgentConfig{
		EmbeddingStrategy: agents.StrategyBalanced,
	}

	req := &RoutingRequest{
		AgentConfig: agentConfig,
		TaskType:    agents.TaskTypeGeneralQA,
	}

	models := []string{"mock-model-small", "mock-model-large", "mock-model-titan", "mock-model-code"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.scoreCandidates(req, models)
	}
}