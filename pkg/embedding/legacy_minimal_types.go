package embedding

import "context"

// ModelType represents the type of embedding model - TEMPORARY for legacy code cleanup
type ModelType string

const (
	ModelTypeOpenAI      ModelType = "openai"
	ModelTypeHuggingFace ModelType = "huggingface"
	ModelTypeBedrock     ModelType = "bedrock"
	ModelTypeAnthropic   ModelType = "anthropic"
	ModelTypeCustom      ModelType = "custom"
)

// ModelConfig contains configuration for embedding models - TEMPORARY for legacy code cleanup
type ModelConfig struct {
	Type       ModelType              `json:"type"`
	Name       string                 `json:"name"`
	APIKey     string                 `json:"api_key,omitempty"`
	Endpoint   string                 `json:"endpoint,omitempty"`
	Dimensions int                    `json:"dimensions"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// EmbeddingVector represents a vector embedding with metadata - TEMPORARY for legacy code cleanup
type EmbeddingVector struct {
	Vector      []float32              `json:"vector"`
	Dimensions  int                    `json:"dimensions"`
	ModelID     string                 `json:"model_id"`
	ContentType string                 `json:"content_type"`
	ContentID   string                 `json:"content_id"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// EmbeddingService defines the interface for generating embeddings - TEMPORARY for legacy code cleanup
type EmbeddingService interface {
	GenerateEmbedding(ctx context.Context, text string, contentType string, contentID string) (*EmbeddingVector, error)
	BatchGenerateEmbeddings(ctx context.Context, texts []string, contentType string, contentIDs []string) ([]*EmbeddingVector, error)
	GetModelConfig() ModelConfig
	GetModelDimensions() int
}

// EmbeddingStorage defines the interface for storing and retrieving embeddings - TEMPORARY for legacy code cleanup
type EmbeddingStorage interface {
	StoreEmbedding(ctx context.Context, embedding *EmbeddingVector) error
	BatchStoreEmbeddings(ctx context.Context, embeddings []*EmbeddingVector) error
	FindSimilarEmbeddings(ctx context.Context, embedding *EmbeddingVector, limit int, threshold float32) ([]*EmbeddingVector, error)
	GetEmbeddingsByContentIDs(ctx context.Context, contentIDs []string) ([]*EmbeddingVector, error)
	DeleteEmbeddingsByContentIDs(ctx context.Context, contentIDs []string) error
}

// Provider interface for embedding providers - TEMPORARY for legacy code cleanup
type Provider interface {
	GenerateEmbedding(ctx context.Context, content string, model string) ([]float32, error)
	GetSupportedModels() []string
	ValidateAPIKey() error
}