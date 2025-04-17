package api

// StoreEmbeddingRequest represents a request to store an embedding
type StoreEmbeddingRequest struct {
	ContextID    string    `json:"context_id"`
	ContentIndex int       `json:"content_index"`
	Text         string    `json:"text"`
	Embedding    []float32 `json:"embedding"`
	ModelID      string    `json:"model_id"`
}

// SearchEmbeddingsRequest represents a request to search for embeddings
type SearchEmbeddingsRequest struct {
	QueryEmbedding []float32 `json:"query_embedding"`
	ContextID      string    `json:"context_id"`
	Limit          int       `json:"limit"`
}
