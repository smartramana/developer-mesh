package embedding

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// InsertEmbedding inserts a new embedding with automatic padding
func (r *Repository) InsertEmbedding(ctx context.Context, req InsertRequest) (uuid.UUID, error) {
	var id uuid.UUID

	err := r.db.QueryRowContext(ctx, `
        SELECT mcp.insert_embedding($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `,
		req.ContextID,
		req.Content,
		pq.Array(req.Embedding),
		req.ModelName,
		req.TenantID,
		req.Metadata,
		req.ContentIndex,
		req.ChunkIndex,
		req.ConfiguredDimensions,
	).Scan(&id)

	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert embedding: %w", err)
	}

	return id, nil
}

// SearchEmbeddings performs similarity search with optional metadata filtering
func (r *Repository) SearchEmbeddings(ctx context.Context, req SearchRequest) ([]EmbeddingSearchResult, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT * FROM mcp.search_embeddings($1, $2, $3, $4, $5, $6, $7)
    `,
		pq.Array(req.QueryEmbedding),
		req.ModelName,
		req.TenantID,
		req.ContextID, // can be nil
		req.Limit,
		req.Threshold,
		req.MetadataFilter, // JSONB filter
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Embedding repository - log but don't fail
			_ = err
		}
	}()

	var results []EmbeddingSearchResult
	for rows.Next() {
		var r EmbeddingSearchResult
		if err := rows.Scan(&r.ID, &r.ContextID, &r.Content, &r.Similarity, &r.Metadata, &r.ModelProvider); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	return results, nil
}

// GetAvailableModels retrieves available embedding models
func (r *Repository) GetAvailableModels(ctx context.Context, filter ModelFilter) ([]Model, error) {
	query := `
        SELECT provider, model_name, model_version, dimensions, max_tokens,
               model_type, supports_dimensionality_reduction, min_dimensions, is_active
        FROM mcp.get_available_models($1, $2)
    `

	rows, err := r.db.QueryContext(ctx, query, filter.Provider, filter.ModelType)
	if err != nil {
		return nil, fmt.Errorf("failed to get available models: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Embedding repository - log but don't fail
			_ = err
		}
	}()

	var models []Model
	for rows.Next() {
		var m Model
		err := rows.Scan(
			&m.Provider,
			&m.ModelName,
			&m.ModelVersion,
			&m.Dimensions,
			&m.MaxTokens,
			&m.ModelType,
			&m.SupportsDimensionalityReduction,
			&m.MinDimensions,
			&m.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan model: %w", err)
		}
		models = append(models, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating models: %w", err)
	}

	return models, nil
}

// GetModelByName retrieves a model by name
func (r *Repository) GetModelByName(ctx context.Context, modelName string) (*Model, error) {
	var model Model

	err := r.db.QueryRowContext(ctx, `
        SELECT id, provider, model_name, model_version, dimensions,
               max_tokens, supports_binary, supports_dimensionality_reduction,
               min_dimensions, cost_per_million_tokens, model_id, model_type,
               is_active, capabilities, created_at
        FROM mcp.embedding_models
        WHERE model_name = $1 AND is_active = true
        LIMIT 1
    `, modelName).Scan(
		&model.ID,
		&model.Provider,
		&model.ModelName,
		&model.ModelVersion,
		&model.Dimensions,
		&model.MaxTokens,
		&model.SupportsBinary,
		&model.SupportsDimensionalityReduction,
		&model.MinDimensions,
		&model.CostPerMillionTokens,
		&model.ModelID,
		&model.ModelType,
		&model.IsActive,
		&model.Capabilities,
		&model.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("model not found: %s", modelName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	return &model, nil
}

// GetEmbeddingsByContext retrieves all embeddings for a context
func (r *Repository) GetEmbeddingsByContext(ctx context.Context, contextID, tenantID uuid.UUID) ([]Embedding, error) {
	query := `
        SELECT id, context_id, content_index, chunk_index, content,
               content_hash, content_tokens, model_id, model_provider,
               model_name, model_dimensions, configured_dimensions,
               processing_time_ms, embedding_created_at, magnitude, 
               tenant_id, metadata, created_at, updated_at
        FROM mcp.embeddings
        WHERE context_id = $1 AND tenant_id = $2
        ORDER BY content_index, chunk_index
    `

	rows, err := r.db.QueryContext(ctx, query, contextID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query embeddings: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Embedding repository - log but don't fail
			_ = err
		}
	}()

	var embeddings []Embedding
	for rows.Next() {
		var e Embedding
		err := rows.Scan(
			&e.ID,
			&e.ContextID,
			&e.ContentIndex,
			&e.ChunkIndex,
			&e.Content,
			&e.ContentHash,
			&e.ContentTokens,
			&e.ModelID,
			&e.ModelProvider,
			&e.ModelName,
			&e.ModelDimensions,
			&e.ConfiguredDimensions,
			&e.ProcessingTimeMS,
			&e.EmbeddingCreatedAt,
			&e.Magnitude,
			&e.TenantID,
			&e.Metadata,
			&e.CreatedAt,
			&e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan embedding: %w", err)
		}
		embeddings = append(embeddings, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating embeddings: %w", err)
	}

	return embeddings, nil
}
