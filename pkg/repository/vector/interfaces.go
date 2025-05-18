// Package vector provides interfaces and implementations for vector embeddings
package vector

import (
	"context"
	"time"
	
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// Embedding represents a vector embedding stored in the database
type Embedding struct {
	ID           string
	ContextID    string
	ContentIndex int
	Text         string
	Embedding    []float32
	ModelID      string
	CreatedAt    time.Time
	Metadata     map[string]interface{}
}

// Repository defines operations for managing vector embeddings
type Repository interface {
	// Core embedding operations
	StoreEmbedding(ctx context.Context, embedding *Embedding) error
	SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*Embedding, error)
	SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*Embedding, error)
	GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error)
	DeleteContextEmbeddings(ctx context.Context, contextID string) error
	GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*Embedding, error)
	GetSupportedModels(ctx context.Context) ([]string, error)
	// DeleteModelEmbeddings deletes all embeddings for a specific model in a context
	DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error
}

// ToModelVector converts an Embedding to a models.Vector
func ToModelVector(e *Embedding) *models.Vector {
	if e == nil {
		return nil
	}
	
	return &models.Vector{
		ID:        e.ID,
		TenantID:  e.ContextID,
		Content:   e.Text,
		Embedding: e.Embedding,
		Metadata: map[string]interface{}{
			"content_index": e.ContentIndex,
			"model_id":      e.ModelID,
		},
	}
}

// FromModelVector converts a models.Vector to an Embedding
func FromModelVector(v *models.Vector) *Embedding {
	if v == nil {
		return nil
	}
	
	// Extract content index and model ID from metadata if available
	var contentIndex int
	var modelID string
	
	if v.Metadata != nil {
		if ci, ok := v.Metadata["content_index"]; ok {
			if ciFloat, ok := ci.(float64); ok {
				contentIndex = int(ciFloat)
			} else if ciInt, ok := ci.(int); ok {
				contentIndex = ciInt
			}
		}
		
		if mid, ok := v.Metadata["model_id"]; ok {
			if midStr, ok := mid.(string); ok {
				modelID = midStr
			}
		}
	}
	
	return &Embedding{
		ID:           v.ID,
		ContextID:    v.TenantID,
		ContentIndex: contentIndex,
		Text:         v.Content,
		Embedding:    v.Embedding,
		ModelID:      modelID,
		CreatedAt:    v.CreatedAt,
	}
}
