package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// ContextManager defines the interface for context operations
type ContextManager interface {
	// GetContext retrieves a context by ID
	GetContext(ctx context.Context, contextID string) (*models.Context, error)

	// UpdateContext updates an existing context
	UpdateContext(ctx context.Context, contextID string, updatedContext *models.Context, options any) (*models.Context, error)
}

// ContextEventHandler handles context-related events
type ContextEventHandler struct {
	contextManager ContextManager
}

// NewContextEventHandler creates a new context event handler
func NewContextEventHandler(contextManager ContextManager) *ContextEventHandler {
	return &ContextEventHandler{
		contextManager: contextManager,
	}
}

// RegisterWithEventBus registers this handler with the event bus
func (h *ContextEventHandler) RegisterWithEventBus(bus *EventBus) {
	// Register for context events
	contextEvents := []EventType{
		EventContextCreated,
		EventContextUpdated,
		EventContextDeleted,
		EventContextRetrieved,
		EventContextSummarized,
	}

	for _, eventType := range contextEvents {
		bus.Subscribe(eventType, h.HandleContextEvent)
	}

	// Register for tool events that affect contexts
	bus.Subscribe(EventToolActionExecuted, h.HandleToolEvent)
}

// HandleContextEvent handles context events
func (h *ContextEventHandler) HandleContextEvent(ctx context.Context, event *models.Event) error {
	// Extract context ID from event data
	var contextID string
	if data, ok := event.Data.(map[string]any); ok {
		if id, ok := data["context_id"].(string); ok {
			contextID = id
		}
	}

	if contextID == "" {
		return fmt.Errorf("context event without context ID: %v", event)
	}

	// Handle based on event type
	switch EventType(event.Type) {
	case EventContextCreated:
		// Already created, nothing to do
		return nil

	case EventContextUpdated:
		// Already updated, nothing to do
		return nil

	case EventContextDeleted:
		// Already deleted, nothing to do
		return nil

	case EventContextRetrieved:
		// Update metadata to track access
		existingContext, err := h.contextManager.GetContext(ctx, contextID)
		if err != nil {
			return fmt.Errorf("failed to get context for metadata update: %w", err)
		}

		// Update metadata
		if existingContext.Metadata == nil {
			existingContext.Metadata = make(map[string]any)
		}

		// Update access timestamp
		existingContext.Metadata["last_accessed_at"] = time.Now()

		// Increment access count
		accessCount := 1
		if count, ok := existingContext.Metadata["access_count"].(float64); ok {
			accessCount = int(count) + 1
		}
		existingContext.Metadata["access_count"] = accessCount

		// Update context
		_, err = h.contextManager.UpdateContext(ctx, contextID, existingContext, nil)
		if err != nil {
			return fmt.Errorf("failed to update context metadata: %w", err)
		}

		return nil

	case EventContextSummarized:
		// Extract summary from event data
		var summary string
		if data, ok := event.Data.(map[string]any); ok {
			if s, ok := data["summary"].(string); ok {
				summary = s
			}
		}

		if summary == "" {
			return fmt.Errorf("summary event without summary: %v", event)
		}

		// Update context metadata with summary
		existingContext, err := h.contextManager.GetContext(ctx, contextID)
		if err != nil {
			return fmt.Errorf("failed to get context for summary update: %w", err)
		}

		// Update metadata
		if existingContext.Metadata == nil {
			existingContext.Metadata = make(map[string]any)
		}

		// Add summary to metadata
		existingContext.Metadata["summary"] = summary
		existingContext.Metadata["summarized_at"] = time.Now()

		// Update context
		_, err = h.contextManager.UpdateContext(ctx, contextID, existingContext, nil)
		if err != nil {
			return fmt.Errorf("failed to update context summary: %w", err)
		}

		return nil

	default:
		return fmt.Errorf("unsupported context event type: %s", event.Type)
	}
}

