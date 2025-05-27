// Package repository provides compatibility with the old repository structure
// This file exists as a bridge for backwards compatibility
package repository

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// VectorRepository defines operations for managing vector embeddings
// This is separate from VectorAPIRepository because they serve different purposes
type VectorRepository interface {
	StoreVectors(ctx context.Context, vectors []*models.Vector) error
	FindSimilar(ctx context.Context, vector []float32, limit int, filter map[string]any) ([]*models.Vector, error)
	DeleteVectors(ctx context.Context, ids []string) error
	GetVector(ctx context.Context, id string) (*models.Vector, error)
}

// The following methods are defined in vector_bridge.go to support legacy code:
// - GetEmbedding
// - DeleteEmbedding
// - DeleteModelEmbeddings
// They are not part of the base VectorAPIRepository interface in interfaces.go
