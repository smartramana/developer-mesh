package common

import (
	"errors"
)

// Error types for common operations
var (
	// ErrVectorDimensionMismatch is returned when comparing vectors with different dimensions
	ErrVectorDimensionMismatch = errors.New("vector dimensions do not match")
	
	// ErrEmptyVector is returned when an operation is attempted on an empty vector
	ErrEmptyVector = errors.New("vector is empty")
	
	// ErrUnsupportedSimilarityMethod is returned when an unsupported similarity method is requested
	ErrUnsupportedSimilarityMethod = errors.New("unsupported similarity method")
	
	// ErrModelMismatch is returned when comparing embeddings from different models
	ErrModelMismatch = errors.New("embeddings from different models cannot be compared")
	
	// ErrInvalidEmbedding is returned when an embedding is invalid
	ErrInvalidEmbedding = errors.New("invalid embedding format")
	
	// ErrPgVectorNotAvailable is returned when the pgvector extension is not available
	ErrPgVectorNotAvailable = errors.New("pgvector extension is not available")
	
	// ErrContextNotFound is returned when a context is not found
	ErrContextNotFound = errors.New("context not found")
)
