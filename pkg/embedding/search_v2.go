package embedding

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// SearchServiceV2 handles enhanced search operations across multiple models
type SearchServiceV2 struct {
	repository       *Repository
	dimensionAdapter *DimensionAdapter
	db               *sql.DB
}

// NewSearchServiceV2 creates a new enhanced search service
func NewSearchServiceV2(db *sql.DB, repository *Repository, dimensionAdapter *DimensionAdapter) *SearchServiceV2 {
	return &SearchServiceV2{
		db:               db,
		repository:       repository,
		dimensionAdapter: dimensionAdapter,
	}
}

// CrossModelSearchRequest represents a cross-model search request
type CrossModelSearchRequest struct {
	Query           string                 `json:"query"`
	QueryEmbedding  []float32              `json:"query_embedding,omitempty"`
	SearchModel     string                 `json:"search_model"`
	IncludeModels   []string               `json:"include_models"`
	ExcludeModels   []string               `json:"exclude_models"`
	TenantID        uuid.UUID              `json:"tenant_id"`
	ContextID       *uuid.UUID             `json:"context_id,omitempty"`
	Limit           int                    `json:"limit"`
	MinSimilarity   float64                `json:"min_similarity"`
	MetadataFilter  map[string]interface{} `json:"metadata_filter"`
	IncludeAgents   []string               `json:"include_agents"`
	ExcludeAgents   []string               `json:"exclude_agents"`
	TaskType        string                 `json:"task_type"`
	TimeRangeFilter *TimeRangeFilter       `json:"time_range_filter"`
}

// TimeRangeFilter filters results by time range
type TimeRangeFilter struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// CrossModelSearchResult represents a search result across models
type CrossModelSearchResult struct {
	ID                uuid.UUID              `json:"id"`
	ContextID         uuid.UUID              `json:"context_id"`
	Content           string                 `json:"content"`
	OriginalModel     string                 `json:"original_model"`
	OriginalDimension int                    `json:"original_dimension"`
	RawSimilarity     float64                `json:"raw_similarity"`
	NormalizedScore   float64                `json:"normalized_score"`
	ModelQualityScore float64                `json:"model_quality_score"`
	FinalScore        float64                `json:"final_score"`
	AgentID           string                 `json:"agent_id"`
	Metadata          map[string]interface{} `json:"metadata"`
	CreatedAt         string                 `json:"created_at"`
}

// CrossModelSearch performs search across embeddings from different models
func (s *SearchServiceV2) CrossModelSearch(ctx context.Context, req CrossModelSearchRequest) ([]CrossModelSearchResult, error) {
	// Validate request
	if err := s.validateSearchRequest(req); err != nil {
		return nil, err
	}

	// Determine target dimension based on search model
	targetDimension := StandardDimension
	if req.SearchModel != "" {
		if model, err := s.repository.GetModelByName(ctx, req.SearchModel); err == nil {
			targetDimension = model.Dimensions
		}
	}

	// Build SQL query
	query, args := s.buildCrossModelSearchQuery(req, targetDimension)

	// Execute search
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute cross-model search: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	// Parse results
	var results []CrossModelSearchResult
	for rows.Next() {
		var result CrossModelSearchResult
		var metadataJSON []byte
		var embedding pq.Float32Array

		err := rows.Scan(
			&result.ID,
			&result.ContextID,
			&result.Content,
			&result.OriginalModel,
			&result.OriginalDimension,
			&embedding,
			&result.RawSimilarity,
			&result.AgentID,
			&metadataJSON,
			&result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}

		// Parse metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				result.Metadata = make(map[string]interface{})
			}
		}

		// Calculate normalized score
		result.NormalizedScore = s.normalizeScore(
			result.RawSimilarity,
			result.OriginalModel,
			req.SearchModel,
			result.OriginalDimension,
			targetDimension,
		)

		// Get model quality score
		result.ModelQualityScore = s.getModelQualityScore(result.OriginalModel)

		// Calculate final score combining similarity and quality
		result.FinalScore = s.calculateFinalScore(
			result.NormalizedScore,
			result.ModelQualityScore,
			req.TaskType,
		)

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	// Sort by final score
	sort.Slice(results, func(i, j int) bool {
		return results[i].FinalScore > results[j].FinalScore
	})

	// Apply limit
	if req.Limit > 0 && len(results) > req.Limit {
		results = results[:req.Limit]
	}

	return results, nil
}

