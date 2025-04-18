package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	
	"github.com/jmoiron/sqlx"
)

// EmbeddingRepository handles vector storage and retrieval
type EmbeddingRepository struct {
	db *sqlx.DB
}

// NewEmbeddingRepository creates a new embedding repository
func NewEmbeddingRepository(db *sqlx.DB) *EmbeddingRepository {
	return &EmbeddingRepository{
		db: db,
	}
}

// Embedding represents a vector embedding of text
type Embedding struct {
	ID              string    `db:"id" json:"id"`
	ContextID       string    `db:"context_id" json:"context_id"`
	ContentIndex    int       `db:"content_index" json:"content_index"`
	Text            string    `db:"text" json:"text"`
	Embedding       []float32 `db:"embedding" json:"embedding"`
	VectorDimensions int      `db:"vector_dimensions" json:"vector_dimensions"`
	ModelID         string    `db:"model_id" json:"model_id"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
}

// StoreEmbedding stores a vector embedding for a context item
func (r *EmbeddingRepository) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	query := `
		INSERT INTO mcp.embeddings (context_id, content_index, text, embedding, vector_dimensions, model_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	
	// Set the vector dimensions
	embedding.VectorDimensions = len(embedding.Embedding)
	
	// Convert the Go slice to a PostgreSQL vector
	vectorStr := fmt.Sprintf("[%s]", strings.Join(floatSliceToStrings(embedding.Embedding), ","))
	
	var id string
	err := r.db.GetContext(ctx, &id, query, 
		embedding.ContextID, 
		embedding.ContentIndex,
		embedding.Text,
		vectorStr,
		embedding.VectorDimensions,
		embedding.ModelID,
	)
	
	if err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}
	
	embedding.ID = id
	return nil
}

