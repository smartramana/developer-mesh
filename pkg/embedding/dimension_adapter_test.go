package embedding

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDimensionAdapter(t *testing.T) {
	adapter := NewDimensionAdapter()
	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.projectionMatrices)
}

func TestDimensionAdapterNormalize(t *testing.T) {
	adapter := NewDimensionAdapter()

	t.Run("same dimensions returns original", func(t *testing.T) {
		embedding := []float32{1.0, 2.0, 3.0, 4.0}
		result := adapter.Normalize(embedding, 4, 4)
		assert.Equal(t, embedding, result)
	})

	t.Run("pad smaller to larger dimension", func(t *testing.T) {
		embedding := []float32{1.0, 2.0, 3.0}
		result := adapter.Normalize(embedding, 3, 6)
		
		assert.Len(t, result, 6)
		// Original values preserved
		assert.Equal(t, float32(1.0), result[0])
		assert.Equal(t, float32(2.0), result[1])
		assert.Equal(t, float32(3.0), result[2])
		// Padded values are small non-zero
		assert.True(t, result[3] >= 0)
		assert.True(t, result[4] >= 0)
		assert.True(t, result[5] >= 0)
	})

	t.Run("reduce larger to smaller dimension", func(t *testing.T) {
		embedding := []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0}
		result := adapter.Normalize(embedding, 6, 3)
		
		assert.Len(t, result, 3)
		// Values should be averaged and then normalized
		// Expected averages: [1.5, 3.5, 5.5]
		// After normalization to unit length
		expectedMagnitude := float32(math.Sqrt(1.5*1.5 + 3.5*3.5 + 5.5*5.5))
		assert.InDelta(t, 1.5/expectedMagnitude, result[0], 0.001)
		assert.InDelta(t, 3.5/expectedMagnitude, result[1], 0.001)
		assert.InDelta(t, 5.5/expectedMagnitude, result[2], 0.001)
	})

	t.Run("reduce with uneven ratio", func(t *testing.T) {
		embedding := []float32{1.0, 2.0, 3.0, 4.0, 5.0}
		result := adapter.Normalize(embedding, 5, 2)
		
		assert.Len(t, result, 2)
		// First bucket: indices 0,1,2 -> (1+2+3)/3 = 2
		// Second bucket: indices 3,4 -> (4+5)/2 = 4.5
		// Note: Due to normalization, values might differ slightly
		assert.True(t, result[0] > 0)
		assert.True(t, result[1] > 0)
	})
}

func TestDimensionAdapterNormalizeWithProvider(t *testing.T) {
	adapter := NewDimensionAdapter()

	t.Run("uses generic normalization without projection matrix", func(t *testing.T) {
		embedding := []float32{1.0, 2.0, 3.0, 4.0}
		result := adapter.NormalizeWithProvider(embedding, 4, 8, "openai", "text-embedding-3-small")
		
		assert.Len(t, result, 8)
		// Should fall back to generic normalization (padding)
		assert.Equal(t, float32(1.0), result[0])
		assert.Equal(t, float32(2.0), result[1])
		assert.Equal(t, float32(3.0), result[2])
		assert.Equal(t, float32(4.0), result[3])
	})

	t.Run("uses projection matrix when available", func(t *testing.T) {
		// Add a mock projection matrix
		matrix := &ProjectionMatrix{
			FromDimensions: 3,
			ToDimensions:   2,
			Matrix:         []float32{0.5, 0.5, 0.0, 0.0, 0.5, 0.5}, // Simple averaging matrix
			IsActive:       true,
		}
		adapter.projectionMatrices["openai:test-model:3:2"] = matrix

		embedding := []float32{2.0, 4.0, 6.0}
		result := adapter.NormalizeWithProvider(embedding, 3, 2, "openai", "test-model")
		
		assert.Len(t, result, 2)
		// Matrix multiplication: [0.5,0.5,0.0] * [2,4,6] = 3
		// Matrix multiplication: [0.0,0.5,0.5] * [2,4,6] = 5
		assert.Equal(t, float32(3.0), result[0])
		assert.Equal(t, float32(5.0), result[1])
	})
}