// HandleToolEvent handles tool events
func (h *ContextEventHandler) HandleToolEvent(ctx context.Context, event *models.Event) error {
	// Extract context ID from event data
	var contextID string
	var tool string
	var action string
	var result any

	if data, ok := event.Data.(map[string]any); ok {
		if id, ok := data["context_id"].(string); ok {
			contextID = id
		}
		if t, ok := data["tool"].(string); ok {
			tool = t
		}
		if a, ok := data["action"].(string); ok {
			action = a
		}
		if r, ok := data["result"]; ok {
			result = r
		}
	}

	if contextID == "" || tool == "" || action == "" {
		return fmt.Errorf("tool event missing required data: %v", event)
	}

	// Get existing context
	existingContext, err := h.contextManager.GetContext(ctx, contextID)
	if err != nil {
		return fmt.Errorf("failed to get context for tool event: %w", err)
	}

	// Create tool action context item
	resultJSON, err := json.Marshal(result)
	if err != nil {
		log.Printf("Warning: failed to marshal tool result: %v", err)
		resultJSON = []byte("{}")
	}

	toolAction := models.ContextItem{
		Role:      "tool",
		Content:   fmt.Sprintf("Tool %s executed action %s with result: %s", tool, action, string(resultJSON)),
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"tool":   tool,
			"action": action,
			"result": result,
		},
	}

	// Add tool action to context
	existingContext.Content = append(existingContext.Content, toolAction)

	// Update context
	_, err = h.contextManager.UpdateContext(ctx, contextID, existingContext, nil)
	if err != nil {
		return fmt.Errorf("failed to update context with tool action: %w", err)
	}

	return nil
}

// AgentEventHandler handles agent-related events
type AgentEventHandler struct {
	// You could add dependencies here, like a service for tracking agent status
}

// NewAgentEventHandler creates a new agent event handler
func NewAgentEventHandler() *AgentEventHandler {
	return &AgentEventHandler{}
}

// RegisterWithEventBus registers this handler with the event bus
func (h *AgentEventHandler) RegisterWithEventBus(bus *EventBus) {
	// Register for agent events
	agentEvents := []EventType{
		EventAgentConnected,
		EventAgentDisconnected,
		EventAgentError,
	}

	for _, eventType := range agentEvents {
		bus.Subscribe(eventType, h.HandleAgentEvent)
	}
}

// HandleAgentEvent handles agent events
func (h *AgentEventHandler) HandleAgentEvent(ctx context.Context, event *models.Event) error {
	// Extract agent ID
	agentID := event.AgentID
	if agentID == "" {
		return fmt.Errorf("agent event without agent ID: %v", event)
	}

	// Handle based on event type
	switch EventType(event.Type) {
	case EventAgentConnected:
		log.Printf("Agent connected: %s", agentID)
	case EventAgentDisconnected:
		log.Printf("Agent disconnected: %s", agentID)
	case EventAgentError:
		log.Printf("Agent error: %s - %v", agentID, event.Data)
	default:
		return fmt.Errorf("unsupported agent event type: %s", event.Type)
	}

	return nil
}

// SystemEventHandler handles system-related events
type SystemEventHandler struct {
	// You could add dependencies here
}

// NewSystemEventHandler creates a new system event handler
func NewSystemEventHandler() *SystemEventHandler {
	return &SystemEventHandler{}
}

// RegisterWithEventBus registers this handler with the event bus
func (h *SystemEventHandler) RegisterWithEventBus(bus *EventBus) {
	// Register for system events
	systemEvents := []EventType{
		EventSystemStartup,
		EventSystemShutdown,
		EventSystemHealthCheck,
	}

	for _, eventType := range systemEvents {
		bus.Subscribe(eventType, h.HandleSystemEvent)
	}
}

// HandleSystemEvent handles system events
func (h *SystemEventHandler) HandleSystemEvent(ctx context.Context, event *models.Event) error {
	// Handle based on event type
	switch EventType(event.Type) {
	case EventSystemStartup:
		log.Printf("System starting up")
	case EventSystemShutdown:
		log.Printf("System shutting down")
	case EventSystemHealthCheck:
		log.Printf("System health check")
	default:
		return fmt.Errorf("unsupported system event type: %s", event.Type)
	}

	return nil
}
