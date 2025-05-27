package models

import "time"

// Database query options
type QueryOptions struct {
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	SortBy string `json:"sortBy"`
	Order  string `json:"order"`
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

// VectorFilter defines filter criteria for vector operations
type VectorFilter struct {
	ID        string `json:"id,omitempty"`
	TenantID  string `json:"tenant_id,omitempty"`
	ContextID string `json:"context_id,omitempty"`
	ModelID   string `json:"model_id,omitempty"`
}

// ToolFilter defines filter criteria for tool operations
type ToolFilter struct {
	ID       string `json:"id,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
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

// GitHub query types are defined in github.go