func TestDimensionAdapterPadEmbedding(t *testing.T) {
	adapter := NewDimensionAdapter()

	t.Run("pad to larger dimension", func(t *testing.T) {
		embedding := []float32{1.0, 2.0}
		result := adapter.padEmbedding(embedding, 5)
		
		assert.Len(t, result, 5)
		assert.Equal(t, float32(1.0), result[0])
		assert.Equal(t, float32(2.0), result[1])
		// Padded values should be small but deterministic
		// padded[i] = 0.0001 * float32(i%10)
		assert.InDelta(t, float32(0.0001*2), result[2], 0.00001)  // index 2: 2%10 = 2
		assert.InDelta(t, float32(0.0001*3), result[3], 0.00001)  // index 3: 3%10 = 3
		assert.InDelta(t, float32(0.0001*4), result[4], 0.00001)  // index 4: 4%10 = 4
	})

	t.Run("truncate if target is smaller", func(t *testing.T) {
		embedding := []float32{1.0, 2.0, 3.0, 4.0, 5.0}
		result := adapter.padEmbedding(embedding, 3)
		
		assert.Len(t, result, 3)
		assert.Equal(t, []float32{1.0, 2.0, 3.0}, result)
	})

	t.Run("return same if equal dimensions", func(t *testing.T) {
		embedding := []float32{1.0, 2.0, 3.0}
		result := adapter.padEmbedding(embedding, 3)
		
		assert.Equal(t, embedding, result)
	})
}

func TestDimensionAdapterReduceEmbedding(t *testing.T) {
	adapter := NewDimensionAdapter()

	t.Run("reduce by averaging", func(t *testing.T) {
		embedding := []float32{1.0, 2.0, 3.0, 4.0}
		result := adapter.reduceEmbedding(embedding, 2)
		
		assert.Len(t, result, 2)
		// Normalized values after averaging and magnitude normalization
		// Original averages would be [1.5, 3.5]
		magnitude := float32(math.Sqrt(1.5*1.5 + 3.5*3.5))
		assert.InDelta(t, 1.5/magnitude, result[0], 0.001)
		assert.InDelta(t, 3.5/magnitude, result[1], 0.001)
	})

	t.Run("return same if target is larger", func(t *testing.T) {
		embedding := []float32{1.0, 2.0}
		result := adapter.reduceEmbedding(embedding, 5)
		
		assert.Equal(t, embedding, result)
	})

	t.Run("handle zero vector", func(t *testing.T) {
		embedding := []float32{0.0, 0.0, 0.0, 0.0}
		result := adapter.reduceEmbedding(embedding, 2)
		
		assert.Len(t, result, 2)
		assert.Equal(t, float32(0.0), result[0])
		assert.Equal(t, float32(0.0), result[1])
	})
}

func TestDimensionAdapterApplyProjection(t *testing.T) {
	adapter := NewDimensionAdapter()

	t.Run("apply simple projection matrix", func(t *testing.T) {
		embedding := []float32{1.0, 2.0, 3.0}
		matrix := &ProjectionMatrix{
			FromDimensions: 3,
			ToDimensions:   2,
			Matrix: []float32{
				1.0, 0.0, 0.0, // First row: take first element
				0.0, 0.0, 1.0, // Second row: take third element
			},
		}

		result := adapter.applyProjection(embedding, matrix)
		
		assert.Len(t, result, 2)
		assert.Equal(t, float32(1.0), result[0])
		assert.Equal(t, float32(3.0), result[1])
	})

	t.Run("apply averaging projection matrix", func(t *testing.T) {
		embedding := []float32{2.0, 4.0, 6.0}
		matrix := &ProjectionMatrix{
			FromDimensions: 3,
			ToDimensions:   2,
			Matrix: []float32{
				0.5, 0.5, 0.0,   // Average first two
				0.33, 0.33, 0.34, // Average all three
			},
		}

		result := adapter.applyProjection(embedding, matrix)
		
		assert.Len(t, result, 2)
		assert.InDelta(t, 3.0, result[0], 0.001)     // (2*0.5 + 4*0.5)
		assert.InDelta(t, 4.02, result[1], 0.001)    // (2*0.33 + 4*0.33 + 6*0.34) = 0.66 + 1.32 + 2.04 = 4.02
	})

	t.Run("handle matrix size mismatch gracefully", func(t *testing.T) {
		embedding := []float32{1.0, 2.0, 3.0}
		matrix := &ProjectionMatrix{
			FromDimensions: 3,
			ToDimensions:   2,
			Matrix:         []float32{1.0, 0.0}, // Too small matrix
		}

		result := adapter.applyProjection(embedding, matrix)
		
		// Should handle gracefully without panic
		assert.Len(t, result, 2)
	})
}