// HybridSearch performs hybrid search combining semantic and keyword search
func (s *SearchServiceV2) HybridSearch(ctx context.Context, req HybridSearchRequest) ([]HybridSearchResult, error) {
	// Perform semantic search
	semanticResults, err := s.semanticSearch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("semantic search failed: %w", err)
	}

	// Perform keyword search
	keywordResults, err := s.keywordSearch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("keyword search failed: %w", err)
	}

	// Merge and rank results
	merged := s.mergeResults(semanticResults, keywordResults, req.HybridWeight)

	// Apply limit
	if req.Limit > 0 && len(merged) > req.Limit {
		merged = merged[:req.Limit]
	}

	return merged, nil
}

// Private methods

func (s *SearchServiceV2) validateSearchRequest(req CrossModelSearchRequest) error {
	if len(req.Query) == 0 && len(req.QueryEmbedding) == 0 {
		return fmt.Errorf("either query or query_embedding must be provided")
	}

	if req.Limit <= 0 {
		req.Limit = 10
	} else if req.Limit > 100 {
		req.Limit = 100
	}

	if req.MinSimilarity <= 0 {
		req.MinSimilarity = 0.7
	}

	return nil
}

func (s *SearchServiceV2) buildCrossModelSearchQuery(req CrossModelSearchRequest, targetDimension int) (string, []interface{}) {
	query := `
		WITH normalized_embeddings AS (
			SELECT 
				e.id,
				e.context_id,
				e.content,
				e.model_name as original_model,
				e.model_dimensions as original_dimension,
				e.embedding,
				e.metadata,
				e.created_at,
				COALESCE(e.metadata->>'agent_id', '') as agent_id,
				-- Calculate similarity based on normalized dimensions
				CASE 
					WHEN e.model_dimensions = $1 THEN
						1 - (e.embedding <=> $2::vector)
					ELSE
						-- Apply dimension normalization penalty
						(1 - (e.embedding <=> $2::vector)) * 
						(1 - ABS(e.model_dimensions - $1)::float / GREATEST(e.model_dimensions, $1)::float * 0.1)
				END as similarity
			FROM mcp.embeddings e
			WHERE e.tenant_id = $3
	`

	args := []interface{}{targetDimension, pq.Array(req.QueryEmbedding), req.TenantID}
	argCount := 3

	// Add filters
	if req.ContextID != nil {
		argCount++
		query += fmt.Sprintf(" AND e.context_id = $%d", argCount)
		args = append(args, *req.ContextID)
	}

	if len(req.IncludeModels) > 0 {
		argCount++
		query += fmt.Sprintf(" AND e.model_name = ANY($%d)", argCount)
		args = append(args, pq.Array(req.IncludeModels))
	}

	if len(req.ExcludeModels) > 0 {
		argCount++
		query += fmt.Sprintf(" AND e.model_name != ALL($%d)", argCount)
		args = append(args, pq.Array(req.ExcludeModels))
	}

	if len(req.IncludeAgents) > 0 {
		argCount++
		query += fmt.Sprintf(" AND e.metadata->>'agent_id' = ANY($%d)", argCount)
		args = append(args, pq.Array(req.IncludeAgents))
	}

	if len(req.ExcludeAgents) > 0 {
		argCount++
		query += fmt.Sprintf(" AND e.metadata->>'agent_id' != ALL($%d)", argCount)
		args = append(args, pq.Array(req.ExcludeAgents))
	}

	if req.MetadataFilter != nil && len(req.MetadataFilter) > 0 {
		argCount++
		query += fmt.Sprintf(" AND e.metadata @> $%d", argCount)
		metadataJSON, _ := json.Marshal(req.MetadataFilter)
		args = append(args, metadataJSON)
	}

	if req.TimeRangeFilter != nil {
		if req.TimeRangeFilter.StartTime != "" {
			argCount++
			query += fmt.Sprintf(" AND e.created_at >= $%d", argCount)
			args = append(args, req.TimeRangeFilter.StartTime)
		}
		if req.TimeRangeFilter.EndTime != "" {
			argCount++
			query += fmt.Sprintf(" AND e.created_at <= $%d", argCount)
			args = append(args, req.TimeRangeFilter.EndTime)
		}
	}

	// Close CTE and select results
	query += fmt.Sprintf(`
		)
		SELECT 
			id,
			context_id,
			content,
			original_model,
			original_dimension,
			embedding,
			similarity,
			agent_id,
			metadata,
			created_at
		FROM normalized_embeddings
		WHERE similarity >= $%d
		ORDER BY similarity DESC
		LIMIT $%d
	`, argCount+1, argCount+2)

	args = append(args, req.MinSimilarity, req.Limit)

	return query, args
}

