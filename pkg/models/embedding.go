package models

// Embedding represents a vector embedding with additional metadata
type Embedding struct {
	ID           string    `json:"id"`
	ContextID    string    `json:"context_id"`
	ContentIndex int       `json:"content_index"`
	Text         string    `json:"text"`
	Embedding    []float32 `json:"embedding"`
	ModelID      string    `json:"model_id"`
}
