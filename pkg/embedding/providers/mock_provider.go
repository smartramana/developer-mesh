package providers

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// MockProvider implements Provider interface for testing
type MockProvider struct {
	mu               sync.RWMutex
	name             string
	models           map[string]ModelInfo
	failureRate      float64
	latency          time.Duration
	requestCount     int
	failAfterCount   int
	healthCheckError error
	closed           bool
	
	// For tracking calls
	generateCalls       []GenerateEmbeddingRequest
	batchGenerateCalls  []BatchGenerateEmbeddingRequest
}

// MockProviderOption configures a MockProvider
type MockProviderOption func(*MockProvider)

// WithFailureRate sets the failure rate (0.0 to 1.0)
func WithFailureRate(rate float64) MockProviderOption {
	return func(m *MockProvider) {
		m.failureRate = rate
	}
}

// WithLatency sets the simulated latency
func WithLatency(latency time.Duration) MockProviderOption {
	return func(m *MockProvider) {
		m.latency = latency
	}
}

// WithFailAfter causes failures after N requests
func WithFailAfter(count int) MockProviderOption {
	return func(m *MockProvider) {
		m.failAfterCount = count
	}
}

// WithHealthCheckError sets a health check error
func WithHealthCheckError(err error) MockProviderOption {
	return func(m *MockProvider) {
		m.healthCheckError = err
	}
}

// NewMockProvider creates a new mock provider
func NewMockProvider(name string, opts ...MockProviderOption) *MockProvider {
	m := &MockProvider{
		name:            name,
		latency:         50 * time.Millisecond,
		generateCalls:   make([]GenerateEmbeddingRequest, 0),
		batchGenerateCalls: make([]BatchGenerateEmbeddingRequest, 0),
	}
	
	// Apply options
	for _, opt := range opts {
		opt(m)
	}
	
	// Initialize default models
	m.models = map[string]ModelInfo{
		"mock-model-small": {
			Name:                "mock-model-small",
			DisplayName:         "Mock Small Model",
			Dimensions:          1536,
			MaxTokens:           8192,
			CostPer1MTokens:     0.02,
			SupportedTaskTypes:  []string{"default", "general_qa"},
			SupportsDimensionReduction: true,
			MinDimensions:       512,
			IsActive:            true,
		},
		"mock-model-large": {
			Name:                "mock-model-large",
			DisplayName:         "Mock Large Model",
			Dimensions:          3072,
			MaxTokens:           8192,
			CostPer1MTokens:     0.13,
			SupportedTaskTypes:  []string{"default", "general_qa", "research"},
			SupportsDimensionReduction: true,
			MinDimensions:       256,
			IsActive:            true,
		},
		"mock-model-code": {
			Name:                "mock-model-code",
			DisplayName:         "Mock Code Model",
			Dimensions:          1024,
			MaxTokens:           16384,
			CostPer1MTokens:     0.10,
			SupportedTaskTypes:  []string{"code_analysis"},
			IsActive:            true,
		},
		"mock-model-titan": {
			Name:                "mock-model-titan",
			DisplayName:         "Mock Titan Model",
			Dimensions:          1024,
			MaxTokens:           8192,
			CostPer1MTokens:     0.02,
			SupportedTaskTypes:  []string{"default", "general_qa", "multilingual"},
			IsActive:            true,
		},
	}
	
	return m
}

// Name returns the provider name
func (m *MockProvider) Name() string {
	return m.name
}

// GenerateEmbedding generates a mock embedding
func (m *MockProvider) GenerateEmbedding(ctx context.Context, req GenerateEmbeddingRequest) (*EmbeddingResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return nil, &ProviderError{
			Provider: m.name,
			Code:     "PROVIDER_CLOSED",
			Message:  "provider is closed",
		}
	}
	
	// Track the call
	m.generateCalls = append(m.generateCalls, req)
	m.requestCount++
	
	// Simulate latency
	if m.latency > 0 {
		time.Sleep(m.latency)
	}
	
	// Check for failures
	if m.shouldFail() {
		return nil, &ProviderError{
			Provider:    m.name,
			Code:        "MOCK_FAILURE",
			Message:     "simulated failure",
			StatusCode:  500,
			IsRetryable: true,
		}
	}
	
	// Get model info
	model, exists := m.models[req.Model]
	if !exists {
		return nil, &ProviderError{
			Provider:   m.name,
			Code:       "MODEL_NOT_FOUND",
			Message:    fmt.Sprintf("model %s not found", req.Model),
			StatusCode: 400,
		}
	}
	
	// Generate mock embedding
	embedding := m.generateMockEmbedding(req.Text, model.Dimensions)
	
	// Calculate mock token count (roughly 4 chars per token)
	tokensUsed := len(req.Text) / 4
	if tokensUsed == 0 {
		tokensUsed = 1
	}
	
	return &EmbeddingResponse{
		Embedding:   embedding,
		Model:       req.Model,
		Dimensions:  model.Dimensions,
		TokensUsed:  tokensUsed,
		Metadata:    req.Metadata,
		ProviderInfo: ProviderMetadata{
			Provider:  m.name,
			LatencyMs: int64(m.latency / time.Millisecond),
		},
	}, nil
}

