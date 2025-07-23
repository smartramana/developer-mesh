package providers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// InMemoryContextStorage implements context storage using in-memory map
// This is primarily for development and testing purposes
type InMemoryContextStorage struct {
	contexts map[string]*models.Context
	lock     sync.RWMutex
}

// NewInMemoryContextStorage creates a new in-memory context storage provider
func NewInMemoryContextStorage() *InMemoryContextStorage {
	return &InMemoryContextStorage{
		contexts: make(map[string]*models.Context),
	}
}

// StoreContext stores a context in memory
func (s *InMemoryContextStorage) StoreContext(ctx context.Context, contextData *models.Context) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Create a deep copy to avoid external modifications
	contextCopy := s.deepCopyContext(contextData)

	// Store context
	s.contexts[contextData.ID] = contextCopy

	return nil
}

// GetContext retrieves a context from memory
func (s *InMemoryContextStorage) GetContext(ctx context.Context, contextID string) (*models.Context, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	// Get context from map
	contextData, exists := s.contexts[contextID]
	if !exists {
		return nil, fmt.Errorf("context not found: %s", contextID)
	}

	// Create a deep copy to avoid external modifications
	return s.deepCopyContext(contextData), nil
}

// DeleteContext deletes a context from memory
func (s *InMemoryContextStorage) DeleteContext(ctx context.Context, contextID string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Check if context exists
	if _, exists := s.contexts[contextID]; !exists {
		return fmt.Errorf("context not found: %s", contextID)
	}

	// Delete context
	delete(s.contexts, contextID)

	return nil
}

// ListContexts lists contexts from memory
func (s *InMemoryContextStorage) ListContexts(ctx context.Context, agentID string, sessionID string) ([]*models.Context, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var result []*models.Context

	// Filter contexts by agent ID and optionally by session ID
	for _, contextData := range s.contexts {
		if contextData.AgentID != agentID {
			continue
		}

		if sessionID != "" && contextData.SessionID != sessionID {
			continue
		}

		// Create a deep copy to avoid external modifications
		result = append(result, s.deepCopyContext(contextData))
	}

	return result, nil
}

// deepCopyContext creates a deep copy of a context
func (s *InMemoryContextStorage) deepCopyContext(src *models.Context) *models.Context {
	if src == nil {
		return nil
	}

	// Copy context
	dst := &models.Context{
		ID:            src.ID,
		AgentID:       src.AgentID,
		ModelID:       src.ModelID,
		SessionID:     src.SessionID,
		CurrentTokens: src.CurrentTokens,
		MaxTokens:     src.MaxTokens,
		CreatedAt:     src.CreatedAt,
		UpdatedAt:     src.UpdatedAt,
		ExpiresAt:     src.ExpiresAt,
		Content:       make([]models.ContextItem, len(src.Content)),
	}

	// Copy metadata
	if src.Metadata != nil {
		dst.Metadata = make(map[string]interface{})
		for k, v := range src.Metadata {
			dst.Metadata[k] = v
		}
	}

	// Note: Links field has been removed as part of migration from mcp.Context to models.Context

	// Copy content
	for i, item := range src.Content {
		dstItem := models.ContextItem{
			ID:        item.ID,
			Role:      item.Role,
			Content:   item.Content,
			Tokens:    item.Tokens,
			Timestamp: item.Timestamp,
		}

		// Copy item metadata
		if item.Metadata != nil {
			dstItem.Metadata = make(map[string]interface{})
			for k, v := range item.Metadata {
				dstItem.Metadata[k] = v
			}
		}

		dst.Content[i] = dstItem
	}

	return dst
}

// Cleanup removes expired contexts
func (s *InMemoryContextStorage) Cleanup() {
	s.lock.Lock()
	defer s.lock.Unlock()

	now := time.Now()

	// Find expired contexts
	var expired []string
	for id, contextData := range s.contexts {
		if !contextData.ExpiresAt.IsZero() && contextData.ExpiresAt.Before(now) {
			expired = append(expired, id)
		}
	}

	// Delete expired contexts
	for _, id := range expired {
		delete(s.contexts, id)
	}
}

// StartCleanupTask starts a periodic cleanup task
func (s *InMemoryContextStorage) StartCleanupTask(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.Cleanup()
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}
