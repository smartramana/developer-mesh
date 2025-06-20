package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/agents"
	"github.com/S-Corkum/devops-mcp/pkg/embedding"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// EmbeddingAPIV2 handles embedding API requests for multi-agent system
type EmbeddingAPIV2 struct {
	embeddingService *embedding.ServiceV2
	agentService     embedding.AgentService
}

// NewEmbeddingAPIV2 creates a new embedding API handler
func NewEmbeddingAPIV2(embeddingService *embedding.ServiceV2, agentService embedding.AgentService) *EmbeddingAPIV2 {
	return &EmbeddingAPIV2{
		embeddingService: embeddingService,
		agentService:     agentService,
	}
}

// RegisterRoutes registers embedding API routes
func (api *EmbeddingAPIV2) RegisterRoutes(router *mux.Router) {
	v2 := router.PathPrefix("/api/v2/embeddings").Subrouter()

	// Embedding generation
	v2.HandleFunc("", api.GenerateEmbedding).Methods("POST")
	v2.HandleFunc("/batch", api.BatchGenerateEmbeddings).Methods("POST")

	// Provider health
	v2.HandleFunc("/providers/health", api.GetProviderHealth).Methods("GET")

	// Agent configuration endpoints
	v2.HandleFunc("/agents", api.CreateAgentConfig).Methods("POST")
	v2.HandleFunc("/agents/{agentId}", api.GetAgentConfig).Methods("GET")
	v2.HandleFunc("/agents/{agentId}", api.UpdateAgentConfig).Methods("PUT")
	v2.HandleFunc("/agents/{agentId}/models", api.GetAgentModels).Methods("GET")
	v2.HandleFunc("/agents/{agentId}/costs", api.GetAgentCosts).Methods("GET")

	// Search endpoints
	v2.HandleFunc("/search", api.SearchEmbeddings).Methods("POST")
	v2.HandleFunc("/search/cross-model", api.CrossModelSearch).Methods("POST")
}

// GenerateEmbedding handles POST /api/v2/embeddings
func (api *EmbeddingAPIV2) GenerateEmbedding(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := getTenantIDFromContext(ctx)

	var req embedding.GenerateEmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Set tenant ID from context
	req.TenantID = tenantID

	// Generate request ID if not provided
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}

	resp, err := api.embeddingService.GenerateEmbedding(ctx, req)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "Failed to generate embedding: "+err.Error())
		return
	}

	sendJSON(w, http.StatusOK, resp)
}

// BatchGenerateEmbeddings handles POST /api/v2/embeddings/batch
func (api *EmbeddingAPIV2) BatchGenerateEmbeddings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := getTenantIDFromContext(ctx)

	var reqs []embedding.GenerateEmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Set tenant ID for all requests
	for i := range reqs {
		reqs[i].TenantID = tenantID
		if reqs[i].RequestID == "" {
			reqs[i].RequestID = uuid.New().String()
		}
	}

	resps, err := api.embeddingService.BatchGenerateEmbeddings(ctx, reqs)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "Failed to generate embeddings: "+err.Error())
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"embeddings": resps,
		"count":      len(resps),
	})
}

// GetProviderHealth handles GET /api/v2/embeddings/providers/health
func (api *EmbeddingAPIV2) GetProviderHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	health := api.embeddingService.GetProviderHealth(ctx)

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"providers": health,
		"timestamp": time.Now().UTC(),
	})
}

// CreateAgentConfig handles POST /api/v2/embeddings/agents
func (api *EmbeddingAPIV2) CreateAgentConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var config agents.AgentConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Set created by from context
	config.CreatedBy = getUserIDFromContext(ctx)

	if err := api.agentService.CreateConfig(ctx, &config); err != nil {
		sendError(w, http.StatusInternalServerError, "Failed to create agent config: "+err.Error())
		return
	}

	sendJSON(w, http.StatusCreated, config)
}

// GetAgentConfig handles GET /api/v2/embeddings/agents/{agentId}
func (api *EmbeddingAPIV2) GetAgentConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	agentID := vars["agentId"]

	config, err := api.agentService.GetConfig(ctx, agentID)
	if err != nil {
		sendError(w, http.StatusNotFound, "Agent config not found: "+err.Error())
		return
	}

	sendJSON(w, http.StatusOK, config)
}

// UpdateAgentConfig handles PUT /api/v2/embeddings/agents/{agentId}
func (api *EmbeddingAPIV2) UpdateAgentConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	agentID := vars["agentId"]

	var update agents.ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Set updated by from context
	update.UpdatedBy = getUserIDFromContext(ctx)

	config, err := api.agentService.UpdateConfig(ctx, agentID, &update)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "Failed to update agent config: "+err.Error())
		return
	}

	sendJSON(w, http.StatusOK, config)
}

