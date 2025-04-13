package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/cache"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/google/uuid"
)

// TruncationStrategy defines how to truncate contexts when they exceed max tokens
type TruncationStrategy string

const (
	// TruncateOldestFirst removes oldest messages first
	TruncateOldestFirst TruncationStrategy = "oldest_first"
	
	// TruncateRelevanceBased removes least relevant messages based on semantic similarity
	TruncateRelevanceBased TruncationStrategy = "relevance_based"
	
	// TruncateUserPreserving removes assistant messages before user messages
	TruncateUserPreserving TruncationStrategy = "preserve_user"
	
	// TruncateCompression attempts to compress messages before removal
	TruncateCompression TruncationStrategy = "compression"
)

// ContextManager handles storage and retrieval of context data
type ContextManager struct {
	db          *database.Database
	cache       cache.Cache
	mu          sync.RWMutex
	subscribers map[string][]func(mcp.Event)
}

// NewContextManager creates a new context manager
func NewContextManager(db *database.Database, cache cache.Cache) *ContextManager {
	return &ContextManager{
		db:          db,
		cache:       cache,
		subscribers: make(map[string][]func(mcp.Event)),
	}
}

// CreateContext creates a new context
func (cm *ContextManager) CreateContext(ctx context.Context, request *mcp.Context) (*mcp.Context, error) {
	if request.ID == "" {
		request.ID = uuid.New().String()
	}
	
	if request.AgentID == "" {
		return nil, errors.New("agent_id is required")
	}
	
	if request.ModelID == "" {
		return nil, errors.New("model_id is required")
	}
	
	// Set timestamps
	now := time.Now()
	request.CreatedAt = now
	request.UpdatedAt = now
	
	// Initialize metadata if not present
	if request.Metadata == nil {
		request.Metadata = make(map[string]interface{})
	}
	
	// Save to database
	err := cm.db.CreateContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create context: %w", err)
	}
	
	// Cache the context
	err = cm.cacheContext(request)
	if err != nil {
		log.Printf("Warning: failed to cache context %s: %v", request.ID, err)
	}
	
	// Notify subscribers
	cm.publishEvent(mcp.Event{
		Source:    "context_manager",
		Type:      "context_created",
		AgentID:   request.AgentID,
		SessionID: request.SessionID,
		Timestamp: now,
		Data:      request,
	})
	
	return request, nil
}

// GetContext retrieves a context by ID
func (cm *ContextManager) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	// Try to get from cache first
	cachedContext, err := cm.getCachedContext(contextID)
	if err == nil {
		return cachedContext, nil
	}
	
	// Fallback to database
	contextData, err := cm.db.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}
	
	// Cache the context
	err = cm.cacheContext(contextData)
	if err != nil {
		log.Printf("Warning: failed to cache context %s: %v", contextID, err)
	}
	
	return contextData, nil
}

// UpdateContext updates an existing context
func (cm *ContextManager) UpdateContext(ctx context.Context, contextID string, updateRequest *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error) {
	// Get the current context
	currentContext, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}
	
	// Update fields
	if updateRequest.AgentID != "" {
		currentContext.AgentID = updateRequest.AgentID
	}
	
	if updateRequest.ModelID != "" {
		currentContext.ModelID = updateRequest.ModelID
	}
	
	if updateRequest.SessionID != "" {
		currentContext.SessionID = updateRequest.SessionID
	}
	
	// Merge metadata
	if updateRequest.Metadata != nil {
		if currentContext.Metadata == nil {
			currentContext.Metadata = make(map[string]interface{})
		}
		
		for k, v := range updateRequest.Metadata {
			currentContext.Metadata[k] = v
		}
	}
	
	// Update content
	if updateRequest.Content != nil {
		// Append new items
		currentContext.Content = append(currentContext.Content, updateRequest.Content...)
		
		// Update token count
		for _, item := range updateRequest.Content {
			currentContext.CurrentTokens += item.Tokens
		}
		
		// Handle truncation if needed
		if options != nil && options.Truncate && currentContext.MaxTokens > 0 && currentContext.CurrentTokens > currentContext.MaxTokens {
			err = cm.truncateContext(currentContext, options.TruncateStrategy)
			if err != nil {
				return nil, fmt.Errorf("failed to truncate context: %w", err)
			}
		}
	}
	
	// Update timestamps
	currentContext.UpdatedAt = time.Now()
	if !updateRequest.ExpiresAt.IsZero() {
		currentContext.ExpiresAt = updateRequest.ExpiresAt
	}
	
	// Save to database
	err = cm.db.UpdateContext(ctx, currentContext)
	if err != nil {
		return nil, fmt.Errorf("failed to update context: %w", err)
	}
	
	// Update cache
	err = cm.cacheContext(currentContext)
	if err != nil {
		log.Printf("Warning: failed to cache context %s: %v", contextID, err)
	}
	
	// Notify subscribers
	cm.publishEvent(mcp.Event{
		Source:    "context_manager",
		Type:      "context_updated",
		AgentID:   currentContext.AgentID,
		SessionID: currentContext.SessionID,
		Timestamp: time.Now(),
		Data:      currentContext,
	})
	
	return currentContext, nil
}

