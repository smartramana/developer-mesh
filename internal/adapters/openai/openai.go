package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

// Config holds configuration for the OpenAI adapter
type Config struct {
	APIKey            string        `mapstructure:"api_key"`
	OrganizationID    string        `mapstructure:"organization_id"`
	APIBaseURL        string        `mapstructure:"api_base_url"`
	DefaultModel      string        `mapstructure:"default_model"`
	RequestTimeout    time.Duration `mapstructure:"request_timeout"`
	RetryMax          int           `mapstructure:"retry_max"`
	RetryDelay        time.Duration `mapstructure:"retry_delay"`
	WebhookSecret     string        `mapstructure:"webhook_secret"`
	TokenCounterURL   string        `mapstructure:"token_counter_url"`
	EnableContextOps  bool          `mapstructure:"enable_context_ops"`
	EnableModelOps    bool          `mapstructure:"enable_model_ops"`
	EnableEmbeddings  bool          `mapstructure:"enable_embeddings"`
	ContextWindowSize int           `mapstructure:"context_window_size"`
}

// Adapter implements the adapter interface for OpenAI
type Adapter struct {
	adapters.BaseAdapter
	config       Config
	client       *http.Client
	subscribers  map[string][]func(interface{})
	healthStatus string
}

// NewAdapter creates a new OpenAI adapter
func NewAdapter(config Config) (*Adapter, error) {
	// Set default values if not provided
	if config.APIBaseURL == "" {
		config.APIBaseURL = "https://api.openai.com/v1"
	}
	if config.DefaultModel == "" {
		config.DefaultModel = "gpt-4o"
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.RetryMax == 0 {
		config.RetryMax = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.ContextWindowSize == 0 {
		config.ContextWindowSize = 128000 // Default for GPT-4o
	}

	adapter := &Adapter{
		BaseAdapter: adapters.BaseAdapter{
			RetryMax:   config.RetryMax,
			RetryDelay: config.RetryDelay,
		},
		config: config,
		client: &http.Client{
			Timeout: config.RequestTimeout,
		},
		subscribers:  make(map[string][]func(interface{})),
		healthStatus: "initializing",
	}

	return adapter, nil
}

// Initialize sets up the adapter with configuration
func (a *Adapter) Initialize(ctx context.Context, config interface{}) error {
	// Parse config if provided
	if config != nil {
		cfg, ok := config.(Config)
		if !ok {
			return fmt.Errorf("invalid config type: %T", config)
		}
		a.config = cfg
	}

	// Validate the configuration
	if a.config.APIKey == "" {
		return fmt.Errorf("OpenAI API key is required")
	}

	// Test the connection to the API
	if err := a.testConnection(ctx); err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
		return err
	}

	a.healthStatus = "healthy"
	return nil
}

// testConnection tests the connection to the OpenAI API
func (a *Adapter) testConnection(ctx context.Context) error {
	// Create a simple request to the models endpoint to verify connectivity
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/models", a.config.APIBaseURL), nil)
	if err != nil {
		return err
	}

	// Add authentication headers
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", a.config.APIKey))
	req.Header.Add("Content-Type", "application/json")
	if a.config.OrganizationID != "" {
		req.Header.Add("OpenAI-Organization", a.config.OrganizationID)
	}

	// Send the request
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to connect to OpenAI API: %s", resp.Status)
	}

	return nil
}

// Health returns the health status of the adapter
func (a *Adapter) Health() string {
	return a.healthStatus
}

// Close gracefully shuts down the adapter
func (a *Adapter) Close() error {
	// Nothing to clean up for HTTP client
	return nil
}

