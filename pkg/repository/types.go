// Package repository defines common types for repository operations
package repository

import (
	"context"
)

// Filter defines a generic filter type for repository operations
// This is used across all repository implementations
type Filter map[string]interface{}

// Repository defines a generic repository interface for CRUD operations
// The type parameter T represents the entity type managed by the repository
type Repository[T any] interface {
	// Create stores a new entity
	Create(ctx context.Context, entity *T) error

	// Get retrieves an entity by its ID
	Get(ctx context.Context, id string) (*T, error)

	// List retrieves entities matching the provided filter
	List(ctx context.Context, filter Filter) ([]*T, error)

	// Update modifies an existing entity
	Update(ctx context.Context, entity *T) error

	// Delete removes an entity by its ID
	Delete(ctx context.Context, id string) error
}
