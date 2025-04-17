package repository

// TestEmbedding represents a simplified vector embedding for testing
type TestEmbedding struct {
	ID           string    `json:"id"`
	ContextID    string    `json:"context_id"`
	ContentIndex int       `json:"content_index"`
	Text         string    `json:"text"`
	Embedding    []float32 `json:"embedding"`
	ModelID      string    `json:"model_id"`
}

// TestEmbeddingSearchResult represents a search result with similarity score
type TestEmbeddingSearchResult struct {
	Embedding *TestEmbedding `json:"embedding"`
	Score     float32        `json:"score"`
}
