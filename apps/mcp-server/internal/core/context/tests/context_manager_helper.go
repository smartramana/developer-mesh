package tests

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// TruncationStrategy defines the strategy for truncating a context
type TruncationStrategy string

const (
	// StrategyOldestFirst truncates the oldest items first
	StrategyOldestFirst TruncationStrategy = "oldest_first"

	// StrategyPreservingUser truncates by removing assistant responses while preserving user messages
	StrategyPreservingUser TruncationStrategy = "preserving_user"

	// StrategyRelevanceBased truncates based on relevance to the current conversation
	StrategyRelevanceBased TruncationStrategy = "relevance_based"
)

// Helper functions for truncation tests

// DoTruncateContext truncates a context based on the specified strategy
func DoTruncateContext(contextData *models.Context, strategy TruncationStrategy) error {
	switch strategy {
	case StrategyOldestFirst:
		return DoTruncateOldestFirst(contextData)
	case StrategyPreservingUser:
		return DoTruncatePreservingUser(contextData)
	case StrategyRelevanceBased:
		return fmt.Errorf("relevance-based truncation not implemented")
	default:
		// Default to oldest-first if strategy is not specified or invalid
		return DoTruncateOldestFirst(contextData)
	}
}

// DoTruncateOldestFirst truncates a context by removing the oldest items first
func DoTruncateOldestFirst(contextData *models.Context) error {
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

// DoTruncatePreservingUser truncates a context while preserving user messages
func DoTruncatePreservingUser(contextData *models.Context) error {
	if contextData.CurrentTokens <= contextData.MaxTokens {
		return nil
	}

	// Group content items by role
	userItems := make([]models.ContextItem, 0)
	assistantItems := make([]models.ContextItem, 0)
	systemItems := make([]models.ContextItem, 0)
	otherItems := make([]models.ContextItem, 0)

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

		// tokensToRemove -= removedUserTokens // Removed: ineffectual assignment
		userTokens -= removedUserTokens
	}

	// Reconstruct content
	newContent := make([]models.ContextItem, 0)
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

// MockContextManager provides a simple implementation for testing
type MockContextManager struct {
}

// NewMockContextManager creates a new mock context manager
func NewMockContextManager() *MockContextManager {
	return &MockContextManager{}
}

// CreateContext creates a new context
func (m *MockContextManager) CreateContext(ctx context.Context, contextData *models.Context) (*models.Context, error) {
	// Validate required fields
	if contextData.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	if contextData.ModelID == "" {
		return nil, fmt.Errorf("model_id is required")
	}

	// Set default values
	if contextData.ID == "" {
		contextData.ID = "test-id"
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
		contextData.Content = []models.ContextItem{}
	}

	// Calculate current tokens if not set
	if contextData.CurrentTokens == 0 && len(contextData.Content) > 0 {
		for _, item := range contextData.Content {
			contextData.CurrentTokens += item.Tokens
		}
	}

	return contextData, nil
}
