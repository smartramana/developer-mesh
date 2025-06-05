package embedding

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/agents"
	"github.com/S-Corkum/devops-mcp/pkg/embedding/providers"
	"github.com/google/uuid"
)

// AgentService defines the interface for agent configuration management
type AgentService interface {
	GetConfig(ctx context.Context, agentID string) (*agents.AgentConfig, error)
	GetModelsForAgent(ctx context.Context, agentID string, taskType agents.TaskType) (primary []string, fallback []string, err error)
	CreateConfig(ctx context.Context, config *agents.AgentConfig) error
	UpdateConfig(ctx context.Context, agentID string, update *agents.ConfigUpdateRequest) (*agents.AgentConfig, error)
}

// ServiceV2 is the enhanced embedding service with multi-agent support
type ServiceV2 struct {
	providers       map[string]providers.Provider
	agentService    AgentService
	repository      *Repository
	metricsRepo     MetricsRepository
	router          *SmartRouter
	dimensionAdapter *DimensionAdapter
	cache           EmbeddingCache
	mu              sync.RWMutex
}

// ServiceV2Config contains configuration for the service
type ServiceV2Config struct {
	Providers       map[string]providers.Provider
	AgentService    AgentService
	Repository      *Repository
	MetricsRepo     MetricsRepository
	Cache           EmbeddingCache
	RouterConfig    *RouterConfig
}