// DeleteContext deletes a context
func (cm *ContextManager) DeleteContext(ctx context.Context, contextID string) error {
	// Get context info for event notification
	contextData, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return err
	}
	
	// Delete from database
	err = cm.db.DeleteContext(ctx, contextID)
	if err != nil {
		return fmt.Errorf("failed to delete context: %w", err)
	}
	
	// Remove from cache
	cacheKey := fmt.Sprintf("context:%s", contextID)
	err = cm.cache.Delete(ctx, cacheKey)
	if err != nil {
		log.Printf("Warning: failed to remove context %s from cache: %v", contextID, err)
	}
	
	// Notify subscribers
	cm.publishEvent(mcp.Event{
		Source:    "context_manager",
		Type:      "context_deleted",
		AgentID:   contextData.AgentID,
		SessionID: contextData.SessionID,
		Timestamp: time.Now(),
		Data: map[string]string{
			"context_id": contextID,
		},
	})
	
	return nil
}

// ListContexts lists contexts for an agent
func (cm *ContextManager) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	// Get from database
	contexts, err := cm.db.ListContexts(ctx, agentID, sessionID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to list contexts: %w", err)
	}
	
	return contexts, nil
}

// SummarizeContext generates a summary of a context
func (cm *ContextManager) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	// Get the context
	contextData, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return "", err
	}
	
	// Here we would use an external summarization service or implement summarization logic
	// For now, we'll return a simple summary
	summary := fmt.Sprintf("Context with %d messages and %d tokens", len(contextData.Content), contextData.CurrentTokens)
	
	return summary, nil
}

// SearchInContext searches for text within a context
func (cm *ContextManager) SearchInContext(ctx context.Context, contextID string, query string) ([]mcp.ContextItem, error) {
	// Get the context
	contextData, err := cm.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}
	
	// Simple search implementation - in a real system, this would use more sophisticated search
	var results []mcp.ContextItem
	for _, item := range contextData.Content {
		if strings.Contains(strings.ToLower(item.Content), strings.ToLower(query)) {
			results = append(results, item)
		}
	}
	
	return results, nil
}

// Subscribe registers a callback for a specific event type
func (cm *ContextManager) Subscribe(eventType string, callback func(mcp.Event)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.subscribers[eventType] = append(cm.subscribers[eventType], callback)
}

// getCachedContext retrieves a context from cache
func (cm *ContextManager) getCachedContext(contextID string) (*mcp.Context, error) {
	cacheKey := fmt.Sprintf("context:%s", contextID)
	
	var contextData mcp.Context
	err := cm.cache.Get(context.Background(), cacheKey, &contextData)
	if err != nil {
		return nil, err
	}
	
	return &contextData, nil
}

// cacheContext stores a context in the cache
func (cm *ContextManager) cacheContext(contextData *mcp.Context) error {
	cacheKey := fmt.Sprintf("context:%s", contextData.ID)
	
	// Set TTL based on expiration time if available
	var ttl time.Duration
	if !contextData.ExpiresAt.IsZero() {
		ttl = time.Until(contextData.ExpiresAt)
		if ttl <= 0 {
			// Don't cache if already expired
			return nil
		}
	} else {
		// Default TTL (e.g., 1 hour)
		ttl = 1 * time.Hour
	}
	
	return cm.cache.Set(context.Background(), cacheKey, contextData, ttl)
}