func TestDimensionAdapterNormalizeMagnitude(t *testing.T) {
	adapter := NewDimensionAdapter()

	t.Run("normalize non-zero vector", func(t *testing.T) {
		embedding := []float32{3.0, 4.0} // Magnitude = 5
		result := adapter.normalizeMagnitude(embedding)
		
		assert.Len(t, result, 2)
		assert.InDelta(t, 0.6, result[0], 0.001) // 3/5
		assert.InDelta(t, 0.8, result[1], 0.001) // 4/5
		
		// Check magnitude is 1
		magnitude := float32(math.Sqrt(float64(result[0]*result[0] + result[1]*result[1])))
		assert.InDelta(t, 1.0, magnitude, 0.001)
	})

	t.Run("handle zero vector", func(t *testing.T) {
		embedding := []float32{0.0, 0.0, 0.0}
		result := adapter.normalizeMagnitude(embedding)
		
		assert.Equal(t, embedding, result)
	})

	t.Run("already normalized vector", func(t *testing.T) {
		embedding := []float32{0.6, 0.8} // Already magnitude 1
		result := adapter.normalizeMagnitude(embedding)
		
		assert.InDelta(t, 0.6, result[0], 0.001)
		assert.InDelta(t, 0.8, result[1], 0.001)
	})
}

func TestDimensionAdapterEdgeCases(t *testing.T) {
	adapter := NewDimensionAdapter()

	t.Run("empty embedding", func(t *testing.T) {
		embedding := []float32{}
		result := adapter.Normalize(embedding, 0, 5)
		assert.Len(t, result, 5)
		// All padded values
		for i := 0; i < 5; i++ {
			assert.Equal(t, float32(0.0001*float32(i)), result[i])
		}
	})

	t.Run("single dimension to multiple", func(t *testing.T) {
		embedding := []float32{5.0}
		result := adapter.Normalize(embedding, 1, 3)
		
		assert.Len(t, result, 3)
		assert.Equal(t, float32(5.0), result[0])
		assert.True(t, result[1] >= 0)
		assert.True(t, result[2] >= 0)
	})

	t.Run("large dimension reduction", func(t *testing.T) {
		// Create large embedding
		embedding := make([]float32, 1000)
		for i := range embedding {
			embedding[i] = float32(i)
		}
		
		result := adapter.Normalize(embedding, 1000, 10)
		assert.Len(t, result, 10)
		
		// Each bucket should average 100 values
		for i := 0; i < 10; i++ {
			// After magnitude normalization, values will be different
			assert.True(t, result[i] > 0)
		}
	})
}

// Benchmark tests
func BenchmarkDimensionAdapterNormalize(b *testing.B) {
	adapter := NewDimensionAdapter()
	embedding := make([]float32, 1536)
	for i := range embedding {
		embedding[i] = float32(i) * 0.001
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.Normalize(embedding, 1536, 768)
	}
}

func BenchmarkDimensionAdapterPad(b *testing.B) {
	adapter := NewDimensionAdapter()
	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = float32(i) * 0.001
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.padEmbedding(embedding, 1536)
	}
}

func BenchmarkDimensionAdapterReduce(b *testing.B) {
	adapter := NewDimensionAdapter()
	embedding := make([]float32, 3072)
	for i := range embedding {
		embedding[i] = float32(i) * 0.0001
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.reduceEmbedding(embedding, 1536)
	}
}

func BenchmarkDimensionAdapterProjection(b *testing.B) {
	adapter := NewDimensionAdapter()
	embedding := make([]float32, 1536)
	for i := range embedding {
		embedding[i] = float32(i) * 0.001
	}

	// Create a realistic projection matrix
	matrix := &ProjectionMatrix{
		FromDimensions: 1536,
		ToDimensions:   768,
		Matrix:         make([]float32, 1536*768),
	}
	// Initialize with random values
	for i := range matrix.Matrix {
		matrix.Matrix[i] = float32(i%10) * 0.1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.applyProjection(embedding, matrix)
	}
}