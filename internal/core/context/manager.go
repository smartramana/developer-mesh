package context

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/cache"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/internal/events/system"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/internal/storage/providers"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/google/uuid"
)

// TruncateStrategy defines the strategy for truncating a context
type TruncateStrategy string

const (
	// TruncateOldestFirst truncates the oldest items first
	TruncateOldestFirst TruncateStrategy = "oldest_first"
	
	// TruncatePreservingUser truncates by removing assistant responses while preserving user messages
	TruncatePreservingUser TruncateStrategy = "preserving_user"
	
	// TruncateRelevanceBased truncates based on relevance to the current conversation
	TruncateRelevanceBased TruncateStrategy = "relevance_based"
)

// Manager manages conversation contexts
type Manager struct {
	db            *database.Database
	cache         cache.Cache
	storage       providers.ContextStorage
	eventBus      *system.EventBus
	logger        *observability.Logger
	subscribers   map[string][]func(mcp.Event)
	lock          sync.RWMutex
	metricsClient observability.MetricsClient
}

// NewManager creates a new context manager
func NewManager(
	db *database.Database, 
	cache cache.Cache, 
	storage providers.ContextStorage,
	eventBus *system.EventBus,
	logger *observability.Logger,
	metricsClient observability.MetricsClient,
) *Manager {
	if logger == nil {
		logger = observability.NewLogger("context_manager")
	}

	return &Manager{
		db:            db,
		cache:         cache,
		storage:       storage,
		eventBus:      eventBus,
		logger:        logger,
		subscribers:   make(map[string][]func(mcp.Event)),
		metricsClient: metricsClient,
	}
}

// CreateContext creates a new context
func (m *Manager) CreateContext(ctx context.Context, contextData *mcp.Context) (*mcp.Context, error) {
	startTime := time.Now()
	defer func() {
		m.recordMetrics("create_context", startTime)
	}()

	// Validate required fields
	if contextData.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	
	if contextData.ModelID == "" {
		return nil, fmt.Errorf("model_id is required")
	}
	
	// Generate ID if not provided
	if contextData.ID == "" {
		contextData.ID = uuid.New().String()
	}
	
	// Set timestamps
	now := time.Now()
	contextData.CreatedAt = now
	contextData.UpdatedAt = now
	
	// Set max tokens if not provided
	if contextData.MaxTokens == 0 {
		contextData.MaxTokens = 4000 // Default value
	}
	
	// Initialize content if nil
	if contextData.Content == nil {
		contextData.Content = []mcp.ContextItem{}
	}
	
	// Calculate current tokens if not set
	if contextData.CurrentTokens == 0 && len(contextData.Content) > 0 {
		for _, item := range contextData.Content {
			contextData.CurrentTokens += item.Tokens
		}
	}
	
	// Save to database
	if err := m.db.Transaction(ctx, func(tx *database.Tx) error {
		return m.createContextInDB(ctx, tx, contextData)
	}); err != nil {
		return nil, fmt.Errorf("failed to create context in database: %w", err)
	}
	
	// Save to storage if we have large content
	if len(contextData.Content) > 0 {
		if err := m.storage.StoreContext(ctx, contextData); err != nil {
			m.logger.Warn("Failed to store context in storage", map[string]interface{}{
				"error":      err.Error(),
				"context_id": contextData.ID,
			})
			// Don't fail the operation if storage fails
		}
	}
	
	// Cache the context
	if err := m.cacheContext(contextData); err != nil {
		m.logger.Warn("Failed to cache context", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextData.ID,
		})
		// Don't fail the operation if caching fails
	}
	
	// Publish event
	m.publishEvent(mcp.Event{
		Source:    "context_manager",
		Type:      "context_created",
		AgentID:   contextData.AgentID,
		SessionID: contextData.SessionID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"context_id": contextData.ID},
	})
	
	return contextData, nil
}

