package common

import (
	"math"
)

// NormalizeVectorL2 normalizes a vector using L2 normalization (Euclidean norm)
func NormalizeVectorL2(vector []float32) []float32 {
	// Calculate the L2 norm (Euclidean norm)
	var sum float32
	for _, v := range vector {
		sum += v * v
	}
	norm := float32(math.Sqrt(float64(sum)))
	
	// Avoid division by zero
	if norm < 1e-10 {
		return vector
	}
	
	// Normalize the vector
	normalized := make([]float32, len(vector))
	for i, v := range vector {
		normalized[i] = v / norm
	}
	
	return normalized
}

// DotProduct calculates the dot product of two vectors
func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	
	return sum
}

// CosineDistance calculates the cosine distance between two vectors
func CosineDistance(a, b []float32) float32 {
	// Normalize vectors
	normA := NormalizeVectorL2(a)
	normB := NormalizeVectorL2(b)
	
	// Calculate dot product of normalized vectors
	// Cosine similarity = dot product of normalized vectors
	similarity := DotProduct(normA, normB)
	
	// Convert similarity to distance: distance = 1 - similarity
	return 1 - similarity
}

// EuclideanDistance calculates the Euclidean distance between two vectors
func EuclideanDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return float32(math.MaxFloat32)
	}
	
	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	
	return float32(math.Sqrt(float64(sum)))
}