// GetData retrieves data from OpenAI API
func (a *Adapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	// Parse the query
	queryMap, ok := query.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid query type: %T", query)
	}

	// Check the operation type
	operation, ok := queryMap["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("missing operation in query")
	}

	// Handle different operations
	switch operation {
	case "get_models":
		return a.getModels(ctx)
	case "get_model_details":
		modelID, ok := queryMap["model_id"].(string)
		if !ok {
			return nil, fmt.Errorf("missing model_id in query")
		}
		return a.getModelDetails(ctx, modelID)
	case "generate_completion":
		return a.generateCompletion(ctx, queryMap)
	case "generate_embeddings":
		return a.generateEmbeddings(ctx, queryMap)
	case "count_tokens":
		return a.countTokens(ctx, queryMap)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// getModels retrieves the list of available models
func (a *Adapter) getModels(ctx context.Context) (interface{}, error) {
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/models", a.config.APIBaseURL), nil)
	if err != nil {
		return nil, err
	}

	// Add authentication headers
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", a.config.APIKey))
	req.Header.Add("Content-Type", "application/json")
	if a.config.OrganizationID != "" {
		req.Header.Add("OpenAI-Organization", a.config.OrganizationID)
	}

	// Send the request
	var result map[string]interface{}
	err = a.CallWithRetry(func() error {
		resp, err := a.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Check for successful response
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to get models: %s", resp.Status)
		}

		// Parse the response
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// getModelDetails retrieves the details of a specific model
func (a *Adapter) getModelDetails(ctx context.Context, modelID string) (interface{}, error) {
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/models/%s", a.config.APIBaseURL, modelID), nil)
	if err != nil {
		return nil, err
	}

	// Add authentication headers
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", a.config.APIKey))
	req.Header.Add("Content-Type", "application/json")
	if a.config.OrganizationID != "" {
		req.Header.Add("OpenAI-Organization", a.config.OrganizationID)
	}

	// Send the request
	var result map[string]interface{}
	err = a.CallWithRetry(func() error {
		resp, err := a.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Check for successful response
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to get model details: %s", resp.Status)
		}

		// Parse the response
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// generateCompletion generates a completion from OpenAI
func (a *Adapter) generateCompletion(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Convert request parameters to OpenAI format
	openAIParams := map[string]interface{}{}

	// Add model
	model, ok := params["model"].(string)
	if !ok {
		model = a.config.DefaultModel
	}
	openAIParams["model"] = model

	// Add messages from context items if provided
	if contextItems, ok := params["messages"].([]mcp.ContextItem); ok {
		messages := []map[string]string{}
		for _, item := range contextItems {
			messages = append(messages, map[string]string{
				"role":    item.Role,
				"content": item.Content,
			})
		}
		openAIParams["messages"] = messages
	} else if messages, ok := params["messages"].([]interface{}); ok {
		openAIParams["messages"] = messages
	} else {
		return nil, fmt.Errorf("missing or invalid messages parameter")
	}

	// Add other parameters
	if temperature, ok := params["temperature"].(float64); ok {
		openAIParams["temperature"] = temperature
	}
	if maxTokens, ok := params["max_tokens"].(int); ok {
		openAIParams["max_tokens"] = maxTokens
	}
	if topP, ok := params["top_p"].(float64); ok {
		openAIParams["top_p"] = topP
	}
	if presencePenalty, ok := params["presence_penalty"].(float64); ok {
		openAIParams["presence_penalty"] = presencePenalty
	}
	if frequencyPenalty, ok := params["frequency_penalty"].(float64); ok {
		openAIParams["frequency_penalty"] = frequencyPenalty
	}
	if stop, ok := params["stop"].([]string); ok {
		openAIParams["stop"] = stop
	}

	// Convert to JSON
	jsonData, err := json.Marshal(openAIParams)
	if err != nil {
		return nil, err
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", a.config.APIBaseURL), strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}

	// Add authentication headers
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", a.config.APIKey))
	req.Header.Add("Content-Type", "application/json")
	if a.config.OrganizationID != "" {
		req.Header.Add("OpenAI-Organization", a.config.OrganizationID)
	}

	// Send the request
	var result map[string]interface{}
	err = a.CallWithRetry(func() error {
		resp, err := a.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Check for successful response
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to generate completion: %s", resp.Status)
		}

		// Parse the response
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert OpenAI response to MCP format
	response := &mcp.ModelResponse{
		ModelID: model,
	}

	// Extract response content
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					response.Content = content
				}
			}
		}
	}

	// Extract token usage
	if usage, ok := result["usage"].(map[string]interface{}); ok {
		if totalTokens, ok :=