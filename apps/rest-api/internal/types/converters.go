// Package types provides shared types for the REST API internal packages
package types

import (
	"time"

	pkgrepo "github.com/developer-mesh/developer-mesh/pkg/repository"
)

// ConvertToPkgEmbedding converts an internal embedding to pkg repository embedding
func ConvertToPkgEmbedding(internal *Embedding) *pkgrepo.Embedding {
	if internal == nil {
		return nil
	}
	return &pkgrepo.Embedding{
		ID:           internal.ID,
		ContextID:    internal.ContextID,
		ContentIndex: internal.ContentIndex,
		Text:         internal.Text,
		Embedding:    internal.Embedding,
		ModelID:      internal.ModelID,
		// Metadata field not used in pkg.Embedding
	}
}

// ConvertToInternalEmbedding converts a pkg repository embedding to internal one
func ConvertToInternalEmbedding(pkgEmb *pkgrepo.Embedding) *Embedding {
	if pkgEmb == nil {
		return nil
	}
	return &Embedding{
		ID:           pkgEmb.ID,
		ContextID:    pkgEmb.ContextID,
		ContentIndex: pkgEmb.ContentIndex,
		Text:         pkgEmb.Text,
		Embedding:    pkgEmb.Embedding,
		ModelID:      pkgEmb.ModelID,
		// No Metadata mapping
	}
}

// ConvertToPkgEmbeddings converts a slice of internal embeddings to pkg repository embeddings
func ConvertToPkgEmbeddings(internalEmbs []*Embedding) []*pkgrepo.Embedding {
	if internalEmbs == nil {
		return nil
	}
	result := make([]*pkgrepo.Embedding, len(internalEmbs))
	for i, emb := range internalEmbs {
		result[i] = ConvertToPkgEmbedding(emb)
	}
	return result
}

// ConvertToInternalEmbeddings converts a slice of pkg repository embeddings to internal ones
func ConvertToInternalEmbeddings(pkgEmbs []*pkgrepo.Embedding) []*Embedding {
	if pkgEmbs == nil {
		return nil
	}
	result := make([]*Embedding, len(pkgEmbs))
	for i, emb := range pkgEmbs {
		result[i] = ConvertToInternalEmbedding(emb)
	}
	return result
}

// AgentFilter defines filter criteria for agent operations
type AgentFilter struct {
	ID       string `json:"id,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
	Name     string `json:"name,omitempty"`
	ModelID  string `json:"model_id,omitempty"`
}

// ModelFilter defines filter criteria for model operations
type ModelFilter struct {
	ID       string `json:"id,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
	Name     string `json:"name,omitempty"`
}

// ContextFilter defines filter criteria for context operations
type ContextFilter struct {
	ID          string    `json:"id,omitempty"`
	TenantID    string    `json:"tenant_id,omitempty"`
	Name        string    `json:"name,omitempty"`
	ContentType string    `json:"content_type,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// PaginationInfo contains pagination metadata
type PaginationInfo struct {
	Total      int  `json:"total"`
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// VectorSearchQuery defines parameters for vector search operations
type VectorSearchQuery struct {
	QueryVector         []float32      `json:"query_vector,omitempty"`
	Query               string         `json:"query,omitempty"`
	ContextID           string         `json:"context_id,omitempty"`
	ModelID             string         `json:"model_id,omitempty"`
	Limit               int            `json:"limit,omitempty"`
	SimilarityThreshold float64        `json:"similarity_threshold,omitempty"`
	Filters             map[string]any `json:"filters,omitempty"`
}

// VectorSearchResult represents a vector search result
type VectorSearchResult struct {
	ID         string         `json:"id"`
	Content    string         `json:"content"`
	Similarity float64        `json:"similarity"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	ContextID  string         `json:"context_id,omitempty"`
	ModelID    string         `json:"model_id,omitempty"`
}

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Details   map[string]string `json:"details,omitempty"`
}

// ConvertFilterToMap converts typed filters to generic map for repository layer
func ConvertAgentFilterToMap(filter *AgentFilter) map[string]any {
	if filter == nil {
		return nil
	}

	result := make(map[string]any)
	if filter.ID != "" {
		result["id"] = filter.ID
	}
	if filter.TenantID != "" {
		result["tenant_id"] = filter.TenantID
	}
	if filter.Name != "" {
		result["name"] = filter.Name
	}
	if filter.ModelID != "" {
		result["model_id"] = filter.ModelID
	}
	return result
}

// ConvertModelFilterToMap converts typed model filter to generic map
func ConvertModelFilterToMap(filter *ModelFilter) map[string]any {
	if filter == nil {
		return nil
	}

	result := make(map[string]any)
	if filter.ID != "" {
		result["id"] = filter.ID
	}
	if filter.TenantID != "" {
		result["tenant_id"] = filter.TenantID
	}
	if filter.Name != "" {
		result["name"] = filter.Name
	}
	return result
}

// ConvertContextFilterToMap converts typed context filter to generic map
func ConvertContextFilterToMap(filter *ContextFilter) map[string]any {
	if filter == nil {
		return nil
	}

	result := make(map[string]any)
	if filter.ID != "" {
		result["id"] = filter.ID
	}
	if filter.TenantID != "" {
		result["tenant_id"] = filter.TenantID
	}
	if filter.Name != "" {
		result["name"] = filter.Name
	}
	if filter.ContentType != "" {
		result["content_type"] = filter.ContentType
	}
	return result
}