func (s *SearchServiceV2) normalizeScore(rawScore float64, sourceModel, targetModel string, sourceDim, targetDim int) float64 {
	// Base normalization
	normalized := rawScore

	// Apply dimension difference penalty
	if sourceDim != targetDim {
		dimRatio := float64(min(sourceDim, targetDim)) / float64(max(sourceDim, targetDim))
		normalized *= (0.9 + 0.1*dimRatio) // 10% max penalty for dimension mismatch
	}

	// Apply model-specific calibration
	modelCalibration := s.getModelCalibration(sourceModel, targetModel)
	normalized *= modelCalibration

	return math.Min(1.0, math.Max(0.0, normalized))
}

func (s *SearchServiceV2) getModelQualityScore(model string) float64 {
	// Model quality scores based on empirical performance
	qualityScores := map[string]float64{
		"text-embedding-3-large":       0.95,
		"text-embedding-3-small":       0.90,
		"text-embedding-ada-002":       0.85,
		"voyage-large-2":               0.93,
		"voyage-2":                     0.88,
		"voyage-code-2":                0.92, // High score for code
		"amazon.titan-embed-text-v2:0": 0.87,
		"cohere.embed-english-v3":      0.89,
		"cohere.embed-multilingual-v3": 0.91, // High score for multilingual
	}

	if score, ok := qualityScores[model]; ok {
		return score
	}
	return 0.80 // Default score for unknown models
}

func (s *SearchServiceV2) getModelCalibration(sourceModel, targetModel string) float64 {
	// Calibration factors for cross-model comparison
	// These would be learned from evaluation data in production
	if sourceModel == targetModel {
		return 1.0
	}

	// Simple heuristic based on model families
	sourceFamily := getModelFamily(sourceModel)
	targetFamily := getModelFamily(targetModel)

	if sourceFamily == targetFamily {
		return 0.95 // Same family, minor adjustment
	}

	// Cross-family calibration
	calibrationMap := map[string]map[string]float64{
		"openai": {
			"voyage":  0.92,
			"bedrock": 0.90,
			"cohere":  0.88,
		},
		"voyage": {
			"openai":  0.93,
			"bedrock": 0.91,
			"cohere":  0.89,
		},
		"bedrock": {
			"openai": 0.91,
			"voyage": 0.90,
			"cohere": 0.92,
		},
	}

	if cal, ok := calibrationMap[sourceFamily][targetFamily]; ok {
		return cal
	}

	return 0.85 // Default cross-family calibration
}

func (s *SearchServiceV2) calculateFinalScore(similarity, quality float64, taskType string) float64 {
	// Task-specific weighting
	var simWeight, qualWeight float64

	switch taskType {
	case "research":
		simWeight = 0.6
		qualWeight = 0.4 // Quality matters more for research
	case "code_analysis":
		simWeight = 0.7
		qualWeight = 0.3
	case "multilingual":
		simWeight = 0.65
		qualWeight = 0.35
	default:
		simWeight = 0.8
		qualWeight = 0.2
	}

	return simWeight*similarity + qualWeight*quality
}

// Hybrid search types

type HybridSearchRequest struct {
	Query          string                 `json:"query"`
	QueryEmbedding []float32              `json:"query_embedding,omitempty"`
	Keywords       []string               `json:"keywords"`
	TenantID       uuid.UUID              `json:"tenant_id"`
	Limit          int                    `json:"limit"`
	HybridWeight   float64                `json:"hybrid_weight"` // 0.0 = keyword only, 1.0 = semantic only
	MetadataFilter map[string]interface{} `json:"metadata_filter"`
}

type HybridSearchResult struct {
	CrossModelSearchResult
	KeywordScore  float64 `json:"keyword_score"`
	SemanticScore float64 `json:"semantic_score"`
	HybridScore   float64 `json:"hybrid_score"`
}

