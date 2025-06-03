package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// HandlerFunc is a function that handles a webhook event
type HandlerFunc func(ctx context.Context, event Event) error

// Filter represents a filter for webhook events
type Filter struct {
	EventTypes   []string         // Event types to handle
	Repositories []string         // Repositories to handle
	Branches     []string         // Branches to handle
	Actions      []string         // Actions to handle
	Custom       func(Event) bool // Custom filter function
}

// Matches checks if an event matches the filter
func (f *Filter) Matches(event Event) bool {
	// Check event type
	if len(f.EventTypes) > 0 {
		matched := false
		for _, eventType := range f.EventTypes {
			if eventType == event.Type {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check repository
	if len(f.Repositories) > 0 && event.RepositoryFullName != "" {
		matched := false
		for _, repo := range f.Repositories {
			if repo == event.RepositoryFullName {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check branch (for push events and pull requests)
	if len(f.Branches) > 0 && event.RefName != "" {
		matched := false
		for _, branch := range f.Branches {
			if branch == event.RefName {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check action (e.g. "opened", "closed", "edited")
	if len(f.Actions) > 0 && event.Action != "" {
		matched := false
		for _, action := range f.Actions {
			if action == event.Action {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Apply custom filter if provided
	if f.Custom != nil {
		return f.Custom(event)
	}

	return true
}

// Handler represents a webhook event handler
type Handler struct {
	ID      string
	Handler HandlerFunc
	Filter  *Filter
}

// Manager manages webhook event handlers
type Manager struct {
	handlers map[string]*Handler
	eventBus any // Changed to generic interface
	logger   observability.Logger
	mu       sync.RWMutex
}

// NewManager creates a new webhook handler manager
func NewManager(eventBus any, logger observability.Logger) *Manager {
	return &Manager{
		handlers: make(map[string]*Handler),
		eventBus: eventBus,
		logger:   logger,
	}
}

// Register registers a new webhook handler
func (m *Manager) Register(id string, handler HandlerFunc, filter *Filter) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if handler with same ID already exists
	if _, exists := m.handlers[id]; exists {
		return fmt.Errorf("handler with ID %s already exists", id)
	}

	// Create handler
	m.handlers[id] = &Handler{
		ID:      id,
		Handler: handler,
		Filter:  filter,
	}

	m.logger.Info("Registered webhook handler", map[string]any{
		"handlerID": id,
	})

	return nil
}

// Unregister unregisters a webhook handler
func (m *Manager) Unregister(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if handler exists
	if _, exists := m.handlers[id]; !exists {
		return fmt.Errorf("no handler with ID %s exists", id)
	}

	// Remove handler
	delete(m.handlers, id)

	m.logger.Info("Unregistered webhook handler", map[string]any{
		"handlerID": id,
	})

	return nil
}

// List lists all registered webhook handlers
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.handlers))
	for id := range m.handlers {
		ids = append(ids, id)
	}

	return ids
}

// ProcessEvent processes a webhook event
func (m *Manager) ProcessEvent(ctx context.Context, event Event) error {
	m.mu.RLock()
	handlers := make([]*Handler, 0, len(m.handlers))
	for _, handler := range m.handlers {
		handlers = append(handlers, handler)
	}
	m.mu.RUnlock()

	// Find matching handlers
	matchingHandlers := make([]*Handler, 0)
	for _, handler := range handlers {
		if handler.Filter == nil || handler.Filter.Matches(event) {
			matchingHandlers = append(matchingHandlers, handler)
		}
	}

	m.logger.Info("Processing webhook event", map[string]any{
		"eventType":        event.Type,
		"deliveryID":       event.DeliveryID,
		"matchingHandlers": len(matchingHandlers),
	})

	// Process event with matching handlers
	for _, handler := range matchingHandlers {
		handlerCtx := context.WithValue(ctx, "handlerID", handler.ID)

		// Execute handler
		err := handler.Handler(handlerCtx, event)
		if err != nil {
			m.logger.Error("Error processing webhook event", map[string]any{
				"eventType":  event.Type,
				"deliveryID": event.DeliveryID,
				"handlerID":  handler.ID,
				"error":      err.Error(),
			})

			// Publish error event if eventBus supports the interface
			if typedEventBus, ok := m.eventBus.(interface {
				Publish(context.Context, string, map[string]any) error
			}); ok {
				if pubErr := typedEventBus.Publish(context.Background(), "github.webhook.error", map[string]any{
					"eventType":  event.Type,
					"deliveryID": event.DeliveryID,
					"handlerID":  handler.ID,
					"error":      err.Error(),
					"source":     "github",
					"timestamp":  time.Now().Format(time.RFC3339),
				}); pubErr != nil {
					m.logger.Warn("Failed to publish webhook error event", map[string]any{"error": pubErr})
				}
			}

			// Continue processing with other handlers
		} else {
			// Publish success event if eventBus supports the interface
			if typedEventBus, ok := m.eventBus.(interface {
				Publish(context.Context, string, map[string]any) error
			}); ok {
				if pubErr := typedEventBus.Publish(context.Background(), "github.webhook.success", map[string]any{
					"eventType":  event.Type,
					"deliveryID": event.DeliveryID,
					"handlerID":  handler.ID,
					"source":     "github",
					"timestamp":  time.Now().Format(time.RFC3339),
				}); pubErr != nil {
					m.logger.Warn("Failed to publish webhook success event", map[string]any{"error": pubErr})
				}
			}
		}
	}

	return nil
}

// Event represents a GitHub webhook event
type Event struct {
	Type               string         // GitHub event type
	DeliveryID         string         // GitHub delivery ID
	Payload            map[string]any // Raw event payload
	RawPayload         []byte         // Raw JSON payload
	Headers            http.Header    // HTTP headers
	Action             string         // Action (e.g. "opened", "closed", "edited")
	RepositoryID       int64          // Repository ID
	RepositoryName     string         // Repository name
	RepositoryOwner    string         // Repository owner
	RepositoryFullName string         // Repository full name (owner/name)
	RefName            string         // Reference name (for push events)
	SenderLogin        string         // Sender login
	ReceivedAt         string         // Time when the event was received
}

// ParseEvent parses a webhook event from payload and headers
func ParseEvent(eventType string, payload []byte, headers http.Header) (Event, error) {
	event := Event{
		Type:       eventType,
		DeliveryID: headers.Get("X-GitHub-Delivery"),
		RawPayload: payload,
		Headers:    headers,
	}

	// Parse payload
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return event, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	event.Payload = data

	// Extract common fields
	if action, ok := data["action"].(string); ok {
		event.Action = action
	}

	// Extract repository information
	if repo, ok := data["repository"].(map[string]any); ok {
		if id, ok := repo["id"].(float64); ok {
			event.RepositoryID = int64(id)
		}
		if name, ok := repo["name"].(string); ok {
			event.RepositoryName = name
		}
		if fullName, ok := repo["full_name"].(string); ok {
			event.RepositoryFullName = fullName
		}
		if owner, ok := repo["owner"].(map[string]any); ok {
			if login, ok := owner["login"].(string); ok {
				event.RepositoryOwner = login
			}
		}
	}

	// Extract ref name for push events
	if ref, ok := data["ref"].(string); ok {
		// Convert refs/heads/master to master
		event.RefName = ref
		if len(ref) > 11 && ref[:11] == "refs/heads/" {
			event.RefName = ref[11:]
		}
	}

	// Extract ref name for pull requests
	if pr, ok := data["pull_request"].(map[string]any); ok {
		if base, ok := pr["base"].(map[string]any); ok {
			if ref, ok := base["ref"].(string); ok {
				event.RefName = ref
			}
		}
	}

	// Extract sender information
	if sender, ok := data["sender"].(map[string]any); ok {
		if login, ok := sender["login"].(string); ok {
			event.SenderLogin = login
		}
	}

	return event, nil
}

// DefaultEventHandlers returns a map of default event handlers
func DefaultEventHandlers() map[string]HandlerFunc {
	handlers := make(map[string]HandlerFunc)

	// Handle push events
	handlers["push"] = func(ctx context.Context, event Event) error {
		fmt.Printf("Received push event to %s\n", event.RefName)
		return nil
	}

	// Handle pull request events
	handlers["pull_request"] = func(ctx context.Context, event Event) error {
		fmt.Printf("Received pull request event with action %s\n", event.Action)
		return nil
	}

	// Handle issues events
	handlers["issues"] = func(ctx context.Context, event Event) error {
		fmt.Printf("Received issues event with action %s\n", event.Action)
		return nil
	}

	// Handle issue comment events
	handlers["issue_comment"] = func(ctx context.Context, event Event) error {
		fmt.Printf("Received issue comment event with action %s\n", event.Action)
		return nil
	}

	// Handle workflow run events
	handlers["workflow_run"] = func(ctx context.Context, event Event) error {
		fmt.Printf("Received workflow run event\n")
		return nil
	}

	// Handle workflow job events
	handlers["workflow_job"] = func(ctx context.Context, event Event) error {
		fmt.Printf("Received workflow job event\n")
		return nil
	}

	// Handle release events
	handlers["release"] = func(ctx context.Context, event Event) error {
		fmt.Printf("Received release event with action %s\n", event.Action)
		return nil
	}

	// Handle repository events
	handlers["repository"] = func(ctx context.Context, event Event) error {
		fmt.Printf("Received repository event with action %s\n", event.Action)
		return nil
	}

	// Handle ping events
	handlers["ping"] = func(ctx context.Context, event Event) error {
		fmt.Printf("Received ping event\n")
		return nil
	}

	return handlers
}
