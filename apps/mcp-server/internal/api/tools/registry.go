package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Tool represents a registered tool with its metadata
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Provider    string                 `json:"provider"`
	Enabled     bool                   `json:"enabled"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ToolHandler defines the interface for tool execution
type ToolHandler interface {
	Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)
	GetSchema() map[string]interface{}
	Validate(params map[string]interface{}) error
}

// Registry manages tool registration and execution
type Registry struct {
	tools    map[string]*Tool
	handlers map[string]ToolHandler
	logger   observability.Logger
	mu       sync.RWMutex
}

// NewRegistry creates a new tool registry
func NewRegistry(logger observability.Logger) *Registry {
	return &Registry{
		tools:    make(map[string]*Tool),
		handlers: make(map[string]ToolHandler),
		logger:   logger,
	}
}

// Register adds a new tool to the registry
func (r *Registry) Register(tool *Tool, handler ToolHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name is required")
	}

	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tool %s already registered", tool.Name)
	}

	if handler == nil {
		return fmt.Errorf("tool handler is required")
	}

	// Set schema from handler if not provided
	if tool.Schema == nil {
		tool.Schema = handler.GetSchema()
	}

	r.tools[tool.Name] = tool
	r.handlers[tool.Name] = handler

	r.logger.Info("Tool registered", map[string]interface{}{
		"tool":     tool.Name,
		"provider": tool.Provider,
		"version":  tool.Version,
	})

	return nil
}

// Unregister removes a tool from the registry
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return fmt.Errorf("tool %s not found", name)
	}

	delete(r.tools, name)
	delete(r.handlers, name)

	r.logger.Info("Tool unregistered", map[string]interface{}{
		"tool": name,
	})

	return nil
}

// Get returns a tool by name
func (r *Registry) Get(name string) (*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return tool, nil
}

// List returns all registered tools
func (r *Registry) List() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}

	return tools
}

// Execute runs a tool with the given parameters
func (r *Registry) Execute(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
	r.mu.RLock()
	tool, exists := r.tools[name]
	handler, handlerExists := r.handlers[name]
	r.mu.RUnlock()

	if !exists || !handlerExists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	if !tool.Enabled {
		return nil, fmt.Errorf("tool %s is disabled", name)
	}

	// Validate parameters
	if err := handler.Validate(params); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// Execute tool
	result, err := handler.Execute(ctx, params)
	if err != nil {
		r.logger.Error("Tool execution failed", map[string]interface{}{
			"tool":  name,
			"error": err.Error(),
		})
		return nil, err
	}

	r.logger.Info("Tool executed successfully", map[string]interface{}{
		"tool": name,
	})

	return result, nil
}

// Enable enables a tool
func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool, exists := r.tools[name]
	if !exists {
		return fmt.Errorf("tool %s not found", name)
	}

	tool.Enabled = true
	return nil
}

// Disable disables a tool
func (r *Registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool, exists := r.tools[name]
	if !exists {
		return fmt.Errorf("tool %s not found", name)
	}

	tool.Enabled = false
	return nil
}

// RegisterBuiltinTools registers all built-in tools
func (r *Registry) RegisterBuiltinTools() error {
	// Register GitHub tools
	githubTool := &Tool{
		Name:        "github",
		Description: "GitHub integration for repository operations",
		Version:     "1.0.0",
		Provider:    "github",
		Enabled:     true,
		Metadata: map[string]interface{}{
			"category": "version_control",
			"author":   "devops-mcp",
		},
	}

	// Create a simple handler for demonstration
	githubHandler := &genericToolHandler{
		name: "github",
		schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Action to perform",
					"enum":        []string{"list_repos", "get_repo", "create_issue"},
				},
				"params": map[string]interface{}{
					"type":        "object",
					"description": "Action-specific parameters",
				},
			},
			"required": []string{"action"},
		},
	}

	if err := r.Register(githubTool, githubHandler); err != nil {
		return fmt.Errorf("failed to register GitHub tool: %w", err)
	}

	// Register Jira tools
	jiraTool := &Tool{
		Name:        "jira",
		Description: "Jira integration for issue tracking",
		Version:     "1.0.0",
		Provider:    "atlassian",
		Enabled:     true,
		Metadata: map[string]interface{}{
			"category": "issue_tracking",
			"author":   "devops-mcp",
		},
	}

	jiraHandler := &genericToolHandler{
		name: "jira",
		schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Action to perform",
					"enum":        []string{"list_issues", "create_issue", "update_issue"},
				},
				"params": map[string]interface{}{
					"type":        "object",
					"description": "Action-specific parameters",
				},
			},
			"required": []string{"action"},
		},
	}

	if err := r.Register(jiraTool, jiraHandler); err != nil {
		return fmt.Errorf("failed to register Jira tool: %w", err)
	}

	// Register Slack tools
	slackTool := &Tool{
		Name:        "slack",
		Description: "Slack integration for team communication",
		Version:     "1.0.0",
		Provider:    "slack",
		Enabled:     true,
		Metadata: map[string]interface{}{
			"category": "communication",
			"author":   "devops-mcp",
		},
	}

	slackHandler := &genericToolHandler{
		name: "slack",
		schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Action to perform",
					"enum":        []string{"send_message", "list_channels", "create_channel"},
				},
				"params": map[string]interface{}{
					"type":        "object",
					"description": "Action-specific parameters",
				},
			},
			"required": []string{"action"},
		},
	}

	if err := r.Register(slackTool, slackHandler); err != nil {
		return fmt.Errorf("failed to register Slack tool: %w", err)
	}

	return nil
}

// genericToolHandler is a simple implementation of ToolHandler for demonstration
type genericToolHandler struct {
	name   string
	schema map[string]interface{}
}

func (h *genericToolHandler) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// This is a placeholder implementation
	// In production, this would integrate with actual services
	return map[string]interface{}{
		"tool":   h.name,
		"status": "success",
		"result": "Tool execution simulated",
		"params": params,
	}, nil
}

func (h *genericToolHandler) GetSchema() map[string]interface{} {
	return h.schema
}

func (h *genericToolHandler) Validate(params map[string]interface{}) error {
	// Basic validation - check for required action field
	if _, ok := params["action"]; !ok {
		return fmt.Errorf("action parameter is required")
	}
	return nil
}