// GetContext retrieves a context by ID
func (m *Manager) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	startTime := time.Now()
	defer func() {
		m.recordMetrics("get_context", startTime)
	}()

	// Try to get from cache first
	cachedContext, err := m.getCachedContext(contextID)
	if err == nil {
		return cachedContext, nil
	}
	
	// If not in cache, get from database
	var contextData *mcp.Context
	
	if err := m.db.Transaction(ctx, func(tx *database.Tx) error {
		var err error
		contextData, err = m.getContextFromDB(ctx, tx, contextID)
		return err
	}); err != nil {
		// If not found in database, try storage
		storageContext, storageErr := m.storage.GetContext(ctx, contextID)
		if storageErr != nil {
			return nil, fmt.Errorf("failed to get context: %w", err)
		}
		
		contextData = storageContext
	}
	
	// Cache the context for future use
	if contextData != nil {
		if err := m.cacheContext(contextData); err != nil {
			m.logger.Warn("Failed to cache context", map[string]interface{}{
				"error":      err.Error(),
				"context_id": contextData.ID,
			})
			// Don't fail the operation if caching fails
		}
	}
	
	return contextData, nil
}

// UpdateContext updates an existing context
func (m *Manager) UpdateContext(ctx context.Context, contextID string, updateData *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error) {
	startTime := time.Now()
	defer func() {
		m.recordMetrics("update_context", startTime)
	}()

	// Get existing context
	existingContext, err := m.GetContext(ctx, contextID)
	if err != nil {
		return nil, err
	}
	
	// Update fields that can be updated
	if updateData.AgentID != "" {
		existingContext.AgentID = updateData.AgentID
	}
	
	if updateData.SessionID != "" {
		existingContext.SessionID = updateData.SessionID
	}
	
	// Update metadata
	if updateData.Metadata != nil {
		if existingContext.Metadata == nil {
			existingContext.Metadata = make(map[string]interface{})
		}
		
		for k, v := range updateData.Metadata {
			existingContext.Metadata[k] = v
		}
	}
	
	// Handle content updates
	if updateData.Content != nil {
		// If ReplaceContent is true, replace the entire content
		if options != nil && options.ReplaceContent {
			existingContext.Content = updateData.Content
			
			// Recalculate token count
			existingContext.CurrentTokens = 0
			for _, item := range existingContext.Content {
				existingContext.CurrentTokens += item.Tokens
			}
		} else {
			// Add new content items
			for _, item := range updateData.Content {
				// Generate ID if not provided
				if item.ID == "" {
					item.ID = uuid.New().String()
				}
				
				// Set timestamp if not provided
				if item.Timestamp.IsZero() {
					item.Timestamp = time.Now()
				}
				
				existingContext.Content = append(existingContext.Content, item)
				existingContext.CurrentTokens += item.Tokens
			}
		}
	}
	
	// Update timestamp
	existingContext.UpdatedAt = time.Now()
	
	// Check if context needs truncation
	if options != nil && options.Truncate && existingContext.CurrentTokens > existingContext.MaxTokens {
		// Truncate context based on strategy
		if err := m.truncateContext(existingContext, TruncateStrategy(options.TruncateStrategy)); err != nil {
			return nil, fmt.Errorf("failed to truncate context: %w", err)
		}
	}
	
	// Save to database
	if err := m.db.Transaction(ctx, func(tx *database.Tx) error {
		return m.updateContextInDB(ctx, tx, existingContext)
	}); err != nil {
		return nil, fmt.Errorf("failed to update context in database: %w", err)
	}
	
	// Save to storage if we have large content
	if len(existingContext.Content) > 0 {
		if err := m.storage.StoreContext(ctx, existingContext); err != nil {
			m.logger.Warn("Failed to store context in storage", map[string]interface{}{
				"error":      err.Error(),
				"context_id": existingContext.ID,
			})
			// Don't fail the operation if storage fails
		}
	}
	
	// Cache the updated context
	if err := m.cacheContext(existingContext); err != nil {
		m.logger.Warn("Failed to cache context", map[string]interface{}{
			"error":      err.Error(),
			"context_id": existingContext.ID,
		})
		// Don't fail the operation if caching fails
	}
	
	// Publish event
	m.publishEvent(mcp.Event{
		Source:    "context_manager",
		Type:      "context_updated",
		AgentID:   existingContext.AgentID,
		SessionID: existingContext.SessionID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"context_id": existingContext.ID},
	})
	
	return existingContext, nil
}