// SearchEmbeddings searches for similar embeddings using vector similarity
func (r *EmbeddingRepository) SearchEmbeddings(
	ctx context.Context, 
	queryVector []float32, 
	contextID string, 
	limit int,
) ([]Embedding, error) {
	// Get the dimensions of the query vector
	dimensions := len(queryVector)
	
	// Search only vectors with matching dimensions
	query := `
		SELECT id, context_id, content_index, text, embedding::text as embedding, vector_dimensions, model_id, created_at
		FROM mcp.embeddings
		WHERE context_id = $1 AND vector_dimensions = $2
		ORDER BY embedding <-> $3
		LIMIT $4
	`
	
	// Convert the query vector to a PostgreSQL vector
	vectorStr := fmt.Sprintf("[%s]", strings.Join(floatSliceToStrings(queryVector), ","))
	
	rows, err := r.db.QueryxContext(ctx, query, contextID, dimensions, vectorStr, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer rows.Close()
	
	var embeddings []Embedding
	for rows.Next() {
		var emb Embedding
		var embStr string
		
		// Create a map to hold the raw data
		data := make(map[string]interface{})
		
		if err := rows.MapScan(data); err != nil {
			return nil, fmt.Errorf("failed to scan embedding: %w", err)
		}
		
		// Manually convert each field with careful type checking
		if v, ok := data["id"]; ok {
			switch val := v.(type) {
			case []byte:
				emb.ID = string(val)
			case string:
				emb.ID = val
			}
		}
		
		if v, ok := data["context_id"]; ok {
			switch val := v.(type) {
			case []byte:
				emb.ContextID = string(val)
			case string:
				emb.ContextID = val
			}
		}
		
		if v, ok := data["content_index"]; ok {
			switch val := v.(type) {
			case int64:
				emb.ContentIndex = int(val)
			case []byte:
				num, _ := strconv.Atoi(string(val))
				emb.ContentIndex = num
			case string:
				num, _ := strconv.Atoi(val)
				emb.ContentIndex = num
			case int:
				emb.ContentIndex = val
			}
		}
		
		if v, ok := data["text"]; ok {
			switch val := v.(type) {
			case []byte:
				emb.Text = string(val)
			case string:
				emb.Text = val
			}
		}
		
		if v, ok := data["embedding"]; ok {
			switch val := v.(type) {
			case []byte:
				embStr = string(val)
			case string:
				embStr = val
			default:
				// Skip embedding if type is unexpected
				continue
			}
			
			// Parse the embedding string - remove brackets and split by commas
			embStr = strings.TrimPrefix(embStr, "{")
			embStr = strings.TrimSuffix(embStr, "}")
			components := strings.Split(embStr, ",")
			
			// Convert strings to float32
			embedding := make([]float32, len(components))
			for i, comp := range components {
				val, err := strconv.ParseFloat(strings.TrimSpace(comp), 32)
				if err != nil {
					continue // Skip invalid floats
				}
				embedding[i] = float32(val)
			}
			emb.Embedding = embedding
		}
		
		if v, ok := data["vector_dimensions"]; ok {
			switch val := v.(type) {
			case int64:
				emb.VectorDimensions = int(val)
			case []byte:
				num, _ := strconv.Atoi(string(val))
				emb.VectorDimensions = num
			case string:
				num, _ := strconv.Atoi(val)
				emb.VectorDimensions = num
			case int:
				emb.VectorDimensions = val
			}
		}
		
		if v, ok := data["vector_dimensions"]; ok {
			switch val := v.(type) {
			case int64:
				emb.VectorDimensions = int(val)
			case []byte:
				num, _ := strconv.Atoi(string(val))
				emb.VectorDimensions = num
			case string:
				num, _ := strconv.Atoi(val)
				emb.VectorDimensions = num
			case int:
				emb.VectorDimensions = val
			}
		}
		
		if v, ok := data["model_id"]; ok {
			switch val := v.(type) {
			case []byte:
				emb.ModelID = string(val)
			case string:
				emb.ModelID = val
			}
		}
		
		if v, ok := data["created_at"]; ok {
			switch val := v.(type) {
			case time.Time:
				emb.CreatedAt = val
			}
		}
		
		embeddings = append(embeddings, emb)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	return embeddings, nil
}

// GetContextEmbeddings retrieves all embeddings for a context
func (r *EmbeddingRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]Embedding, error) {
	query := `
		SELECT id, context_id, content_index, text, embedding::text as embedding, vector_dimensions, model_id, created_at
		FROM mcp.embeddings
		WHERE context_id = $1
	`
	
	rows, err := r.db.QueryxContext(ctx, query, contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context embeddings: %w", err)
	}
	defer rows.Close()
	
	var embeddings []Embedding
	for rows.Next() {
		var emb Embedding
		var embStr string
		
		// Create a map to hold the raw data
		data := make(map[string]interface{})
		
		if err := rows.MapScan(data); err != nil {
			return nil, fmt.Errorf("failed to scan embedding: %w", err)
		}
		
		// Manually convert each field
		if v, ok := data["id"]; ok {
			switch val := v.(type) {
			case []byte:
				emb.ID = string(val)
			case string:
				emb.ID = val
			}
		}
		
		if v, ok := data["context_id"]; ok {
			switch val := v.(type) {
			case []byte:
				emb.ContextID = string(val)
			case string:
				emb.ContextID = val
			}
		}
		
		if v, ok := data["content_index"]; ok {
			switch val := v.(type) {
			case int64:
				emb.ContentIndex = int(val)
			case []byte:
				num, _ := strconv.Atoi(string(val))
				emb.ContentIndex = num
			case string:
				num, _ := strconv.Atoi(val)
				emb.ContentIndex = num
			case int:
				emb.ContentIndex = val
			}
		}
		
		if v, ok := data["text"]; ok {
			switch val := v.(type) {
			case []byte:
				emb.Text = string(val)
			case string:
				emb.Text = val
			}
		}
		
		if v, ok := data["embedding"]; ok {
			switch val := v.(type) {
			case []byte:
				embStr = string(val)
			case string:
				embStr = val
			default:
				// Skip embedding if type is unexpected
				continue
			}
			
			// Parse the embedding string - remove brackets and split by commas
			embStr = strings.TrimPrefix(embStr, "{")
			embStr = strings.TrimSuffix(embStr, "}")
			components := strings.Split(embStr, ",")
			
			// Convert strings to float32
			embedding := make([]float32, len(components))
			for i, comp := range components {
				val, err := strconv.ParseFloat(strings.TrimSpace(comp), 32)
				if err != nil {
					continue // Skip invalid floats
				}
				embedding[i] = float32(val)
			}
			emb.Embedding = embedding
		}
		
		if v, ok := data["model_id"]; ok {
			switch val := v.(type) {
			case []byte:
				emb.ModelID = string(val)
			case string:
				emb.ModelID = val
			}
		}
		
		if v, ok := data["created_at"]; ok {
			switch val := v.(type) {
			case time.Time:
				emb.CreatedAt = val
			}
		}
		
		embeddings = append(embeddings, emb)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	return embeddings, nil
}

// SearchEmbeddingsByModelAndDimensions searches for similar embeddings with specific model and dimensions
func (r *EmbeddingRepository) SearchEmbeddingsByModelAndDimensions(
	ctx context.Context, 
	queryVector []float32, 
	contextID string,
	modelID string,
	limit int,
) ([]Embedding, error) {
	// Get the dimensions of the query vector
	dimensions := len(queryVector)
	
	// Search only vectors with matching dimensions and model
	query := `
		SELECT id, context_id, content_index, text, embedding::text as embedding, vector_dimensions, model_id, created_at
		FROM mcp.embeddings
		WHERE context_id = $1 AND vector_dimensions = $2 AND model_id = $3
		ORDER BY embedding <-> $4
		LIMIT $5
	`
	
	// Convert the query vector to a PostgreSQL vector
	vectorStr := fmt.Sprintf("[%s]", strings.Join(floatSliceToStrings(queryVector), ","))
	
	rows, err := r.db.QueryxContext(ctx, query, contextID, dimensions, modelID, vectorStr, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer rows.Close()
	
	var embeddings []Embedding
	for rows.Next() {
		var emb Embedding
		var embStr string
		
		// Create a map to hold the raw data
		data := make(map[string]interface{})
		
		if err := rows.MapScan(data); err != nil {
			return nil, fmt.Errorf("failed to scan embedding: %w", err)
		}
		
		// Manually convert each field with careful type checking
		if v, ok := data["id"]; ok {
			switch val := v.(type) {
			case []byte:
				emb.ID = string(val)
			case string:
				emb.ID = val
			}
		}
		
		if v, ok := data["context_id"]; ok {
			switch val := v.(type) {
			case []byte:
				emb.ContextID = string(val)
			case string:
				emb.ContextID = val
			}
		}
		
		if v, ok := data["content_index"]; ok {
			switch val := v.(type) {
			case int64:
				emb.ContentIndex = int(val)
			case []byte:
				num, _ := strconv.Atoi(string(val))
				emb.ContentIndex = num
			case string:
				num, _ := strconv.Atoi(val)
				emb.ContentIndex = num
			case int:
				emb.ContentIndex = val
			}
		}
		
		if v, ok := data["text"]; ok {
			switch val := v.(type) {
			case []byte:
				emb.Text = string(val)
			case string:
				emb.Text = val
			}
		}
		
		if v, ok := data["embedding"]; ok {
			switch val := v.(type) {
			case []byte:
				embStr = string(val)
			case string:
				embStr = val
			default:
				// Skip embedding if type is unexpected
				continue
			}
			
			// Parse the embedding string - remove brackets and split by commas
			embStr = strings.TrimPrefix(embStr, "{")
			embStr = strings.TrimSuffix(embStr, "}")
			components := strings.Split(embStr, ",")
			
			// Convert strings to float32
			embedding := make([]float32, len(components))
			for i, comp := range components {
				val, err := strconv.ParseFloat(strings.TrimSpace(comp), 32)
				if err != nil {
					continue // Skip invalid floats
				}
				embedding[i] = float32(val)
			}
			emb.Embedding = embedding
		}
		
		if v, ok := data["vector_dimensions"]; ok {
			switch val := v.(type) {
			case int64:
				emb.VectorDimensions = int(val)
			case []byte:
				num, _ := strconv.Atoi(string(val))
				emb.VectorDimensions = num
			case string:
				num, _ := strconv.Atoi(val)
				emb.VectorDimensions = num
			case int:
				emb.VectorDimensions = val
			}
		}
		
		if v, ok := data["model_id"]; ok {
			switch val := v.(type) {
			case []byte:
				emb.ModelID = string(val)
			case string:
				emb.ModelID = val
			}
		}
		
		if v, ok := data["created_at"]; ok {
			switch val := v.(type) {
			case time.Time:
				emb.CreatedAt = val
			}
		}
		
		embeddings = append(embeddings, emb)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	return embeddings, nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (r *EmbeddingRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	query := `
		DELETE FROM mcp.embeddings
		WHERE context_id = $1
	`
	
	_, err := r.db.ExecContext(ctx, query, contextID)
	if err != nil {
		return fmt.Errorf("failed to delete context embeddings: %w", err)
	}
	
	return nil
}

// Helper function to convert float slice to string slice
func floatSliceToStrings(floats []float32) []string {
	strings := make([]string, len(floats))
	for i, f := range floats {
		strings[i] = fmt.Sprintf("%f", f)
	}
	return strings
}

// Helper function to convert PG vector string to float32 slice
func parseVectorString(vectorStr string) []float32 {
	// Remove brackets and split by commas
	vectorStr = strings.TrimPrefix(vectorStr, "{")
	vectorStr = strings.TrimSuffix(vectorStr, "}")
	components := strings.Split(vectorStr, ",")
	
	// Convert strings to float32
	result := make([]float32, 0, len(components))
	for _, comp := range components {
		comp = strings.TrimSpace(comp)
		if comp == "" {
			continue
		}
		
		val, err := strconv.ParseFloat(comp, 32)
		if err != nil {
			continue // Skip invalid floats
		}
		result = append(result, float32(val))
	}
	
	return result
}
