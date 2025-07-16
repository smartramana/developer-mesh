package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/agents"
	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/embedding"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/google/uuid"
)

// SearchServiceAdapter adapts the embedding service for search operations
type SearchServiceAdapter struct {
	embeddingService *embedding.ServiceV2
	searchService    embedding.AdvancedSearchService
	cache            cache.Cache
	logger           observability.Logger
	mu               sync.RWMutex
}

// NewSearchServiceAdapter creates a new search service adapter
func NewSearchServiceAdapter(embeddingService *embedding.ServiceV2, logger observability.Logger) *SearchServiceAdapter {
	return &SearchServiceAdapter{
		embeddingService: embeddingService,
		logger:           logger,
	}
}

// SetSearchService allows injecting the search service if available
func (a *SearchServiceAdapter) SetSearchService(searchService embedding.AdvancedSearchService) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.searchService = searchService
}

// SetCache allows injecting a cache instance
func (a *SearchServiceAdapter) SetCache(cache cache.Cache) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cache = cache
}

// Search performs a similarity search using the embedding service
func (a *SearchServiceAdapter) Search(ctx context.Context, req embedding.SearchRequest) ([]embedding.EmbeddingSearchResult, error) {
	a.mu.RLock()
	searchSvc := a.searchService
	a.mu.RUnlock()

	// If we have a dedicated search service, use it
	if searchSvc != nil {
		return a.performDirectSearch(ctx, searchSvc, req)
	}

	// Otherwise, fall back to using the embedding service's repository
	return a.performRepositorySearch(ctx, req)
}

// performDirectSearch uses the dedicated search service
func (a *SearchServiceAdapter) performDirectSearch(ctx context.Context, searchSvc embedding.AdvancedSearchService, req embedding.SearchRequest) ([]embedding.EmbeddingSearchResult, error) {
	// Convert to cross-model search request
	crossReq := embedding.CrossModelSearchRequest{
		QueryEmbedding: req.QueryEmbedding,
		SearchModel:    req.ModelName,
		TenantID:       req.TenantID,
		ContextID:      req.ContextID,
		Limit:          req.Limit,
		MinSimilarity:  float32(req.Threshold),
		MetadataFilter: make(map[string]interface{}),
	}

	// Parse metadata filter if provided
	if len(req.MetadataFilter) > 0 {
		// Note: In production, properly unmarshal the JSON metadata filter
		// For now, we'll use an empty filter
		a.logger.Debug("Metadata filter provided but not parsed", map[string]interface{}{
			"filter_size": len(req.MetadataFilter),
		})
	}

	// Perform cross-model search
	results, err := searchSvc.CrossModelSearch(ctx, crossReq)
	if err != nil {
		return nil, fmt.Errorf("cross-model search failed: %w", err)
	}

	// Convert results to expected format
	searchResults := make([]embedding.EmbeddingSearchResult, len(results))
	for i, result := range results {
		sr := embedding.EmbeddingSearchResult{
			ID:            result.ID,
			Content:       result.Content,
			Similarity:    float64(result.FinalScore),
			Metadata:      mustMarshalJSON(result.Metadata),
			ModelProvider: getProviderFromModel(result.OriginalModel),
		}
		if result.ContextID != nil {
			sr.ContextID = *result.ContextID
		}
		searchResults[i] = sr
	}

	return searchResults, nil
}

// performRepositorySearch uses the embedding service's repository directly
func (a *SearchServiceAdapter) performRepositorySearch(ctx context.Context, req embedding.SearchRequest) ([]embedding.EmbeddingSearchResult, error) {
	// This is a simplified implementation that would need access to the repository
	// In production, the embedding service should expose search capabilities
	a.logger.Warn("Direct repository search not implemented, returning empty results", nil)
	return []embedding.EmbeddingSearchResult{}, nil
}

// EmbeddingServiceAdapter provides batch processing and caching for embeddings
type EmbeddingServiceAdapter struct {
	serviceV2    *embedding.ServiceV2
	cache        cache.Cache
	metrics      observability.MetricsClient
	logger       observability.Logger
	progressFunc func(float64)
	mu           sync.RWMutex
}

