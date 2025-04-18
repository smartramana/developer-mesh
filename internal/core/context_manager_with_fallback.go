package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"go.opentelemetry.io/otel/attribute"
)

// ContextManagerWithFallback wraps the standard ContextManager with fallback capabilities
type ContextManagerWithFallback struct {
	primary  interfaces.ContextManager
	fallback *FallbackService
	metricsClient *observability.MetricsClient
}

// NewContextManagerWithFallback creates a new context manager with fallback
func NewContextManagerWithFallback(db *database.Database, cache interfaces.Cache) *ContextManagerWithFallback {
	// Create primary context manager
	primary := NewContextManager(db, cache)
	
	// Create fallback service
	fallback := NewFallbackService(primary)
	
	return &ContextManagerWithFallback{
		primary:      primary,
		fallback:     fallback,
		metricsClient: observability.NewMetricsClient(),
	}
}

// CreateContext creates a new context with fallback
func (cm *ContextManagerWithFallback) CreateContext(ctx context.Context, context *mcp.Context) (*mcp.Context, error) {
	// Start tracing
	ctx, span := observability.StartSpan(ctx, "context_manager_with_fallback.create_context")
	defer span.End()
	
	startTime := time.Now()
	
	// Try primary method first
	result, err := cm.primary.CreateContext(ctx, context)
	if err == nil {
		// Record metrics - primary succeeded
		duration := time.Since(startTime)
		cm.metricsClient.RecordContextOperation("create", context.ModelID, duration.Seconds(), 0)
		return result, nil
	}
	
	// Log the primary error
	log.Printf("Primary context creation failed: %v, attempting fallback", err)
	span.RecordError(err)
	
	// Try fallback
	emergencyContext, fallbackErr := cm.fallback.CreateEmergencyContext(
		ctx, 
		context.AgentID, 
		context.ModelID, 
		context.SessionID, 
		context.MaxTokens,
	)
	
	if fallbackErr != nil {
		// Both primary and fallback failed
		log.Printf("Fallback context creation also failed: %v", fallbackErr)
		span.RecordError(fallbackErr)
		return nil, fmt.Errorf("context creation failed: %w, fallback also failed: %v", err, fallbackErr)
	}
	
	// Record metrics - fallback succeeded
	duration := time.Since(startTime)
	cm.metricsClient.RecordContextOperation("create_fallback", context.ModelID, duration.Seconds(), 0)
	
	return emergencyContext, nil
}

// GetContext gets a context with fallback
func (cm *ContextManagerWithFallback) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	// Start tracing
	ctx, span := observability.StartSpan(ctx, "context_manager_with_fallback.get_context")
	span.SetAttributes(attribute.String("context_id", contextID))
	defer span.End()
	
	startTime := time.Now()
	
	// Try primary method first
	result, err := cm.primary.GetContext(ctx, contextID)
	if err == nil {
		// Record metrics - primary succeeded
		duration := time.Since(startTime)
		cm.metricsClient.RecordContextOperation("get", result.ModelID, duration.Seconds(), 0)
		return result, nil
	}
	
	// Log the primary error
	log.Printf("Primary context retrieval failed: %v, attempting fallback", err)
	span.RecordError(err)
	
	// Try fallback
	fallbackContext, fallbackErr := cm.fallback.GetContextWithFallback(ctx, contextID)
	
	if fallbackErr != nil {
		// Both primary and fallback failed
		log.Printf("Fallback context retrieval also failed: %v", fallbackErr)
		span.RecordError(fallbackErr)
		return nil, fmt.Errorf("context retrieval failed: %w, fallback also failed: %v", err, fallbackErr)
	}
	
	// Record metrics - fallback succeeded
	duration := time.Since(startTime)
	cm.metricsClient.RecordContextOperation("get_fallback", fallbackContext.ModelID, duration.Seconds(), 0)
	
	return fallbackContext, nil
}

