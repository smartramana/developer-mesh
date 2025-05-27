package models

import (
	"time"
)

// Vector represents a vector embedding stored in the database
type Vector struct {
	ID        string         `json:"id" db:"id"`
	TenantID  string         `json:"tenant_id" db:"tenant_id"`
	Content   string         `json:"content" db:"content"`
	Embedding []float32      `json:"embedding" db:"embedding"`
	Metadata  map[string]any `json:"metadata" db:"metadata"`
	CreatedAt time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt time.Time      `json:"updated_at" db:"updated_at"`
}

// NewVector creates a new vector embedding
func NewVector(tenantID, content string, embedding []float32, metadata map[string]any) *Vector {
	now := time.Now()
	return &Vector{
		TenantID:  tenantID,
		Content:   content,
		Embedding: embedding,
		Metadata:  metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