// NewEmbeddingServiceAdapter creates a new embedding service adapter
func NewEmbeddingServiceAdapter(serviceV2 *embedding.ServiceV2, cache cache.Cache, metrics observability.MetricsClient, logger observability.Logger) *EmbeddingServiceAdapter {
	return &EmbeddingServiceAdapter{
		serviceV2: serviceV2,
		cache:     cache,
		metrics:   metrics,
		logger:    logger,
	}
}

// SetProgressCallback sets a callback for batch processing progress
func (a *EmbeddingServiceAdapter) SetProgressCallback(fn func(float64)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.progressFunc = fn
}

// GenerateBatch generates embeddings for multiple texts with caching and progress tracking
func (a *EmbeddingServiceAdapter) GenerateBatch(ctx context.Context, agentID string, texts []string, taskType string) ([][]float32, error) {
	const batchSize = 100
	var results [][]float32
	totalTexts := len(texts)

	// Track overall timing
	start := time.Now()
	defer func() {
		if a.metrics != nil {
			a.metrics.RecordDuration("embedding.batch.total", time.Since(start))
			a.metrics.IncrementCounter("embedding.batch.processed", float64(totalTexts))
		}
	}()

	// Process in batches
	for i := 0; i < totalTexts; i += batchSize {
		end := min(i+batchSize, totalTexts)
		batch := texts[i:end]

		// Check cache for each text
		batchResults, err := a.processBatchWithCache(ctx, agentID, batch, taskType)
		if err != nil {
			a.logger.Error("Batch processing failed", map[string]interface{}{
				"error":       err.Error(),
				"batch_start": i,
				"batch_end":   end,
			})
			return nil, fmt.Errorf("failed to process batch %d-%d: %w", i, end, err)
		}

		results = append(results, batchResults...)

		// Report progress
		if a.progressFunc != nil {
			progress := float64(end) / float64(totalTexts)
			a.progressFunc(progress)
		}

		// Record batch metrics
		if a.metrics != nil {
			a.metrics.IncrementCounter("embedding.batch.completed", 1)
		}
	}

	return results, nil
}

// processBatchWithCache processes a batch of texts with cache support
func (a *EmbeddingServiceAdapter) processBatchWithCache(ctx context.Context, agentID string, texts []string, taskType string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	uncachedIndices := []int{}
	uncachedTexts := []string{}

	// Check cache for each text
	for i, text := range texts {
		if a.cache != nil {
			cacheKey := a.generateCacheKey(agentID, taskType, text)
			var cached []float32
			if err := a.cache.Get(ctx, cacheKey, &cached); err == nil {
				// Use cached embedding
				results[i] = cached
				continue
			}
		}
		uncachedIndices = append(uncachedIndices, i)
		uncachedTexts = append(uncachedTexts, text)
	}

	// Generate embeddings for uncached texts
	if len(uncachedTexts) > 0 {
		// Create batch request
		reqs := make([]embedding.GenerateEmbeddingRequest, len(uncachedTexts))
		for i, text := range uncachedTexts {
			reqs[i] = embedding.GenerateEmbeddingRequest{
				AgentID:   agentID,
				Text:      text,
				TaskType:  parseTaskType(taskType),
				RequestID: uuid.New().String(),
			}
		}

		// Generate embeddings
		responses, err := a.serviceV2.BatchGenerateEmbeddings(ctx, reqs)
		if err != nil {
			return nil, fmt.Errorf("batch generation failed: %w", err)
		}

		// Store results and update cache
		for i, resp := range responses {
			originalIdx := uncachedIndices[i]

			// Extract embedding from response metadata
			if embeddings, ok := resp.Metadata["normalized_embedding"].([]float32); ok {
				results[originalIdx] = embeddings
			} else {
				// Fall back to generating individual embedding
				indivResp, err := a.serviceV2.GenerateEmbedding(ctx, reqs[i])
				if err != nil {
					return nil, fmt.Errorf("failed to generate embedding for text %d: %w", originalIdx, err)
				}
				if embeddings, ok := indivResp.Metadata["normalized_embedding"].([]float32); ok {
					results[originalIdx] = embeddings
				} else {
					return nil, fmt.Errorf("no embedding found in response for text %d", originalIdx)
				}
			}

			// Update cache
			if a.cache != nil {
				cacheKey := a.generateCacheKey(agentID, taskType, uncachedTexts[i])
				_ = a.cache.Set(ctx, cacheKey, results[originalIdx], 1*time.Hour)
			}
		}
	}

	// Record cache metrics
	if a.metrics != nil {
		a.metrics.IncrementCounter("embedding.cache.hits", float64(len(texts)-len(uncachedTexts)))
		a.metrics.IncrementCounter("embedding.cache.misses", float64(len(uncachedTexts)))
	}

	return results, nil
}