// DeleteContext deletes a context
func (m *Manager) DeleteContext(ctx context.Context, contextID string) error {
	startTime := time.Now()
	defer func() {
		m.recordMetrics("delete_context", startTime)
	}()

	// Get context to ensure it exists and to get metadata for the event
	contextData, err := m.GetContext(ctx, contextID)
	if err != nil {
		return err
	}
	
	// Delete from database
	if err := m.db.Transaction(ctx, func(tx *database.Tx) error {
		return m.deleteContextFromDB(ctx, tx, contextID)
	}); err != nil {
		return fmt.Errorf("failed to delete context from database: %w", err)
	}
	
	// Delete from storage
	if err := m.storage.DeleteContext(ctx, contextID); err != nil {
		m.logger.Warn("Failed to delete context from storage", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		// Don't fail the operation if storage deletion fails
	}
	
	// Delete from cache
	if err := m.cache.Delete(ctx, fmt.Sprintf("context:%s", contextID)); err != nil {
		m.logger.Warn("Failed to delete context from cache", map[string]interface{}{
			"error":      err.Error(),
			"context_id": contextID,
		})
		// Don't fail the operation if cache deletion fails
	}
	
	// Publish event
	m.publishEvent(mcp.Event{
		Source:    "context_manager",
		Type:      "context_deleted",
		AgentID:   contextData.AgentID,
		SessionID: contextData.SessionID,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"context_id": contextID},
	})
	
	return nil
}

// ListContexts lists contexts for an agent
func (m *Manager) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	startTime := time.Now()
	defer func() {
		m.recordMetrics("list_contexts", startTime)
	}()

	var contexts []*mcp.Context
	
	// Get from database
	if err := m.db.Transaction(ctx, func(tx *database.Tx) error {
		var err error
		contexts, err = m.listContextsFromDB(ctx, tx, agentID, sessionID, options)
		return err
	}); err != nil {
		// If database query fails, try storage
		storageContexts, storageErr := m.storage.ListContexts(ctx, agentID, sessionID)
		if storageErr != nil {
			return nil, fmt.Errorf("failed to list contexts: %w", err)
		}
		
		contexts = storageContexts
	}
	
	return contexts, nil
}

// SummarizeContext generates a summary of a context
func (m *Manager) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	startTime := time.Now()
	defer func() {
		m.recordMetrics("summarize_context", startTime)
	}()

	// Get context
	contextData, err := m.GetContext(ctx, contextID)
	if err != nil {
		return "", fmt.Errorf("failed to get context: %w", err)
	}
	
	// Simple summary implementation
	var userMessages, assistantMessages, systemMessages, otherMessages int
	
	for _, item := range contextData.Content {
		switch item.Role {
		case "user":
			userMessages++
		case "assistant":
			assistantMessages++
		case "system":
			systemMessages++
		default:
			otherMessages++
		}
	}
	
	summary := fmt.Sprintf(
		"%d messages (%d user, %d assistant, %d system, %d other), %d/%d tokens",
		len(contextData.Content),
		userMessages,
		assistantMessages,
		systemMessages,
		otherMessages,
		contextData.CurrentTokens,
		contextData.MaxTokens,
	)
	
	return summary, nil
}

// SearchInContext searches for text within a context
func (m *Manager) SearchInContext(ctx context.Context, contextID string, query string) ([]mcp.ContextItem, error) {
	startTime := time.Now()
	defer func() {
		m.recordMetrics("search_in_context", startTime)
	}()

	if query == "" {
		return []mcp.ContextItem{}, nil
	}
	
	// Get context
	contextData, err := m.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}
	
	// Simple text search implementation
	var results []mcp.ContextItem
	for _, item := range contextData.Content {
		if strings.Contains(strings.ToLower(item.Content), strings.ToLower(query)) {
			results = append(results, item)
		}
	}
	
	return results, nil
}

// Subscribe subscribes to context events
func (m *Manager) Subscribe(eventType string, handler func(mcp.Event)) {
	m.lock.Lock()
	defer m.lock.Unlock()
	
	if m.subscribers[eventType] == nil {
		m.subscribers[eventType] = make([]func(mcp.Event), 0)
	}
	
	m.subscribers[eventType] = append(m.subscribers[eventType], handler)
}

