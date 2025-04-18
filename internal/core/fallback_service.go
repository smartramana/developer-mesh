package core

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"go.opentelemetry.io/otel/attribute"
)

// FallbackService provides degraded service when primary services fail
type FallbackService struct {
	contextManager *ContextManager
	metricsClient  *observability.MetricsClient
}

// NewFallbackService creates a new fallback service
func NewFallbackService(contextManager *ContextManager) *FallbackService {
	return &FallbackService{
		contextManager: contextManager,
		metricsClient:  observability.NewMetricsClient(),
	}
}

// SummarizeContext creates a basic summary of a context when the primary summarizer fails
func (s *FallbackService) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	// Start tracing
	ctx, span := observability.StartSpan(ctx, "fallback.summarize_context")
	span.SetAttributes(attribute.String("context_id", contextID))
	defer span.End()
	
	startTime := time.Now()
	
	// Get the context
	context, err := s.contextManager.GetContext(ctx, contextID)
	if err != nil {
		return "", fmt.Errorf("fallback summarizer: failed to get context: %w", err)
	}
	
	// Generate basic summary with limited information
	modelName := "Unknown model"
	if context.ModelID != "" {
		parts := strings.Split(context.ModelID, ".")
		if len(parts) > 0 {
			modelName = parts[0]
		}
	}
	
	// Get message counts by role
	userMsgCount := 0
	assistantMsgCount := 0
	otherMsgCount := 0
	
	for _, item := range context.Content {
		switch item.Role {
		case "user":
			userMsgCount++
		case "assistant":
			assistantMsgCount++
		default:
			otherMsgCount++
		}
	}
	
	// Generate summary
	summary := fmt.Sprintf(
		"This conversation includes %d messages (%d from user, %d from assistant, %d other) using %s model.",
		len(context.Content),
		userMsgCount,
		assistantMsgCount,
		otherMsgCount,
		modelName,
	)
	
	// Add metadata if available
	if context.Metadata != nil {
		if topic, ok := context.Metadata["topic"].(string); ok && topic != "" {
			summary += fmt.Sprintf(" Topic: %s.", topic)
		}
		
		if completed, ok := context.Metadata["completed"].(bool); ok && completed {
			summary += " This conversation is marked as completed."
		}
	}
	
	// Record metrics
	duration := time.Since(startTime)
	s.metricsClient.RecordContextOperation("fallback_summarize", context.ModelID, duration.Seconds(), 0)
	
	return summary, nil
}

// ReduceContextSize reduces context size when standard truncation fails
func (s *FallbackService) ReduceContextSize(ctx context.Context, context *mcp.Context, targetTokens int) (*mcp.Context, error) {
	// Start tracing
	ctx, span := observability.StartSpan(ctx, "fallback.reduce_context_size")
	span.SetAttributes(
		attribute.String("context_id", context.ID),
		attribute.Int("target_tokens", targetTokens),
	)
	defer span.End()
	
	startTime := time.Now()
	
	// Simple strategy: keep system messages and most recent messages
	if len(context.Content) == 0 {
		return context, nil
	}
	
	// Count current tokens
	currentTokens := 0
	for _, item := range context.Content {
		currentTokens += item.Tokens
	}
	
	// Only proceed if we need to reduce
	if currentTokens <= targetTokens {
		return context, nil
	}
	
	log.Printf("Fallback context reduction: reducing from %d to %d tokens", currentTokens, targetTokens)
	
	// Create new context with same metadata
	reducedContext := &mcp.Context{
		ID:          context.ID,
		AgentID:     context.AgentID,
		ModelID:     context.ModelID,
		SessionID:   context.SessionID,
		MaxTokens:   context.MaxTokens,
		Metadata:    context.Metadata,
	}
	
	// Collect system messages
	systemMessages := make([]mcp.ContextItem, 0)
	for _, item := range context.Content {
		if item.Role == "system" {
			systemMessages = append(systemMessages, item)
		}
	}
	
	// Calculate tokens used by system messages
	systemTokens := 0
	for _, item := range systemMessages {
		systemTokens += item.Tokens
	}
	
	// Calculate how many tokens we have left for non-system messages
	remainingTokens := targetTokens - systemTokens
	if remainingTokens < 0 {
		// If system messages alone exceed target, just take system messages that fit
		reducedSystemMessages := make([]mcp.ContextItem, 0)
		currentSystemTokens := 0
		
		// Take newest system messages first (assuming they're more relevant)
		for i := len(systemMessages) - 1; i >= 0; i-- {
			if currentSystemTokens + systemMessages[i].Tokens <= targetTokens {
				reducedSystemMessages = append([]mcp.ContextItem{systemMessages[i]}, reducedSystemMessages...)
				currentSystemTokens += systemMessages[i].Tokens
			}
		}
		
		reducedContext.Content = reducedSystemMessages
		reducedContext.Metadata["truncated"] = true
		reducedContext.Metadata["truncated_at"] = time.Now()
		reducedContext.Metadata["original_tokens"] = currentTokens
		reducedContext.Metadata["truncated_tokens"] = currentSystemTokens
		reducedContext.Metadata["truncation_method"] = "fallback_system_only"
		
		// Record metrics
		duration := time.Since(startTime)
		s.metricsClient.RecordContextOperation("fallback_truncate", context.ModelID, duration.Seconds(), currentSystemTokens)
		
		return reducedContext, nil
	}
	
	// Start with system messages
	reducedContext.Content = append(reducedContext.Content, systemMessages...)
	
	// Add most recent non-system messages that fit
	nonSystemMessages := make([]mcp.ContextItem, 0)
	for _, item := range context.Content {
		if item.Role != "system" {
			nonSystemMessages = append(nonSystemMessages, item)
		}
	}
	
	// Take newest non-system messages first
	currentNonSystemTokens := 0
	for i := len(nonSystemMessages) - 1; i >= 0; i-- {
		if currentNonSystemTokens + nonSystemMessages[i].Tokens <= remainingTokens {
			reducedContext.Content = append([]mcp.ContextItem{nonSystemMessages[i]}, reducedContext.Content...)
			currentNonSystemTokens += nonSystemMessages[i].Tokens
		}
	}
	
	// Update metadata
	reducedContext.Metadata["truncated"] = true
	reducedContext.Metadata["truncated_at"] = time.Now()
	reducedContext.Metadata["original_tokens"] = currentTokens
	reducedContext.Metadata["truncated_tokens"] = systemTokens + currentNonSystemTokens
	reducedContext.Metadata["truncation_method"] = "fallback_newest_messages"
	
	// Record metrics
	duration := time.Since(startTime)
	s.metricsClient.RecordContextOperation("fallback_truncate", context.ModelID, duration.Seconds(), systemTokens+currentNonSystemTokens)
	
	return reducedContext, nil
}