// generateCacheKey creates a deterministic cache key
func (a *EmbeddingServiceAdapter) generateCacheKey(agentID, taskType, text string) string {
	return fmt.Sprintf("emb:%s:%s:%s", agentID, taskType, hashString(text))
}

// MetricsRepositoryAdapter provides access to embedding metrics
type MetricsRepositoryAdapter struct {
	metricsRepo embedding.MetricsRepository
	logger      observability.Logger
}

// NewMetricsRepositoryAdapter creates a new metrics repository adapter
func NewMetricsRepositoryAdapter(serviceV2 *embedding.ServiceV2, logger observability.Logger) *MetricsRepositoryAdapter {
	// Note: In production, ServiceV2 should expose its metrics repository
	// For now, we return an adapter that logs warnings
	return &MetricsRepositoryAdapter{
		logger: logger,
	}
}

// SetMetricsRepository allows injecting the metrics repository if available
func (a *MetricsRepositoryAdapter) SetMetricsRepository(repo embedding.MetricsRepository) {
	a.metricsRepo = repo
}

// GetAgentCosts retrieves cost metrics for an agent
func (a *MetricsRepositoryAdapter) GetAgentCosts(ctx context.Context, agentID string, period time.Duration) (*embedding.CostSummary, error) {
	if a.metricsRepo == nil {
		a.logger.Warn("Metrics repository not available", map[string]interface{}{
			"agent_id": agentID,
		})
		return &embedding.CostSummary{
			AgentID:      agentID,
			Period:       period.String(),
			TotalCostUSD: 0,
			ByProvider:   make(map[string]float64),
			ByModel:      make(map[string]float64),
			RequestCount: 0,
			TokensUsed:   0,
		}, nil
	}

	return a.metricsRepo.GetAgentCosts(ctx, agentID, period)
}

// GetMetrics retrieves embedding metrics with filters
func (a *MetricsRepositoryAdapter) GetMetrics(ctx context.Context, filter embedding.MetricsFilter) ([]*embedding.EmbeddingMetric, error) {
	if a.metricsRepo == nil {
		return []*embedding.EmbeddingMetric{}, nil
	}

	return a.metricsRepo.GetMetrics(ctx, filter)
}

// Helper functions

func parseTaskType(taskType string) agents.TaskType {
	if taskType == "" {
		return agents.TaskTypeGeneralQA
	}
	return agents.TaskType(taskType)
}

func getProviderFromModel(model string) string {
	// Simple heuristic to determine provider from model name
	if contains(model, "text-embedding") || contains(model, "ada") {
		return "openai"
	}
	if contains(model, "voyage") {
		return "voyage"
	}
	if contains(model, "titan") || contains(model, "anthropic") {
		return "amazon"
	}
	if contains(model, "cohere") {
		return "cohere"
	}
	return "unknown"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func hashString(s string) string {
	// Simple hash for cache key
	if len(s) > 32 {
		return fmt.Sprintf("%x", s[:32])
	}
	return fmt.Sprintf("%x", s)
}

func mustMarshalJSON(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