// UpdateContext updates a context with fallback
func (cm *ContextManagerWithFallback) UpdateContext(ctx context.Context, contextID string, context *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error) {
	// Start tracing
	ctx, span := observability.StartSpan(ctx, "context_manager_with_fallback.update_context")
	span.SetAttributes(attribute.String("context_id", contextID))
	defer span.End()
	
	startTime := time.Now()
	
	// Try primary method first
	result, err := cm.primary.UpdateContext(ctx, contextID, context, options)
	if err == nil {
		// Record metrics - primary succeeded
		duration := time.Since(startTime)
		cm.metricsClient.RecordContextOperation("update", context.ModelID, duration.Seconds(), 0)
		return result, nil
	}
	
	// Log the primary error
	log.Printf("Primary context update failed: %v, attempting fallback", err)
	span.RecordError(err)
	
	// Check if this is a context too large error
	if errors.Is(err, ErrContextTooLarge) || (err != nil && (errors.Unwrap(err) == ErrContextTooLarge || errors.Unwrap(errors.Unwrap(err)) == ErrContextTooLarge)) {
		// Try fallback context truncation
		targetTokens := context.MaxTokens
		if targetTokens <= 0 {
			// Default to a reasonable size if no max specified
			targetTokens = 8000
		}
		
		// Create a copy of the context to avoid modifying the original
		contextCopy := *context
		
		// Use fallback to reduce context size
		truncatedContext, fallbackErr := cm.fallback.ReduceContextSize(ctx, &contextCopy, targetTokens)
		if fallbackErr != nil {
			// Both primary and fallback failed
			log.Printf("Fallback context truncation also failed: %v", fallbackErr)
			span.RecordError(fallbackErr)
			return nil, fmt.Errorf("context update failed: %w, fallback truncation also failed: %v", err, fallbackErr)
		}
		
		// Try to update with the truncated context
		result, truncateErr := cm.primary.UpdateContext(ctx, contextID, truncatedContext, nil)
		if truncateErr != nil {
			// Still failed
			log.Printf("Update with truncated context also failed: %v", truncateErr)
			span.RecordError(truncateErr)
			return nil, fmt.Errorf("context update failed: %w, update with truncated context also failed: %v", err, truncateErr)
		}
		
		// Record metrics - fallback succeeded
		duration := time.Since(startTime)
		cm.metricsClient.RecordContextOperation("update_fallback_truncate", context.ModelID, duration.Seconds(), 0)
		
		return result, nil
	}
	
	// For other types of errors, just return the original error
	return nil, err
}

// DeleteContext deletes a context
func (cm *ContextManagerWithFallback) DeleteContext(ctx context.Context, contextID string) error {
	// Start tracing
	ctx, span := observability.StartSpan(ctx, "context_manager_with_fallback.delete_context")
	span.SetAttributes(attribute.String("context_id", contextID))
	defer span.End()
	
	// For deletion, we don't need a fallback - just try the primary
	err := cm.primary.DeleteContext(ctx, contextID)
	if err != nil {
		span.RecordError(err)
	}
	
	return err
}

// ListContexts lists contexts for an agent
func (cm *ContextManagerWithFallback) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	// Start tracing
	ctx, span := observability.StartSpan(ctx, "context_manager_with_fallback.list_contexts")
	span.SetAttributes(
		attribute.String("agent_id", agentID),
		attribute.String("session_id", sessionID),
	)
	defer span.End()
	
	// Try primary method first
	result, err := cm.primary.ListContexts(ctx, agentID, sessionID, options)
	if err == nil {
		return result, nil
	}
	
	// Log the primary error
	log.Printf("Primary context listing failed: %v, no fallback available", err)
	span.RecordError(err)
	
	// No fallback for listing contexts
	return nil, err
}

// SearchInContext searches within a context
func (cm *ContextManagerWithFallback) SearchInContext(ctx context.Context, contextID string, query string) ([]mcp.SearchResult, error) {
	// Start tracing
	ctx, span := observability.StartSpan(ctx, "context_manager_with_fallback.search_in_context")
	span.SetAttributes(
		attribute.String("context_id", contextID),
		attribute.String("query", query),
	)
	defer span.End()
	
	// Try primary method first
	result, err := cm.primary.SearchInContext(ctx, contextID, query)
	if err == nil {
		return result, nil
	}
	
	// Log the primary error
	log.Printf("Primary context search failed: %v, no fallback available", err)
	span.RecordError(err)
	
	// No fallback for search
	return nil, err
}

// SummarizeContext summarizes a context with fallback
func (cm *ContextManagerWithFallback) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	// Start tracing
	ctx, span := observability.StartSpan(ctx, "context_manager_with_fallback.summarize_context")
	span.SetAttributes(attribute.String("context_id", contextID))
	defer span.End()
	
	startTime := time.Now()
	
	// Try primary method first
	summary, err := cm.primary.SummarizeContext(ctx, contextID)
	if err == nil {
		// Get model ID for metrics
		context, _ := cm.primary.GetContext(ctx, contextID)
		modelID := "unknown"
		if context != nil {
			modelID = context.ModelID
		}
		
		// Record metrics - primary succeeded
		duration := time.Since(startTime)
		cm.metricsClient.RecordContextOperation("summarize", modelID, duration.Seconds(), 0)
		
		return summary, nil
	}
	
	// Log the primary error
	log.Printf("Primary context summarization failed: %v, attempting fallback", err)
	span.RecordError(err)
	
	// Try fallback
	fallbackSummary, fallbackErr := cm.fallback.SummarizeContext(ctx, contextID)
	
	if fallbackErr != nil {
		// Both primary and fallback failed
		log.Printf("Fallback context summarization also failed: %v", fallbackErr)
		span.RecordError(fallbackErr)
		return "", fmt.Errorf("context summarization failed: %w, fallback also failed: %v", err, fallbackErr)
	}
	
	// Get model ID for metrics
	context, _ := cm.primary.GetContext(ctx, contextID)
	modelID := "unknown"
	if context != nil {
		modelID = context.ModelID
	}
	
	// Record metrics - fallback succeeded
	duration := time.Since(startTime)
	cm.metricsClient.RecordContextOperation("summarize_fallback", modelID, duration.Seconds(), 0)
	
	return fallbackSummary, nil
}