// CreateEmergencyContext creates a minimal context when normal context creation fails
func (s *FallbackService) CreateEmergencyContext(ctx context.Context, agentID, modelID, sessionID string, maxTokens int) (*mcp.Context, error) {
	// Start tracing
	ctx, span := observability.StartSpan(ctx, "fallback.create_emergency_context")
	span.SetAttributes(
		attribute.String("agent_id", agentID),
		attribute.String("model_id", modelID),
		attribute.String("session_id", sessionID),
	)
	defer span.End()
	
	startTime := time.Now()
	
	// Generate a unique ID
	contextID := GenerateContextID()
	
	// Create a minimal context
	emergencyContext := &mcp.Context{
		ID:        contextID,
		AgentID:   agentID,
		ModelID:   modelID,
		SessionID: sessionID,
		MaxTokens: maxTokens,
		Content: []mcp.ContextItem{
			{
				Role:      "system",
				Content:   "The system is currently experiencing high load. Limited functionality is available.",
				Timestamp: time.Now(),
				Tokens:    13,
			},
		},
		Metadata: map[string]interface{}{
			"emergency_mode": true,
			"created_at":    time.Now(),
		},
	}
	
	// Record metrics
	duration := time.Since(startTime)
	s.metricsClient.RecordContextOperation("emergency_context", modelID, duration.Seconds(), 13)
	
	return emergencyContext, nil
}

// GetContextWithFallback gets a context with fallback mechanisms
func (s *FallbackService) GetContextWithFallback(ctx context.Context, contextID string) (*mcp.Context, error) {
	// Start tracing
	ctx, span := observability.StartSpan(ctx, "fallback.get_context")
	span.SetAttributes(attribute.String("context_id", contextID))
	defer span.End()
	
	// Try primary method first
	context, err := s.contextManager.GetContext(ctx, contextID)
	if err == nil {
		return context, nil
	}
	
	// If primary failed, try to get from cache directly
	// We're bypassing the normal path to try to get something
	if s.contextManager.cache != nil {
		cachedContext, err := s.contextManager.cache.GetContext(ctx, contextID)
		if err == nil && cachedContext != nil {
			// Add metadata to indicate this was a fallback
			if cachedContext.Metadata == nil {
				cachedContext.Metadata = make(map[string]interface{})
			}
			cachedContext.Metadata["fallback_retrieval"] = true
			cachedContext.Metadata["fallback_timestamp"] = time.Now()
			return cachedContext, nil
		}
	}
	
	// If we still don't have a context, return the original error
	return nil, err
}