// publishEvent publishes an event to subscribers
func (m *Manager) publishEvent(event mcp.Event) {
	// Publish to event bus if available
	if m.eventBus != nil {
		eventData := make(map[string]interface{})
		
		// Convert event.Data to map if it's not nil
		if event.Data != nil {
			if dataMap, ok := event.Data.(map[string]interface{}); ok {
				eventData = dataMap
			}
		}
		
		// Add standard fields
		eventData["agent_id"] = event.AgentID
		eventData["source"] = event.Source
		
		if event.SessionID != "" {
			eventData["session_id"] = event.SessionID
		}
		
		// Publish to event bus
		if err := m.eventBus.Publish(context.Background(), system.Event{
			Type:      system.EventType(event.Type),
			Timestamp: event.Timestamp,
			Data:      eventData,
		}); err != nil {
			m.logger.Warn("Failed to publish event to event bus", map[string]interface{}{
				"error":      err.Error(),
				"event_type": event.Type,
			})
		}
	}

	// Notify subscribers
	m.lock.RLock()
	defer m.lock.RUnlock()
	
	// Notify specific event type subscribers
	if handlers, ok := m.subscribers[event.Type]; ok {
		for _, handler := range handlers {
			go handler(event)
		}
	}
	
	// Notify "all" event subscribers
	if handlers, ok := m.subscribers["all"]; ok {
		for _, handler := range handlers {
			go handler(event)
		}
	}
}

// truncateContext truncates a context based on the specified strategy
func (m *Manager) truncateContext(contextData *mcp.Context, strategy TruncateStrategy) error {
	switch strategy {
	case TruncateOldestFirst:
		return m.truncateOldestFirst(contextData)
	case TruncatePreservingUser:
		return m.truncatePreservingUser(contextData)
	case TruncateRelevanceBased:
		return fmt.Errorf("relevance-based truncation not implemented")
	default:
		// Default to oldest-first if strategy is not specified or invalid
		return m.truncateOldestFirst(contextData)
	}
}

// truncateOldestFirst truncates a context by removing the oldest items first
func (m *Manager) truncateOldestFirst(contextData *mcp.Context) error {
	if contextData.CurrentTokens <= contextData.MaxTokens {
		return nil
	}
	
	// Sort content by timestamp (oldest first)
	sort.Slice(contextData.Content, func(i, j int) bool {
		return contextData.Content[i].Timestamp.Before(contextData.Content[j].Timestamp)
	})
	
	// Remove oldest items until under max tokens
	tokensToRemove := contextData.CurrentTokens - contextData.MaxTokens
	removed := 0
	removeCount := 0
	
	for i := 0; i < len(contextData.Content) && removed < tokensToRemove; i++ {
		removed += contextData.Content[i].Tokens
		removeCount++
	}
	
	// Update content and token count
	if removeCount > 0 {
		contextData.Content = contextData.Content[removeCount:]
		contextData.CurrentTokens -= removed
	}
	
	return nil
}

// truncatePreservingUser truncates a context while preserving user messages
func (m *Manager) truncatePreservingUser(contextData *mcp.Context) error {
	if contextData.CurrentTokens <= contextData.MaxTokens {
		return nil
	}
	
	// Group content items by role
	userItems := make([]mcp.ContextItem, 0)
	assistantItems := make([]mcp.ContextItem, 0)
	systemItems := make([]mcp.ContextItem, 0)
	otherItems := make([]mcp.ContextItem, 0)
	
	for _, item := range contextData.Content {
		switch item.Role {
		case "user":
			userItems = append(userItems, item)
		case "assistant":
			assistantItems = append(assistantItems, item)
		case "system":
			systemItems = append(systemItems, item)
		default:
			otherItems = append(otherItems, item)
		}
	}
	
	// Sort assistant items by timestamp (oldest first)
	sort.Slice(assistantItems, func(i, j int) bool {
		return assistantItems[i].Timestamp.Before(assistantItems[j].Timestamp)
	})
	
	// Calculate tokens by role
	userTokens := 0
	for _, item := range userItems {
		userTokens += item.Tokens
	}
	
	assistantTokens := 0
	for _, item := range assistantItems {
		assistantTokens += item.Tokens
	}
	
	systemTokens := 0
	for _, item := range systemItems {
		systemTokens += item.Tokens
	}
	
	otherTokens := 0
	for _, item := range otherItems {
		otherTokens += item.Tokens
	}
	
	// Tokens to remove
	tokensToRemove := contextData.CurrentTokens - contextData.MaxTokens
	
	// Remove assistant messages first (oldest first)
	removedAssistantTokens := 0
	removedAssistantCount := 0
	
	for i := 0; i < len(assistantItems) && removedAssistantTokens < tokensToRemove; i++ {
		removedAssistantTokens += assistantItems[i].Tokens
		removedAssistantCount++
	}
	
	// Remove removed assistant items
	if removedAssistantCount > 0 {
		assistantItems = assistantItems[removedAssistantCount:]
	}
	
	// If still over max tokens, remove oldest user messages
	tokensToRemove -= removedAssistantTokens
	
	if tokensToRemove > 0 {
		// Sort user items by timestamp (oldest first)
		sort.Slice(userItems, func(i, j int) bool {
			return userItems[i].Timestamp.Before(userItems[j].Timestamp)
		})
		
		removedUserTokens := 0
		removedUserCount := 0
		
		for i := 0; i < len(userItems) && removedUserTokens < tokensToRemove; i++ {
			removedUserTokens += userItems[i].Tokens
			removedUserCount++
		}
		
		// Remove removed user items
		if removedUserCount > 0 {
			userItems = userItems[removedUserCount:]
		}
		
		tokensToRemove -= removedUserTokens
		userTokens -= removedUserTokens
	}
	
	// Reconstruct content
	newContent := make([]mcp.ContextItem, 0)
	newContent = append(newContent, systemItems...)
	
	// Interleave user and assistant messages by timestamp
	allItems := append(userItems, assistantItems...)
	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].Timestamp.Before(allItems[j].Timestamp)
	})
	
	newContent = append(newContent, allItems...)
	newContent = append(newContent, otherItems...)
	
	// Update context
	contextData.Content = newContent
	contextData.CurrentTokens = systemTokens + (userTokens) + (assistantTokens - removedAssistantTokens) + otherTokens
	
	return nil
}