func (s *SearchServiceV2) semanticSearch(ctx context.Context, req HybridSearchRequest) ([]HybridSearchResult, error) {
	// Convert to cross-model search request
	crossReq := CrossModelSearchRequest{
		Query:          req.Query,
		QueryEmbedding: req.QueryEmbedding,
		TenantID:       req.TenantID,
		Limit:          req.Limit * 2, // Get more for merging
		MetadataFilter: req.MetadataFilter,
	}

	results, err := s.CrossModelSearch(ctx, crossReq)
	if err != nil {
		return nil, err
	}

	// Convert to hybrid results
	hybridResults := make([]HybridSearchResult, len(results))
	for i, r := range results {
		hybridResults[i] = HybridSearchResult{
			CrossModelSearchResult: r,
			SemanticScore:          r.FinalScore,
		}
	}

	return hybridResults, nil
}

func (s *SearchServiceV2) keywordSearch(ctx context.Context, req HybridSearchRequest) ([]HybridSearchResult, error) {
	// Perform full-text search
	query := `
		SELECT 
			e.id,
			e.context_id,
			e.content,
			e.model_name,
			e.model_dimensions,
			e.metadata,
			e.created_at,
			COALESCE(e.metadata->>'agent_id', '') as agent_id,
			ts_rank_cd(to_tsvector('english', e.content), query) as rank
		FROM mcp.embeddings e,
			to_tsquery('english', $1) query
		WHERE e.tenant_id = $2
			AND to_tsvector('english', e.content) @@ query
		ORDER BY rank DESC
		LIMIT $3
	`

	// Build query string from keywords
	queryStr := buildTsQuery(req.Keywords)
	
	rows, err := s.db.QueryContext(ctx, query, queryStr, req.TenantID, req.Limit*2)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var results []HybridSearchResult
	for rows.Next() {
		var r HybridSearchResult
		var metadataJSON []byte
		var rank float64

		err := rows.Scan(
			&r.ID,
			&r.ContextID,
			&r.Content,
			&r.OriginalModel,
			&r.OriginalDimension,
			&metadataJSON,
			&r.CreatedAt,
			&r.AgentID,
			&rank,
		)
		if err != nil {
			return nil, err
		}

		// Parse metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &r.Metadata); err != nil {
				// Log warning but continue - metadata is non-critical
				_ = err
			}
		}

		// Normalize keyword score to 0-1 range
		r.KeywordScore = math.Min(1.0, rank/4.0) // ts_rank typically returns 0-4
		results = append(results, r)
	}

	return results, nil
}

func (s *SearchServiceV2) mergeResults(semantic, keyword []HybridSearchResult, weight float64) []HybridSearchResult {
	// Create map for deduplication
	resultMap := make(map[uuid.UUID]*HybridSearchResult)

	// Add semantic results
	for i := range semantic {
		r := semantic[i]
		r.HybridScore = weight * r.SemanticScore
		resultMap[r.ID] = &r
	}

	// Merge keyword results
	for i := range keyword {
		k := keyword[i]
		if existing, ok := resultMap[k.ID]; ok {
			// Combine scores
			existing.KeywordScore = k.KeywordScore
			existing.HybridScore = weight*existing.SemanticScore + (1-weight)*k.KeywordScore
		} else {
			// Add new result
			k.HybridScore = (1 - weight) * k.KeywordScore
			resultMap[k.ID] = &k
		}
	}

	// Convert to slice
	results := make([]HybridSearchResult, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, *r)
	}

	// Sort by hybrid score
	sort.Slice(results, func(i, j int) bool {
		return results[i].HybridScore > results[j].HybridScore
	})

	return results
}

// Helper functions

func getModelFamily(model string) string {
	if contains(model, "text-embedding") || contains(model, "ada") {
		return "openai"
	}
	if contains(model, "voyage") {
		return "voyage"
	}
	if contains(model, "titan") || contains(model, "anthropic") {
		return "bedrock"
	}
	if contains(model, "cohere") {
		return "cohere"
	}
	return "unknown"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || len(substr) > 0 && len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func buildTsQuery(keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}
	
	// Join keywords with AND operator
	query := ""
	for i, kw := range keywords {
		if i > 0 {
			query += " & "
		}
		query += kw
	}
	return query
}

// min and max functions are defined in service_v2.go