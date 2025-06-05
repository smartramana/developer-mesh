package embedding

import (
	"database/sql"
	"fmt"
	"math"
)

// DimensionAdapter handles dimension normalization and projection
type DimensionAdapter struct {
	projectionMatrices map[string]*ProjectionMatrix
	db                 *sql.DB
}

// ProjectionMatrix represents a dimension projection matrix
type ProjectionMatrix struct {
	ID             int       `json:"id" db:"id"`
	FromDimensions int       `json:"from_dimensions" db:"from_dimensions"`
	ToDimensions   int       `json:"to_dimensions" db:"to_dimensions"`
	FromProvider   string    `json:"from_provider" db:"from_provider"`
	FromModel      string    `json:"from_model" db:"from_model"`
	Matrix         []float32 `json:"matrix" db:"matrix"`
	QualityScore   float64   `json:"quality_score" db:"quality_score"`
	IsActive       bool      `json:"is_active" db:"is_active"`
}

// NewDimensionAdapter creates a new dimension adapter
func NewDimensionAdapter() *DimensionAdapter {
	return &DimensionAdapter{
		projectionMatrices: make(map[string]*ProjectionMatrix),
	}
}

// NewDimensionAdapterWithDB creates a new dimension adapter with database support
func NewDimensionAdapterWithDB(db *sql.DB) *DimensionAdapter {
	adapter := &DimensionAdapter{
		projectionMatrices: make(map[string]*ProjectionMatrix),
		db:                 db,
	}
	// Load projection matrices from database if available
	// This would be done asynchronously in production
	return adapter
}

// Normalize normalizes an embedding to the target dimension
func (da *DimensionAdapter) Normalize(embedding []float32, fromDim, toDim int) []float32 {
	if fromDim == toDim {
		return embedding
	}

	if fromDim < toDim {
		// Pad with zeros
		return da.padEmbedding(embedding, toDim)
	}

	// Reduce dimensions
	return da.reduceEmbedding(embedding, toDim)
}

// NormalizeWithProvider normalizes using provider-specific projection if available
func (da *DimensionAdapter) NormalizeWithProvider(embedding []float32, fromDim, toDim int, provider, model string) []float32 {
	// Check for pre-computed projection matrix
	key := fmt.Sprintf("%s:%s:%d:%d", provider, model, fromDim, toDim)
	if matrix, ok := da.projectionMatrices[key]; ok && matrix.IsActive {
		return da.applyProjection(embedding, matrix)
	}

	// Fall back to generic normalization
	return da.Normalize(embedding, fromDim, toDim)
}

// padEmbedding pads an embedding with zeros to reach target dimension
func (da *DimensionAdapter) padEmbedding(embedding []float32, targetDim int) []float32 {
	if len(embedding) >= targetDim {
		return embedding[:targetDim]
	}

	padded := make([]float32, targetDim)
	copy(padded, embedding)
	
	// Initialize padding values with small random values to avoid all zeros
	// In production, use a more sophisticated padding strategy
	for i := len(embedding); i < targetDim; i++ {
		padded[i] = 0.0001 * float32(i%10)
	}

	return padded
}

// reduceEmbedding reduces embedding dimensions using averaging
func (da *DimensionAdapter) reduceEmbedding(embedding []float32, targetDim int) []float32 {
	if len(embedding) <= targetDim {
		return embedding
	}

	reduced := make([]float32, targetDim)
	ratio := float64(len(embedding)) / float64(targetDim)

	for i := 0; i < targetDim; i++ {
		// Average values from the corresponding range in the original embedding
		start := int(float64(i) * ratio)
		end := int(float64(i+1) * ratio)
		if end > len(embedding) {
			end = len(embedding)
		}

		sum := float32(0)
		count := end - start
		for j := start; j < end; j++ {
			sum += embedding[j]
		}
		reduced[i] = sum / float32(count)
	}

	// Normalize magnitude to preserve similarity properties
	return da.normalizeMagnitude(reduced)
}

// applyProjection applies a pre-computed projection matrix
func (da *DimensionAdapter) applyProjection(embedding []float32, matrix *ProjectionMatrix) []float32 {
	// Simple matrix multiplication
	// In production, use optimized linear algebra libraries
	result := make([]float32, matrix.ToDimensions)
	
	for i := 0; i < matrix.ToDimensions; i++ {
		sum := float32(0)
		for j := 0; j < len(embedding); j++ {
			// Matrix is stored in row-major order
			matrixIdx := i*len(embedding) + j
			if matrixIdx < len(matrix.Matrix) {
				sum += embedding[j] * matrix.Matrix[matrixIdx]
			}
		}
		result[i] = sum
	}

	return result
}

// normalizeMagnitude normalizes the magnitude of an embedding
func (da *DimensionAdapter) normalizeMagnitude(embedding []float32) []float32 {
	// Calculate magnitude
	magnitude := float32(0)
	for _, val := range embedding {
		magnitude += val * val
	}
	magnitude = float32(math.Sqrt(float64(magnitude)))

	if magnitude == 0 {
		return embedding
	}

	// Normalize
	normalized := make([]float32, len(embedding))
	for i, val := range embedding {
		normalized[i] = val / magnitude
	}

	return normalized
}

// TrainProjectionMatrix trains a new projection matrix (would be async in production)
func (da *DimensionAdapter) TrainProjectionMatrix(fromDim, toDim int, provider, model string, trainingData [][]float32) error {
	// This would use PCA, autoencoders, or other dimensionality reduction techniques
	// For now, we'll use a simple random projection
	key := fmt.Sprintf("%s:%s:%d:%d", provider, model, fromDim, toDim)
	
	// Create random projection matrix
	matrixSize := toDim * fromDim
	matrix := make([]float32, matrixSize)
	
	// Initialize with random values (in production, use proper initialization)
	for i := range matrix {
		matrix[i] = float32(math.Sin(float64(i))) * 0.1
	}

	da.projectionMatrices[key] = &ProjectionMatrix{
		FromDimensions: fromDim,
		ToDimensions:   toDim,
		FromProvider:   provider,
		FromModel:      model,
		Matrix:         matrix,
		QualityScore:   0.95, // Placeholder quality score
		IsActive:       true,
	}

	// Store in database if available - implementation pending

	return nil
}

// GetProjectionQuality returns the quality score for a projection
func (da *DimensionAdapter) GetProjectionQuality(fromDim, toDim int, provider, model string) float64 {
	key := fmt.Sprintf("%s:%s:%d:%d", provider, model, fromDim, toDim)
	if matrix, ok := da.projectionMatrices[key]; ok {
		return matrix.QualityScore
	}
	
	// Estimate quality based on dimension difference
	ratio := float64(toDim) / float64(fromDim)
	if ratio > 1 {
		return 1.0 // Padding doesn't lose information
	}
	
	// More dimensions lost = lower quality
	return math.Max(0.5, ratio)
}