// cacheContext caches a context
func (m *Manager) cacheContext(contextData *mcp.Context) error {
	// Skip caching if context is expired
	if !contextData.ExpiresAt.IsZero() && contextData.ExpiresAt.Before(time.Now()) {
		return nil
	}
	
	// Determine cache expiration
	expiration := 24 * time.Hour // Default expiration
	
	if !contextData.ExpiresAt.IsZero() {
		// If context has explicit expiration, use time until expiration
		expiration = time.Until(contextData.ExpiresAt)
	}
	
	// Cache the context
	cacheKey := fmt.Sprintf("context:%s", contextData.ID)
	err := m.cache.Set(context.Background(), cacheKey, contextData, expiration)
	if err != nil {
		return fmt.Errorf("failed to cache context: %w", err)
	}
	
	return nil
}

// getCachedContext gets a context from cache
func (m *Manager) getCachedContext(contextID string) (*mcp.Context, error) {
	var contextData mcp.Context
	
	cacheKey := fmt.Sprintf("context:%s", contextID)
	err := m.cache.Get(context.Background(), cacheKey, &contextData)
	if err != nil {
		return nil, err
	}
	
	return &contextData, nil
}

// recordMetrics records metrics for the operation
func (m *Manager) recordMetrics(operation string, startTime time.Time) {
	duration := time.Since(startTime)
	
	if m.metricsClient != nil {
		m.metricsClient.RecordHistogram(
			"context_manager_operation_duration_ms",
			float64(duration.Milliseconds()),
			map[string]string{"operation": operation},
		)
		
		m.metricsClient.IncrementCounter(
			"context_manager_operations_total",
			map[string]string{"operation": operation},
		)
	}
}

// Database operations

// createContextInDB creates a context in the database
func (m *Manager) createContextInDB(ctx context.Context, tx *database.Tx, contextData *mcp.Context) error {
	// Implementation depends on the database schema
	// This is a placeholder implementation
	return nil
}

// getContextFromDB retrieves a context from the database
func (m *Manager) getContextFromDB(ctx context.Context, tx *database.Tx, contextID string) (*mcp.Context, error) {
	// Implementation depends on the database schema
	// This is a placeholder implementation
	return nil, fmt.Errorf("not implemented")
}

// updateContextInDB updates a context in the database
func (m *Manager) updateContextInDB(ctx context.Context, tx *database.Tx, contextData *mcp.Context) error {
	// Implementation depends on the database schema
	// This is a placeholder implementation
	return nil
}

// deleteContextFromDB deletes a context from the database
func (m *Manager) deleteContextFromDB(ctx context.Context, tx *database.Tx, contextID string) error {
	// Implementation depends on the database schema
	// This is a placeholder implementation
	return nil
}

// listContextsFromDB lists contexts from the database
func (m *Manager) listContextsFromDB(ctx context.Context, tx *database.Tx, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	// Implementation depends on the database schema
	// This is a placeholder implementation
	return nil, fmt.Errorf("not implemented")
}
