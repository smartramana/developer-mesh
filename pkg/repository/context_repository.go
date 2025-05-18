package repository

import (
	"context"
)

// Context represents a persistent context object
type Context struct {
	ID         string                 `json:"id" db:"id"`
	Name       string                 `json:"name" db:"name"`
	AgentID    string                 `json:"agent_id" db:"agent_id"`
	SessionID  string                 `json:"session_id" db:"session_id"`
	Status     string                 `json:"status" db:"status"`
	Properties map[string]interface{} `json:"properties" db:"properties"`
	CreatedAt  int64                  `json:"created_at" db:"created_at"`
	UpdatedAt  int64                  `json:"updated_at" db:"updated_at"`
}

// ContextItem represents an item in a context
type ContextItem struct {
	ID        string                 `json:"id" db:"id"`
	ContextID string                 `json:"context_id" db:"context_id"`
	Content   string                 `json:"content" db:"content"`
	Type      string                 `json:"type" db:"type"`
	Score     float64                `json:"score" db:"score"`
	Metadata  map[string]interface{} `json:"metadata" db:"metadata"`
}

// ContextRepository defines methods for interacting with contexts
type ContextRepository interface {
	// Create creates a new context
	Create(ctx context.Context, contextObj *Context) error
	
	// Get retrieves a context by ID
	Get(ctx context.Context, id string) (*Context, error)
	
	// Update updates an existing context
	Update(ctx context.Context, contextObj *Context) error
	
	// Delete deletes a context by ID
	Delete(ctx context.Context, id string) error
	
	// List lists contexts with optional filtering
	List(ctx context.Context, filter map[string]interface{}) ([]*Context, error)
	
	// Search searches for text within a context
	Search(ctx context.Context, contextID, query string) ([]ContextItem, error)
	
	// Summarize generates a summary of a context
	Summarize(ctx context.Context, contextID string) (string, error)
}