// truncateContext truncates a context based on the specified strategy
func (cm *ContextManager) truncateContext(contextData *mcp.Context, strategy string) error {
	switch TruncationStrategy(strategy) {
	case TruncateOldestFirst:
		return cm.truncateOldestFirst(contextData)
	case TruncateRelevanceBased:
		return cm.truncateByRelevance(contextData)
	case TruncateUserPreserving:
		return cm.truncatePreservingUser(contextData)
	case TruncateCompression:
		return cm.truncateWithCompression(contextData)
	default:
		// Default to oldest first
		return cm.truncateOldestFirst(contextData)
	}
}

// truncateOldestFirst removes oldest messages until below max tokens
func (cm *ContextManager) truncateOldestFirst(contextData *mcp.Context) error {
	// Keep removing oldest items until we're below the max tokens
	for len(contextData.Content) > 0 && contextData.CurrentTokens > contextData.MaxTokens {
		// Remove the oldest item (first in the slice)
		removedItem := contextData.Content[0]
		contextData.Content = contextData.Content[1:]
		contextData.CurrentTokens -= removedItem.Tokens
	}
	
	return nil
}

// truncateByRelevance removes least relevant messages
func (cm *ContextManager) truncateByRelevance(contextData *mcp.Context) error {
	// This would require embedding/relevance scoring implementation
	// For now, fallback to oldest first
	return cm.truncateOldestFirst(contextData)
}

// truncatePreservingUser removes assistant messages before user messages
func (cm *ContextManager) truncatePreservingUser(contextData *mcp.Context) error {
	// Keep a minimum number of recent messages
	minRecentMessages := 4
	if len(contextData.Content) <= minRecentMessages {
		return nil
	}
	
	// First try removing assistant messages from older parts of the conversation
	var newContent []mcp.ContextItem
	var newTokenCount int
	
	// Always keep the most recent messages
	recentMessages := contextData.Content[len(contextData.Content)-minRecentMessages:]
	olderMessages := contextData.Content[:len(contextData.Content)-minRecentMessages]
	
	// Count tokens in recent messages
	recentTokens := 0
	for _, item := range recentMessages {
		recentTokens += item.Tokens
	}
	
	// Start with system messages and user messages from older parts
	for _, item := range olderMessages {
		if item.Role == "system" || item.Role == "user" {
			newContent = append(newContent, item)
			newTokenCount += item.Tokens
		}
	}
	
	// If we still have room, add assistant messages
	remainingTokens := contextData.MaxTokens - recentTokens - newTokenCount
	for _, item := range olderMessages {
		if item.Role == "assistant" && item.Tokens <= remainingTokens {
			newContent = append(newContent, item)
			newTokenCount += item.Tokens
			remainingTokens -= item.Tokens
		}
	}
	
	// Sort content by timestamp
	// In a real implementation, we'd need to sort newContent by timestamp
	
	// Add recent messages
	newContent = append(newContent, recentMessages...)
	
	// Update context
	contextData.Content = newContent
	contextData.CurrentTokens = newTokenCount + recentTokens
	
	// If we're still over the limit, fall back to oldest first
	if contextData.CurrentTokens > contextData.MaxTokens {
		return cm.truncateOldestFirst(contextData)
	}
	
	return nil
}

// truncateWithCompression attempts to compress messages before removal
func (cm *ContextManager) truncateWithCompression(contextData *mcp.Context) error {
	// In a real implementation, this would use an LLM to summarize/compress older messages
	// For now, fallback to oldest first
	return cm.truncateOldestFirst(contextData)
}

// publishEvent notifies all subscribers of an event
func (cm *ContextManager) publishEvent(event mcp.Event) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	// Notify subscribers of this specific event type
	if callbacks, ok := cm.subscribers[event.Type]; ok {
		for _, callback := range callbacks {
			go callback(event)
		}
	}
	
	// Notify subscribers of all events
	if callbacks, ok := cm.subscribers["all"]; ok {
		for _, callback := range callbacks {
			go callback(event)
		}
	}
}