// GetAgentModels handles GET /api/v2/embeddings/agents/{agentId}/models
func (api *EmbeddingAPIV2) GetAgentModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	agentID := vars["agentId"]

	// Get task type from query param
	taskType := agents.TaskType(r.URL.Query().Get("task_type"))
	if taskType == "" {
		taskType = agents.TaskTypeGeneralQA
	}

	primary, fallback, err := api.agentService.GetModelsForAgent(ctx, agentID, taskType)
	if err != nil {
		sendError(w, http.StatusNotFound, "Failed to get agent models: "+err.Error())
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"agent_id":        agentID,
		"task_type":       taskType,
		"primary_models":  primary,
		"fallback_models": fallback,
	})
}

// GetAgentCosts handles GET /api/v2/embeddings/agents/{agentId}/costs
func (api *EmbeddingAPIV2) GetAgentCosts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	agentID := vars["agentId"]

	// Mark ctx as used
	_ = ctx

	// Get period from query param (default 30 days)
	periodDays := 30
	if p := r.URL.Query().Get("period_days"); p != "" {
		if days, err := strconv.Atoi(p); err == nil && days > 0 {
			periodDays = days
		}
	}

	// This would call the metrics repository to get cost data
	// For now, return a mock response
	costs := map[string]interface{}{
		"agent_id":       agentID,
		"period_days":    periodDays,
		"total_cost_usd": 45.67,
		"by_provider": map[string]float64{
			"openai":  35.50,
			"bedrock": 10.17,
		},
		"by_model": map[string]float64{
			"text-embedding-3-small":       25.30,
			"text-embedding-3-large":       10.20,
			"amazon.titan-embed-text-v2:0": 10.17,
		},
		"request_count": 125430,
		"tokens_used":   234567890,
	}

	sendJSON(w, http.StatusOK, costs)
}

// SearchEmbeddings handles POST /api/v2/embeddings/search
func (api *EmbeddingAPIV2) SearchEmbeddings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := getTenantIDFromContext(ctx)

	var req EmbeddingSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// For now, return a mock response
	// This would integrate with the enhanced search functionality
	results := []SearchResult{
		{
			ID:         uuid.New(),
			Content:    "Sample matching content",
			Similarity: 0.95,
			Metadata: map[string]interface{}{
				"agent_id": req.AgentID,
				"model":    "text-embedding-3-small",
			},
		},
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
		"query": map[string]interface{}{
			"agent_id": req.AgentID,
			"limit":    req.Limit,
		},
	})

	// Mark tenantID as used
	_ = tenantID
}

// CrossModelSearch handles POST /api/v2/embeddings/search/cross-model
func (api *EmbeddingAPIV2) CrossModelSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := getTenantIDFromContext(ctx)

	var req CrossModelSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// For now, return a mock response
	// This would use the dimension adapter to search across different model embeddings
	results := []CrossModelSearchResult{
		{
			ID:                uuid.New(),
			Content:           "Cross-model matching content",
			OriginalModel:     "text-embedding-3-large",
			OriginalDimension: 3072,
			NormalizedScore:   0.92,
			Metadata: map[string]interface{}{
				"source_agent": "agent-001",
			},
		},
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"results":      results,
		"count":        len(results),
		"search_model": req.SearchModel,
		"included_models": []string{
			"text-embedding-3-small",
			"text-embedding-3-large",
			"amazon.titan-embed-text-v2:0",
		},
	})

	// Mark tenantID as used
	_ = tenantID
}

// Request/Response types

type EmbeddingSearchRequest struct {
	AgentID   string                 `json:"agent_id" validate:"required"`
	Query     string                 `json:"query" validate:"required"`
	Limit     int                    `json:"limit,omitempty"`
	Threshold float64                `json:"threshold,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type SearchResult struct {
	ID         uuid.UUID              `json:"id"`
	Content    string                 `json:"content"`
	Similarity float64                `json:"similarity"`
	Metadata   map[string]interface{} `json:"metadata"`
}

type CrossModelSearchRequest struct {
	Query          string                 `json:"query" validate:"required"`
	SearchModel    string                 `json:"search_model,omitempty"`
	IncludeModels  []string               `json:"include_models,omitempty"`
	ExcludeModels  []string               `json:"exclude_models,omitempty"`
	Limit          int                    `json:"limit,omitempty"`
	MinSimilarity  float64                `json:"min_similarity,omitempty"`
	MetadataFilter map[string]interface{} `json:"metadata_filter,omitempty"`
}

type CrossModelSearchResult struct {
	ID                uuid.UUID              `json:"id"`
	Content           string                 `json:"content"`
	OriginalModel     string                 `json:"original_model"`
	OriginalDimension int                    `json:"original_dimension"`
	NormalizedScore   float64                `json:"normalized_score"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// Helper functions

func sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func sendError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	errorResponse := map[string]string{
		"error": message,
	}
	_ = json.NewEncoder(w).Encode(errorResponse) // Error response encoding failure is non-critical
}

func getTenantIDFromContext(ctx context.Context) uuid.UUID {
	// This would extract tenant ID from context
	// For now, return a default
	return uuid.MustParse("00000000-0000-0000-0000-000000000001")
}

func getUserIDFromContext(ctx context.Context) string {
	// This would extract user ID from context
	// For now, return a default
	return "system"
}