// BatchGenerateEmbeddings generates mock embeddings for multiple texts
func (m *MockProvider) BatchGenerateEmbeddings(ctx context.Context, req BatchGenerateEmbeddingRequest) (*BatchEmbeddingResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return nil, &ProviderError{
			Provider: m.name,
			Code:     "PROVIDER_CLOSED",
			Message:  "provider is closed",
		}
	}
	
	// Track the call
	m.batchGenerateCalls = append(m.batchGenerateCalls, req)
	m.requestCount += len(req.Texts)
	
	// Simulate latency
	if m.latency > 0 {
		time.Sleep(m.latency * time.Duration(len(req.Texts)))
	}
	
	// Check for failures
	if m.shouldFail() {
		return nil, &ProviderError{
			Provider:    m.name,
			Code:        "MOCK_FAILURE",
			Message:     "simulated batch failure",
			StatusCode:  500,
			IsRetryable: true,
		}
	}
	
	// Get model info
	model, exists := m.models[req.Model]
	if !exists {
		return nil, &ProviderError{
			Provider:   m.name,
			Code:       "MODEL_NOT_FOUND",
			Message:    fmt.Sprintf("model %s not found", req.Model),
			StatusCode: 400,
		}
	}
	
	// Generate mock embeddings
	embeddings := make([][]float32, len(req.Texts))
	totalTokens := 0
	
	for i, text := range req.Texts {
		embeddings[i] = m.generateMockEmbedding(text, model.Dimensions)
		tokens := len(text) / 4
		if tokens == 0 {
			tokens = 1
		}
		totalTokens += tokens
	}
	
	return &BatchEmbeddingResponse{
		Embeddings:  embeddings,
		Model:       req.Model,
		Dimensions:  model.Dimensions,
		TotalTokens: totalTokens,
		Metadata:    req.Metadata,
		ProviderInfo: ProviderMetadata{
			Provider:  m.name,
			LatencyMs: int64(m.latency * time.Duration(len(req.Texts)) / time.Millisecond),
		},
	}, nil
}

// GetSupportedModels returns the list of mock models
func (m *MockProvider) GetSupportedModels() []ModelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	models := make([]ModelInfo, 0, len(m.models))
	for _, model := range m.models {
		models = append(models, model)
	}
	return models
}

// GetModel returns information about a specific model
func (m *MockProvider) GetModel(modelName string) (ModelInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	model, exists := m.models[modelName]
	if !exists {
		return ModelInfo{}, &ProviderError{
			Provider: m.name,
			Code:     "MODEL_NOT_FOUND",
			Message:  fmt.Sprintf("model %s not found", modelName),
		}
	}
	return model, nil
}

// HealthCheck verifies the provider is functioning
func (m *MockProvider) HealthCheck(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.closed {
		return fmt.Errorf("provider is closed")
	}
	
	if m.healthCheckError != nil {
		return m.healthCheckError
	}
	
	return nil
}

// Close cleans up resources
func (m *MockProvider) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.closed = true
	return nil
}

// Test helper methods

// GetGenerateCalls returns all generate embedding calls made
func (m *MockProvider) GetGenerateCalls() []GenerateEmbeddingRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	calls := make([]GenerateEmbeddingRequest, len(m.generateCalls))
	copy(calls, m.generateCalls)
	return calls
}

// GetBatchGenerateCalls returns all batch generate embedding calls made
func (m *MockProvider) GetBatchGenerateCalls() []BatchGenerateEmbeddingRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	calls := make([]BatchGenerateEmbeddingRequest, len(m.batchGenerateCalls))
	copy(calls, m.batchGenerateCalls)
	return calls
}

// ResetCalls clears the call history
func (m *MockProvider) ResetCalls() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.generateCalls = m.generateCalls[:0]
	m.batchGenerateCalls = m.batchGenerateCalls[:0]
	m.requestCount = 0
}

// SetModel adds or updates a model
func (m *MockProvider) SetModel(model ModelInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.models[model.Name] = model
}

// Private methods

func (m *MockProvider) shouldFail() bool {
	// Check fail after count
	if m.failAfterCount > 0 && m.requestCount > m.failAfterCount {
		return true
	}
	
	// Check failure rate
	if m.failureRate > 0 && rand.Float64() < m.failureRate {
		return true
	}
	
	return false
}

func (m *MockProvider) generateMockEmbedding(text string, dimensions int) []float32 {
	// Generate deterministic but varied embeddings based on text
	embedding := make([]float32, dimensions)
	
	// Use text hash as seed for reproducibility
	hash := 0
	for _, ch := range text {
		hash = hash*31 + int(ch)
	}
	r := rand.New(rand.NewSource(int64(hash)))
	
	// Generate values with some structure
	for i := 0; i < dimensions; i++ {
		// Create some patterns in the embedding
		base := r.Float64()*2 - 1 // Range -1 to 1
		
		// Add some periodic components
		wave1 := math.Sin(float64(i) * 0.1)
		wave2 := math.Cos(float64(i) * 0.05)
		
		embedding[i] = float32(base*0.7 + wave1*0.2 + wave2*0.1)
	}
	
	// Normalize the vector
	var sum float32
	for _, val := range embedding {
		sum += val * val
	}
	magnitude := float32(math.Sqrt(float64(sum)))
	
	if magnitude > 0 {
		for i := range embedding {
			embedding[i] /= magnitude
		}
	}
	
	return embedding
}