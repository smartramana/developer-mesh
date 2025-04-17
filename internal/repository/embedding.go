package repository

// Embedding represents a vector embedding in the database
type Embedding struct {
	ID           string    `json:"id"`
	ContextID    string    `json:"context_id"`
	ContentIndex int       `json:"content_index"`
	Text         string    `json:"text"`
	Embedding    []float32 `json:"embedding"`
	ModelID      string    `json:"model_id"`
}

// EmbeddingSearchResult represents a search result with similarity score
type EmbeddingSearchResult struct {
	Embedding *Embedding `json:"embedding"`
	Score     float32    `json:"score"`
}

// EmbeddingRepository handles storage and retrieval of embeddings
type EmbeddingRepository struct {
	// Implementation details would be here for a real repository
}
