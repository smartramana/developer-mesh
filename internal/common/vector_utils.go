package common

import (
	"math"
)

// NormalizeVectorL2 normalizes a vector using L2 (Euclidean) normalization
// This is necessary for cosine similarity operations
func NormalizeVectorL2(vector []float32) []float32 {
	if len(vector) == 0 {
		return vector
	}
	
	// Calculate the L2 norm (Euclidean length)
	var sum float64
	for _, v := range vector {
		sum += float64(v * v)
	}
	
	// If the vector is all zeros, return it as is
	if sum == 0 {
		return vector
	}
	
	length := math.Sqrt(sum)
	
	// Normalize each component
	normalized := make([]float32, len(vector))
	for i, v := range vector {
		normalized[i] = float32(float64(v) / length)
	}
	
	return normalized
}

// CalculateVectorSimilarity calculates the similarity between two vectors
// using the specified method (cosine, dot, or euclidean)
func CalculateVectorSimilarity(vec1, vec2 []float32, method string) (float32, error) {
	if len(vec1) != len(vec2) {
		return 0, ErrVectorDimensionMismatch
	}
	
	if len(vec1) == 0 {
		return 0, ErrEmptyVector
	}
	
	switch method {
	case "cosine":
		return cosineSimilarity(vec1, vec2), nil
	case "dot":
		return dotProduct(vec1, vec2), nil
	case "euclidean":
		return euclideanDistance(vec1, vec2), nil
	default:
		return 0, ErrUnsupportedSimilarityMethod
	}
}

// cosineSimilarity calculates the cosine similarity between two vectors
// Assumes the vectors are already normalized for efficiency
func cosineSimilarity(vec1, vec2 []float32) float32 {
	return dotProduct(vec1, vec2)
}

// dotProduct calculates the dot product between two vectors
func dotProduct(vec1, vec2 []float32) float32 {
	var sum float32
	for i := 0; i < len(vec1); i++ {
		sum += vec1[i] * vec2[i]
	}
	return sum
}

// euclideanDistance calculates the Euclidean distance between two vectors
// Returns a similarity score (1.0 for identical vectors, decreasing as distance increases)
func euclideanDistance(vec1, vec2 []float32) float32 {
	var sumSquares float32
	for i := 0; i < len(vec1); i++ {
		diff := vec1[i] - vec2[i]
		sumSquares += diff * diff
	}
	
	// Convert to a similarity score (1.0 for identical vectors)
	// Using a negative exponential of the distance
	distance := float32(math.Sqrt(float64(sumSquares)))
	return float32(math.Exp(-float64(distance)))
}