// EmbeddingCache defines the interface for caching embeddings
type EmbeddingCache interface {
	Get(ctx context.Context, key string) (*CachedEmbedding, error)
	Set(ctx context.Context, key string, embedding *CachedEmbedding, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// CachedEmbedding represents a cached embedding
type CachedEmbedding struct {
	Embedding    []float32              `json:"embedding"`
	Model        string                 `json:"model"`
	Provider     string                 `json:"provider"`
	Dimensions   int                    `json:"dimensions"`
	Metadata     map[string]interface{} `json:"metadata"`
	CachedAt     time.Time              `json:"cached_at"`
}

// MetricsRepository stores embedding metrics
type MetricsRepository interface {
	RecordMetric(ctx context.Context, metric *EmbeddingMetric) error
	GetMetrics(ctx context.Context, filter MetricsFilter) ([]*EmbeddingMetric, error)
	GetAgentCosts(ctx context.Context, agentID string, period time.Duration) (*CostSummary, error)
}

// EmbeddingMetric represents a single metric entry
type EmbeddingMetric struct {
	ID                   uuid.UUID              `json:"id" db:"id"`
	AgentID              string                 `json:"agent_id" db:"agent_id"`
	ModelProvider        string                 `json:"model_provider" db:"model_provider"`
	ModelName            string                 `json:"model_name" db:"model_name"`
	ModelDimensions      int                    `json:"model_dimensions" db:"model_dimensions"`
	RequestID            uuid.UUID              `json:"request_id" db:"request_id"`
	TokenCount           int                    `json:"token_count" db:"token_count"`
	TotalLatencyMs       int                    `json:"total_latency_ms" db:"total_latency_ms"`
	ProviderLatencyMs    int                    `json:"provider_latency_ms" db:"provider_latency_ms"`
	NormalizationLatencyMs int                  `json:"normalization_latency_ms" db:"normalization_latency_ms"`
	CostUSD              float64                `json:"cost_usd" db:"cost_usd"`
	Status               string                 `json:"status" db:"status"`
	ErrorMessage         string                 `json:"error_message" db:"error_message"`
	RetryCount           int                    `json:"retry_count" db:"retry_count"`
	FinalProvider        string                 `json:"final_provider" db:"final_provider"`
	TenantID             uuid.UUID              `json:"tenant_id" db:"tenant_id"`
	Timestamp            time.Time              `json:"timestamp" db:"timestamp"`
}

// GenerateEmbeddingRequest represents a request to generate an embedding
type GenerateEmbeddingRequest struct {
	AgentID      string                 `json:"agent_id" validate:"required"`
	Text         string                 `json:"text" validate:"required,max=50000"`
	TaskType     agents.TaskType        `json:"task_type"`
	Metadata     map[string]interface{} `json:"metadata"`
	RequestID    string                 `json:"request_id"`
	TenantID     uuid.UUID              `json:"tenant_id"`
	ContextID    uuid.UUID              `json:"context_id"`
}

// GenerateEmbeddingResponse represents the response from generating an embedding
type GenerateEmbeddingResponse struct {
	EmbeddingID          uuid.UUID              `json:"embedding_id"`
	RequestID            string                 `json:"request_id"`
	ModelUsed            string                 `json:"model_used"`
	Provider             string                 `json:"provider"`
	Dimensions           int                    `json:"dimensions"`
	NormalizedDimensions int                    `json:"normalized_dimensions"`
	CostUSD              float64                `json:"cost_usd"`
	TokensUsed           int                    `json:"tokens_used"`
	GenerationTimeMs     int64                  `json:"generation_time_ms"`
	Cached               bool                   `json:"cached"`
	Metadata             map[string]interface{} `json:"metadata"`
}

// NewServiceV2 creates a new enhanced embedding service
func NewServiceV2(config ServiceV2Config) (*ServiceV2, error) {
	if len(config.Providers) == 0 {
		return nil, fmt.Errorf("at least one provider is required")
	}

	if config.AgentService == nil {
		return nil, fmt.Errorf("agent service is required")
	}

	if config.Repository == nil {
		return nil, fmt.Errorf("repository is required")
	}

	s := &ServiceV2{
		providers:    config.Providers,
		agentService: config.AgentService,
		repository:   config.Repository,
		metricsRepo:  config.MetricsRepo,
		cache:        config.Cache,
	}

	// Initialize router
	if config.RouterConfig == nil {
		config.RouterConfig = DefaultRouterConfig()
	}
	s.router = NewSmartRouter(config.RouterConfig, config.Providers)

	// Initialize dimension adapter
	s.dimensionAdapter = NewDimensionAdapter()

	return s, nil
}

// GenerateEmbedding generates an embedding for the given request
func (s *ServiceV2) GenerateEmbedding(ctx context.Context, req GenerateEmbeddingRequest) (*GenerateEmbeddingResponse, error) {
	start := time.Now()
	requestID := req.RequestID
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Get agent configuration
	agentConfig, err := s.agentService.GetConfig(ctx, req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent config: %w", err)
	}

	// Determine task type
	taskType := req.TaskType
	if taskType == "" {
		taskType = agents.TaskTypeGeneralQA
	}

	// Check cache if enabled
	if s.cache != nil {
		cacheKey := s.generateCacheKey(req)
		if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			return &GenerateEmbeddingResponse{
				EmbeddingID:          uuid.New(),
				RequestID:            requestID,
				ModelUsed:            cached.Model,
				Provider:             cached.Provider,
				Dimensions:           cached.Dimensions,
				NormalizedDimensions: StandardDimension,
				Cached:               true,
				Metadata:             cached.Metadata,
				GenerationTimeMs:     time.Since(start).Milliseconds(),
			}, nil
		}
	}

	// Select model and provider using router
	routingDecision, err := s.router.SelectProvider(ctx, &RoutingRequest{
		AgentConfig: agentConfig,
		TaskType:    taskType,
		RequestID:   requestID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to select provider: %w", err)
	}

	// Generate embedding with selected provider
	var embeddingResp *providers.EmbeddingResponse
	var lastErr error
	retryCount := 0

	for _, candidate := range routingDecision.Candidates {
		provider := s.providers[candidate.Provider]
		if provider == nil {
			continue
		}

		providerStart := time.Now()
		embeddingResp, lastErr = provider.GenerateEmbedding(ctx, providers.GenerateEmbeddingRequest{
			Text:      req.Text,
			Model:     candidate.Model,
			Metadata:  req.Metadata,
			RequestID: requestID,
		})
		providerLatency := time.Since(providerStart)

		if lastErr == nil {
			// Success - record metrics
			s.recordMetric(ctx, &EmbeddingMetric{
				ID:                uuid.New(),
				AgentID:           req.AgentID,
				ModelProvider:     candidate.Provider,
				ModelName:         candidate.Model,
				ModelDimensions:   embeddingResp.Dimensions,
				RequestID:         uuid.MustParse(requestID),
				TokenCount:        embeddingResp.TokensUsed,
				TotalLatencyMs:    int(time.Since(start).Milliseconds()),
				ProviderLatencyMs: int(providerLatency.Milliseconds()),
				CostUSD:           calculateCost(embeddingResp.TokensUsed, candidate.Model),
				Status:            "success",
				RetryCount:        retryCount,
				FinalProvider:     candidate.Provider,
				TenantID:          req.TenantID,
				Timestamp:         time.Now(),
			})
			break
		}

		// Failure - record and try next
		s.recordMetric(ctx, &EmbeddingMetric{
			ID:            uuid.New(),
			AgentID:       req.AgentID,
			ModelProvider: candidate.Provider,
			ModelName:     candidate.Model,
			RequestID:     uuid.MustParse(requestID),
			Status:        "failure",
			ErrorMessage:  lastErr.Error(),
			RetryCount:    retryCount,
			TenantID:      req.TenantID,
			Timestamp:     time.Now(),
		})

		retryCount++
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed: %w", lastErr)
	}

	// Normalize embedding to standard dimension
	normStart := time.Now()
	normalizedEmbedding := s.dimensionAdapter.Normalize(
		embeddingResp.Embedding,
		embeddingResp.Dimensions,
		StandardDimension,
	)
	normLatency := time.Since(normStart)

	// Store embedding
	// Add agent-specific metadata
	metadata := make(map[string]interface{})
	if req.Metadata != nil {
		metadata = req.Metadata
	}
	metadata["agent_id"] = req.AgentID
	metadata["task_type"] = taskType
	metadata["normalized_embedding"] = normalizedEmbedding
	metadata["cost_usd"] = calculateCost(embeddingResp.TokensUsed, embeddingResp.Model)
	metadata["generation_time_ms"] = time.Since(start).Milliseconds()

	insertReq := InsertRequest{
		ContextID:            req.ContextID,
		Content:              req.Text,
		Embedding:            embeddingResp.Embedding,
		ModelName:            embeddingResp.Model,
		TenantID:             req.TenantID,
		Metadata:             json.RawMessage(mustMarshalJSON(metadata)),
		ConfiguredDimensions: &embeddingResp.Dimensions,
	}

	embeddingID, err := s.repository.InsertEmbedding(ctx, insertReq)
	if err != nil {
		return nil, fmt.Errorf("failed to store embedding: %w", err)
	}

	// Update cache
	if s.cache != nil {
		cacheKey := s.generateCacheKey(req)
		_ = s.cache.Set(ctx, cacheKey, &CachedEmbedding{
			Embedding:  embeddingResp.Embedding,
			Model:      embeddingResp.Model,
			Provider:   embeddingResp.ProviderInfo.Provider,
			Dimensions: embeddingResp.Dimensions,
			Metadata:   metadata,
			CachedAt:   time.Now(),
		}, 1*time.Hour) // Cache errors are non-critical
	}

	// Update final metrics with normalization time
	s.recordMetric(ctx, &EmbeddingMetric{
		ID:                     uuid.New(),
		AgentID:                req.AgentID,
		ModelProvider:          embeddingResp.ProviderInfo.Provider,
		ModelName:              embeddingResp.Model,
		NormalizationLatencyMs: int(normLatency.Milliseconds()),
		TenantID:               req.TenantID,
		Timestamp:              time.Now(),
	})

	return &GenerateEmbeddingResponse{
		EmbeddingID:          embeddingID,
		RequestID:            requestID,
		ModelUsed:            embeddingResp.Model,
		Provider:             embeddingResp.ProviderInfo.Provider,
		Dimensions:           embeddingResp.Dimensions,
		NormalizedDimensions: StandardDimension,
		CostUSD:              calculateCost(embeddingResp.TokensUsed, embeddingResp.Model),
		TokensUsed:           embeddingResp.TokensUsed,
		GenerationTimeMs:     time.Since(start).Milliseconds(),
		Cached:               false,
		Metadata:             metadata,
	}, nil
}

// BatchGenerateEmbeddings generates embeddings for multiple texts
func (s *ServiceV2) BatchGenerateEmbeddings(ctx context.Context, reqs []GenerateEmbeddingRequest) ([]*GenerateEmbeddingResponse, error) {
	// For now, process sequentially
	// TODO: Implement proper batching with provider support
	responses := make([]*GenerateEmbeddingResponse, len(reqs))
	
	for i, req := range reqs {
		resp, err := s.GenerateEmbedding(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding %d: %w", i, err)
		}
		responses[i] = resp
	}
	
	return responses, nil
}

// GetProviderHealth returns health status of all providers
func (s *ServiceV2) GetProviderHealth(ctx context.Context) map[string]ProviderHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()

	health := make(map[string]ProviderHealth)
	
	for name, provider := range s.providers {
		status := ProviderHealth{
			Name:   name,
			Status: "healthy",
		}
		
		if err := provider.HealthCheck(ctx); err != nil {
			status.Status = "unhealthy"
			status.Error = err.Error()
		}
		
		// Get circuit breaker status from router
		if cbStatus := s.router.GetCircuitBreakerStatus(name); cbStatus != nil {
			status.CircuitBreakerState = cbStatus.State
			status.FailureCount = cbStatus.FailureCount
		}
		
		health[name] = status
	}
	
	return health
}

// Private helper methods

func (s *ServiceV2) generateCacheKey(req GenerateEmbeddingRequest) string {
	// Create a deterministic cache key
	return fmt.Sprintf("embedding:%s:%s:%s", req.AgentID, req.TaskType, hashString(req.Text))
}

func (s *ServiceV2) recordMetric(ctx context.Context, metric *EmbeddingMetric) {
	if s.metricsRepo != nil {
		go func() {
			// In production, metrics errors should be logged but not fail the operation
			_ = s.metricsRepo.RecordMetric(context.Background(), metric)
		}()
	}
}

func calculateCost(tokens int, model string) float64 {
	// Cost calculation based on model
	costPer1MTokens := map[string]float64{
		"text-embedding-3-small":       0.02,
		"text-embedding-3-large":       0.13,
		"text-embedding-ada-002":       0.10,
		"amazon.titan-embed-text-v2:0": 0.02,
		"voyage-code-2":                0.10,
	}
	
	if cost, ok := costPer1MTokens[model]; ok {
		return float64(tokens) * cost / 1_000_000
	}
	
	// Default cost
	return float64(tokens) * 0.05 / 1_000_000
}

func mustMarshalJSON(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}

func hashString(s string) string {
	// Simple hash for cache key
	// In production, use a proper hash function
	return fmt.Sprintf("%x", s[:min(len(s), 32)])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Types for health and metrics

type ProviderHealth struct {
	Name                string `json:"name"`
	Status              string `json:"status"`
	Error               string `json:"error,omitempty"`
	CircuitBreakerState string `json:"circuit_breaker_state,omitempty"`
	FailureCount        int    `json:"failure_count,omitempty"`
}

type MetricsFilter struct {
	AgentID       string
	Provider      string
	StartTime     time.Time
	EndTime       time.Time
	Status        string
	Limit         int
}

type CostSummary struct {
	AgentID      string              `json:"agent_id"`
	Period       string              `json:"period"`
	TotalCostUSD float64             `json:"total_cost_usd"`
	ByProvider   map[string]float64  `json:"by_provider"`
	ByModel      map[string]float64  `json:"by_model"`
	RequestCount int                 `json:"request_count"`
	TokensUsed   int                 `json:"tokens_used"`
}

const StandardDimension = 1536 // OpenAI standard for cross-model compatibility