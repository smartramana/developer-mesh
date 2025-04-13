package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/aws/aws-sdk-go-v2/aws"
	sdkconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// ModelProvider represents a provider of LLMs in Amazon Bedrock
type ModelProvider string

const (
	// Supported model providers
	Anthropic   ModelProvider = "anthropic"
	AI21        ModelProvider = "ai21"
	Cohere      ModelProvider = "cohere"
	Meta        ModelProvider = "meta"
	Mistral     ModelProvider = "mistral"
	Stability   ModelProvider = "stability"
	Amazon      ModelProvider = "amazon"
)

// Config holds configuration for the Amazon Bedrock adapter
type Config struct {
	Region             string        `mapstructure:"region"`
	Profile            string        `mapstructure:"profile"`
	DefaultModelID     string        `mapstructure:"default_model_id"`
	RequestTimeout     time.Duration `mapstructure:"request_timeout"`
	RetryMax           int           `mapstructure:"retry_max"`
	RetryDelay         time.Duration `mapstructure:"retry_delay"`
	EnableContextOps   bool          `mapstructure:"enable_context_ops"`
	EnableModelOps     bool          `mapstructure:"enable_model_ops"`
	EnableEmbeddings   bool          `mapstructure:"enable_embeddings"`
	DefaultMaxTokens   int           `mapstructure:"default_max_tokens"`
}

// ModelConfig defines configuration specific to a model provider
type ModelConfig struct {
	Provider          ModelProvider
	ContextWindowSize int
	DefaultParams     map[string]interface{}
}

// Adapter implements the adapter interface for Amazon Bedrock
type Adapter struct {
	adapters.BaseAdapter
	config        Config
	client        *bedrockruntime.Client
	subscribers   map[string][]func(interface{})
	healthStatus  string
	modelConfigs  map[string]ModelConfig
}

// NewAdapter creates a new Amazon Bedrock adapter
func NewAdapter(config Config) (*Adapter, error) {
	// Set default values if not provided
	if config.Region == "" {
		config.Region = "us-east-1"
	}
	if config.DefaultModelID == "" {
		config.DefaultModelID = "anthropic.claude-3-sonnet-20240229-v1:0"
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 120 * time.Second // LLMs can take time to respond
	}
	if config.RetryMax == 0 {
		config.RetryMax = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.DefaultMaxTokens == 0 {
		config.DefaultMaxTokens = 4096
	}

	adapter := &Adapter{
		BaseAdapter: adapters.BaseAdapter{
			RetryMax:   config.RetryMax,
			RetryDelay: config.RetryDelay,
		},
		config:       config,
		subscribers:  make(map[string][]func(interface{})),
		healthStatus: "initializing",
		modelConfigs: initializeModelConfigs(),
	}

	return adapter, nil
}

// initializeModelConfigs sets up the configurations for different model providers
func initializeModelConfigs() map[string]ModelConfig {
	configs := make(map[string]ModelConfig)
	
	// Anthropic models
	configs["anthropic.claude-3-opus-20240229-v1:0"] = ModelConfig{
		Provider:          Anthropic,
		ContextWindowSize: 200000,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"top_p": 0.9,
		},
	}
	configs["anthropic.claude-3-sonnet-20240229-v1:0"] = ModelConfig{
		Provider:          Anthropic,
		ContextWindowSize: 180000,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"top_p": 0.9,
		},
	}
	configs["anthropic.claude-3-haiku-20240307-v1:0"] = ModelConfig{
		Provider:          Anthropic,
		ContextWindowSize: 150000,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"top_p": 0.9,
		},
	}
	configs["anthropic.claude-instant-v1"] = ModelConfig{
		Provider:          Anthropic,
		ContextWindowSize: 100000,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"top_p": 0.9,
		},
	}
	
	// AI21 models
	configs["ai21.j2-mid-v1"] = ModelConfig{
		Provider:          AI21,
		ContextWindowSize: 8192,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"top_p": 0.9,
		},
	}
	configs["ai21.j2-ultra-v1"] = ModelConfig{
		Provider:          AI21,
		ContextWindowSize: 8192,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"top_p": 0.9,
		},
	}
	
	// Cohere models
	configs["cohere.command-text-v14"] = ModelConfig{
		Provider:          Cohere,
		ContextWindowSize: 4096,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"p": 0.9,
		},
	}
	configs["cohere.command-light-text-v14"] = ModelConfig{
		Provider:          Cohere,
		ContextWindowSize: 4096,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"p": 0.9,
		},
	}
	
	// Meta models
	configs["meta.llama2-13b-chat-v1"] = ModelConfig{
		Provider:          Meta,
		ContextWindowSize: 4096,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"top_p": 0.9,
		},
	}
	configs["meta.llama2-70b-chat-v1"] = ModelConfig{
		Provider:          Meta,
		ContextWindowSize: 4096,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"top_p": 0.9,
		},
	}
	
	// Mistral models
	configs["mistral.mistral-7b-instruct-v0:2"] = ModelConfig{
		Provider:          Mistral,
		ContextWindowSize: 32768,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"top_p": 0.9,
		},
	}
	
	// Amazon models
	configs["amazon.titan-text-express-v1"] = ModelConfig{
		Provider:          Amazon,
		ContextWindowSize: 8000,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"top_p": 0.9,
		},
	}
	configs["amazon.titan-text-lite-v1"] = ModelConfig{
		Provider:          Amazon,
		ContextWindowSize: 4000,
		DefaultParams: map[string]interface{}{
			"temperature": 0.7,
			"top_p": 0.9,
		},
	}
	
	return configs
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

	// Create AWS configuration
	var awsConfig aws.Config
	var err error
	
	if a.config.Profile != "" {
		// Load config with specific profile
		awsConfig, err = sdkconfig.LoadDefaultConfig(ctx,
			sdkconfig.WithRegion(a.config.Region),
			sdkconfig.WithSharedConfigProfile(a.config.Profile),
		)
	} else {
		// Load default config
		awsConfig, err = sdkconfig.LoadDefaultConfig(ctx,
			sdkconfig.WithRegion(a.config.Region),
		)
	}
	
	if err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
		return err
	}

	// Create the Bedrock runtime client
	a.client = bedrockruntime.NewFromConfig(awsConfig)

	// Test the connection
	if err := a.testConnection(ctx); err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
		return err
	}

	a.healthStatus = "healthy"
	return nil
}

// testConnection tests the connection to Amazon Bedrock
func (a *Adapter) testConnection(ctx context.Context) error {
	// Use a dummy request to test connectivity
	// We'll invoke the smallest/fastest model available for this test
	testModel := "amazon.titan-text-lite-v1"
	
	// Create a simple prompt
	requestBody := map[string]interface{}{
		"inputText": "hello",
		"textGenerationConfig": map[string]interface{}{
			"maxTokenCount": 1,
			"temperature": 0,
		},
	}
	
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	
	// Setup the request
	input := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(testModel),
		Body:        jsonBody,
		ContentType: aws.String("application/json"),
	}
	
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	// Send the request
	_, err = a.client.InvokeModel(timeoutCtx, input)
	if err != nil {
		return fmt.Errorf("Bedrock connectivity test failed: %v", err)
	}
	
	return nil
}

// Health returns the health status of the adapter
func (a *Adapter) Health() string {
	return a.healthStatus
}

// Close gracefully shuts down the adapter
func (a *Adapter) Close() error {
	// Nothing to clean up for AWS SDK client
	return nil
}

// Subscribe registers a callback for a specific event type
func (a *Adapter) Subscribe(eventType string, callback func(interface{})) error {
	a.subscribers[eventType] = append(a.subscribers[eventType], callback)
	return nil
}

// HandleWebhook processes webhook events from Amazon (if any)
func (a *Adapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// Process the webhook payload
	var event map[string]interface{}
	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	// Notify subscribers
	if callbacks, ok := a.subscribers[eventType]; ok {
		for _, callback := range callbacks {
			callback(event)
		}
	}

	// Also notify subscribers of "all" events
	if callbacks, ok := a.subscribers["all"]; ok {
		for _, callback := range callbacks {
			callback(event)
		}
	}

	return nil
}

// GetData retrieves data from the Amazon Bedrock API
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
		if !a.config.EnableEmbeddings {
			return nil, fmt.Errorf("embeddings are not enabled for this adapter")
		}
		return a.generateEmbeddings(ctx, queryMap)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// getModels returns the list of available models
func (a *Adapter) getModels(ctx context.Context) (interface{}, error) {
	// In a real implementation, we would call the Bedrock ListFoundationModels API
	// For now, we'll return the predefined list of supported models
	models := make([]map[string]interface{}, 0, len(a.modelConfigs))
	
	for modelId, config := range a.modelConfigs {
		models = append(models, map[string]interface{}{
			"modelId": modelId,
			"provider": string(config.Provider),
			"contextWindow": config.ContextWindowSize,
		})
	}
	
	return map[string]interface{}{
		"models": models,
	}, nil
}

// getModelDetails returns details about a specific model
func (a *Adapter) getModelDetails(ctx context.Context, modelID string) (interface{}, error) {
	config, ok := a.modelConfigs[modelID]
	if !ok {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}
	
	return map[string]interface{}{
		"modelId": modelID,
		"provider": string(config.Provider),
		"contextWindow": config.ContextWindowSize,
		"defaultParams": config.DefaultParams,
	}, nil
}

// generateCompletion generates a text completion from the specified model
func (a *Adapter) generateCompletion(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Get model ID
	modelID, ok := params["model"].(string)
	if !ok {
		modelID = a.config.DefaultModelID
	}
	
	// Get model configuration
	modelConfig, ok := a.modelConfigs[modelID]
	if !ok {
		return nil, fmt.Errorf("unsupported model: %s", modelID)
	}
	
	// Prepare the request based on the model provider
	var jsonBody []byte
	var err error
	
	switch modelConfig.Provider {
	case Anthropic:
		jsonBody, err = a.prepareAnthropicRequest(params, modelConfig)
	case AI21:
		jsonBody, err = a.prepareAI21Request(params, modelConfig)
	case Cohere:
		jsonBody, err = a.prepareCohereRequest(params, modelConfig)
	case Meta:
		jsonBody, err = a.prepareMetaRequest(params, modelConfig)
	case Mistral:
		jsonBody, err = a.prepareMistralRequest(params, modelConfig)
	case Amazon:
		jsonBody, err = a.prepareAmazonRequest(params, modelConfig)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", modelConfig.Provider)
	}
	
	if err != nil {
		return nil, err
	}
	
	// Set up the request
	input := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		Body:        jsonBody,
		ContentType: aws.String("application/json"),
	}
	
	// Send the request with retry logic
	var response *bedrockruntime.InvokeModelOutput
	err = a.CallWithRetry(func() error {
		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, a.config.RequestTimeout)
		defer cancel()
		
		var err error
		response, err = a.client.InvokeModel(timeoutCtx, input)
		return err
	})
	
	if err != nil {
		return nil, err
	}
	
	// Parse the response based on the model provider
	var result interface{}
	
	switch modelConfig.Provider {
	case Anthropic:
		result, err = a.parseAnthropicResponse(response)
	case AI21:
		result, err = a.parseAI21Response(response)
	case Cohere:
		result, err = a.parseCohereResponse(response)
	case Meta:
		result, err = a.parseMetaResponse(response)
	case Mistral:
		result, err = a.parseMistralResponse(response)
	case Amazon:
		result, err = a.parseAmazonResponse(response)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", modelConfig.Provider)
	}
	
	if err != nil {
		return nil, err
	}
	
	// Convert to MCP ModelResponse format
	mcpResponse := a.convertToMCPResponse(modelID, result)
	
	return mcpResponse, nil
}

// parseAnthropicResponse parses the response from Anthropic models
func (a *Adapter) parseAnthropicResponse(response *bedrockruntime.InvokeModelOutput) (interface{}, error) {
	var anthropicResponse map[string]interface{}
	if err := json.Unmarshal(response.Body, &anthropicResponse); err != nil {
		return nil, err
	}
	
	// Extract the generated text
	content, ok := anthropicResponse["content"].([]interface{})
	if !ok || len(content) == 0 {
		return nil, fmt.Errorf("invalid or empty content in Anthropic response")
	}
	
	// Extract the first content block
	contentBlock, ok := content[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid content block in Anthropic response")
	}
	
	// Get the text
	text, ok := contentBlock["text"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid text in Anthropic response")
	}
	
	// Get usage information if available
	usage := map[string]interface{}{}
	if usageInfo, ok := anthropicResponse["usage"].(map[string]interface{}); ok {
		usage = usageInfo
	}
	
	return map[string]interface{}{
		"completion": text,
		"usage": usage,
		"raw_response": anthropicResponse,
	}, nil
}

// parseAI21Response parses the response from AI21 models
func (a *Adapter) parseAI21Response(response *bedrockruntime.InvokeModelOutput) (interface{}, error) {
	var ai21Response map[string]interface{}
	if err := json.Unmarshal(response.Body, &ai21Response); err != nil {
		return nil, err
	}
	
	// Extract the generated text
	completions, ok := ai21Response["completions"].([]interface{})
	if !ok || len(completions) == 0 {
		return nil, fmt.Errorf("invalid or empty completions in AI21 response")
	}
	
	// Get the first completion
	completion, ok := completions[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid completion in AI21 response")
	}
	
	// Get the text
	data, ok := completion["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data in AI21 completion")
	}
	
	text, ok := data["text"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid text in AI21 completion")
	}
	
	return map[string]interface{}{
		"completion": text,
		"raw_response": ai21Response,
	}, nil
}

// parseCohereResponse parses the response from Cohere models
func (a *Adapter) parseCohereResponse(response *bedrockruntime.InvokeModelOutput) (interface{}, error) {
	var cohereResponse map[string]interface{}
	if err := json.Unmarshal(response.Body, &cohereResponse); err != nil {
		return nil, err
	}
	
	// Extract the generated text
	generations, ok := cohereResponse["generations"].([]interface{})
	if !ok || len(generations) == 0 {
		return nil, fmt.Errorf("invalid or empty generations in Cohere response")
	}
	
	// Get the first generation
	generation, ok := generations[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid generation in Cohere response")
	}
	
	// Get the text
	text, ok := generation["text"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid text in Cohere generation")
	}
	
	return map[string]interface{}{
		"completion": text,
		"raw_response": cohereResponse,
	}, nil
}

// parseMetaResponse parses the response from Meta models
func (a *Adapter) parseMetaResponse(response *bedrockruntime.InvokeModelOutput) (interface{}, error) {
	var metaResponse map[string]interface{}
	if err := json.Unmarshal(response.Body, &metaResponse); err != nil {
		return nil, err
	}
	
	// Extract the generated text
	generation, ok := metaResponse["generation"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid generation in Meta response")
	}
	
	return map[string]interface{}{
		"completion": generation,
		"raw_response": metaResponse,
	}, nil
}

// parseMistralResponse parses the response from Mistral models
func (a *Adapter) parseMistralResponse(response *bedrockruntime.InvokeModelOutput) (interface{}, error) {
	var mistralResponse map[string]interface{}
	if err := json.Unmarshal(response.Body, &mistralResponse); err != nil {
		return nil, err
	}
	
	// Extract the generated text
	outputs, ok := mistralResponse["outputs"].([]interface{})
	if !ok || len(outputs) == 0 {
		return nil, fmt.Errorf("invalid or empty outputs in Mistral response")
	}
	
	// Get the first output
	output, ok := outputs[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid output in Mistral response")
	}
	
	// Get the text
	text, ok := output["text"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid text in Mistral output")
	}
	
	return map[string]interface{}{
		"completion": text,
		"raw_response": mistralResponse,
	}, nil
}

// parseAmazonResponse parses the response from Amazon models
func (a *Adapter) parseAmazonResponse(response *bedrockruntime.InvokeModelOutput) (interface{}, error) {
	var amazonResponse map[string]interface{}
	if err := json.Unmarshal(response.Body, &amazonResponse); err != nil {
		return nil, err
	}
	
	// Extract the generated text
	results, ok := amazonResponse["results"].([]interface{})
	if !ok || len(results) == 0 {
		// Try alternative response format
		if outputText, ok := amazonResponse["outputText"].(string); ok {
			return map[string]interface{}{
				"completion": outputText,
				"raw_response": amazonResponse,
			}, nil
		}
		return nil, fmt.Errorf("invalid or empty results in Amazon response")
	}
	
	// Get the first result
	result, ok := results[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result in Amazon response")
	}
	
	// Get the text
	outputText, ok := result["outputText"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid outputText in Amazon result")
	}
	
	return map[string]interface{}{
		"completion": outputText,
		"raw_response": amazonResponse,
	}, nil
}

// convertToMCPResponse converts the provider-specific response to the MCP format
func (a *Adapter) convertToMCPResponse(modelID string, result interface{}) interface{} {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return result
	}
	
	// Create an MCP-formatted response
	mcpResponse := map[string]interface{}{
		"model": modelID,
	}
	
	// Copy relevant fields
	if completion, ok := resultMap["completion"].(string); ok {
		mcpResponse["completion"] = completion
	}
	
	if usage, ok := resultMap["usage"].(map[string]interface{}); ok {
		mcpResponse["usage"] = usage
	}
	
	// Add raw response if available
	if rawResponse, ok := resultMap["raw_response"].(map[string]interface{}); ok {
		mcpResponse["provider_response"] = rawResponse
	}
	
	return mcpResponse
}

// generateEmbeddings generates embeddings for the provided text
func (a *Adapter) generateEmbeddings(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Get the text to embed
	var text string
	if textParam, ok := params["text"].(string); ok {
		text = textParam
	} else if inputParam, ok := params["input"].(string); ok {
		text = inputParam
	} else {
		return nil, fmt.Errorf("missing text parameter for embedding generation")
	}
	
	// Get model ID, default to a suitable embedding model if not specified
	modelID, ok := params["model"].(string)
	if !ok {
		modelID = "amazon.titan-embed-text-v1" // Default embedding model
	}
	
	// Prepare the request
	requestBody := map[string]interface{}{
		"inputText": text,
	}
	
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	
	// Set up the request
	input := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		Body:        jsonBody,
		ContentType: aws.String("application/json"),
	}
	
	// Send the request with retry logic
	var response *bedrockruntime.InvokeModelOutput
	err = a.CallWithRetry(func() error {
		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, a.config.RequestTimeout)
		defer cancel()
		
		var err error
		response, err = a.client.InvokeModel(timeoutCtx, input)
		return err
	})
	
	if err != nil {
		return nil, err
	}
	
	// Parse the response
	var embedResponse map[string]interface{}
	if err := json.Unmarshal(response.Body, &embedResponse); err != nil {
		return nil, err
	}
	
	// Extract embedding vector (the structure varies by model)
	var embedding []float64
	if amazonResponse, ok := embedResponse["embedding"].([]interface{}); ok {
		// Amazon Titan embeddings
		embedding = make([]float64, len(amazonResponse))
		for i, v := range amazonResponse {
			if f, ok := v.(float64); ok {
				embedding[i] = f
			}
		}
	} else if cohereResponse, ok := embedResponse["embeddings"].([]interface{}); ok && len(cohereResponse) > 0 {
		// Cohere embeddings
		if cohereVector, ok := cohereResponse[0].([]interface{}); ok {
			embedding = make([]float64, len(cohereVector))
			for i, v := range cohereVector {
				if f, ok := v.(float64); ok {
					embedding[i] = f
				}
			}
		}
	} else {
		return nil, fmt.Errorf("could not parse embedding from response")
	}
	
	return map[string]interface{}{
		"model": modelID,
		"embedding": embedding,
		"dimensions": len(embedding),
	}, nil
}

// prepareAnthropicRequest prepares a request for Anthropic models
func (a *Adapter) prepareAnthropicRequest(params map[string]interface{}, config ModelConfig) ([]byte, error) {
	// Convert messages to Anthropic format
	anthropicRequest := map[string]interface{}{}
	
	// Get messages from context items if provided
	var messages []interface{}
	
	if contextItems, ok := params["messages"].([]mcp.ContextItem); ok {
		for _, item := range contextItems {
			messages = append(messages, map[string]string{
				"role":    item.Role,
				"content": item.Content,
			})
		}
	} else if rawMessages, ok := params["messages"].([]interface{}); ok {
		messages = rawMessages
	} else {
		return nil, fmt.Errorf("missing or invalid messages parameter")
	}
	
	// Convert messages to Anthropic format
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}
	
	// Extract system message if present
	var systemPrompt string
	var nonSystemMessages []map[string]interface{}
	
	for _, msg := range messages {
		if msgMap, ok := msg.(map[string]interface{}); ok {
			if role, ok := msgMap["role"].(string); ok {
				if role == "system" {
					if content, ok := msgMap["content"].(string); ok {
						systemPrompt = content
					}
				} else {
					nonSystemMessages = append(nonSystemMessages, msgMap)
				}
			}
		} else if msgMap, ok := msg.(map[string]string); ok {
			if msgMap["role"] == "system" {
				systemPrompt = msgMap["content"]
			} else {
				nonSystemMessages = append(nonSystemMessages, map[string]interface{}{
					"role":    msgMap["role"],
					"content": msgMap["content"],
				})
			}
		}
	}
	
	// Structure request for Anthropic
	anthropicRequest["anthropic_version"] = "bedrock-2023-05-31"
	
	if systemPrompt != "" {
		anthropicRequest["system"] = systemPrompt
	}
	
	// Add messages
	anthropicRequest["messages"] = nonSystemMessages
	
	// Add parameters from the request or use defaults
	if temperature, ok := params["temperature"].(float64); ok {
		anthropicRequest["temperature"] = temperature
	} else if temp, ok := config.DefaultParams["temperature"].(float64); ok {
		anthropicRequest["temperature"] = temp
	}
	
	if topP, ok := params["top_p"].(float64); ok {
		anthropicRequest["top_p"] = topP
	} else if tp, ok := config.DefaultParams["top_p"].(float64); ok {
		anthropicRequest["top_p"] = tp
	}
	
	// Set max tokens
	if maxTokens, ok := params["max_tokens"].(int); ok {
		anthropicRequest["max_tokens"] = maxTokens
	} else {
		anthropicRequest["max_tokens"] = a.config.DefaultMaxTokens
	}
	
	return json.Marshal(anthropicRequest)
}

// prepareAI21Request prepares a request for AI21 models
func (a *Adapter) prepareAI21Request(params map[string]interface{}, config ModelConfig) ([]byte, error) {
	// Convert to AI21 format
	ai21Request := map[string]interface{}{}
	
	// Handle messages or prompt
	var prompt string
	
	if contextItems, ok := params["messages"].([]mcp.ContextItem); ok {
		// Convert chat messages to prompt
		var promptBuilder strings.Builder
		
		for _, item := range contextItems {
			if item.Role == "system" {
				promptBuilder.WriteString("System: ")
			} else if item.Role == "user" {
				promptBuilder.WriteString("Human: ")
			} else if item.Role == "assistant" {
				promptBuilder.WriteString("Assistant: ")
			}
			
			promptBuilder.WriteString(item.Content)
			promptBuilder.WriteString("\n\n")
		}
		
		promptBuilder.WriteString("Assistant: ")
		prompt = promptBuilder.String()
	} else if rawPrompt, ok := params["prompt"].(string); ok {
		prompt = rawPrompt
	} else if rawMessages, ok := params["messages"].([]interface{}); ok {
		// Convert interface messages to prompt
		var promptBuilder strings.Builder
		
		for _, msg := range rawMessages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				role, _ := msgMap["role"].(string)
				content, _ := msgMap["content"].(string)
				
				if role == "system" {
					promptBuilder.WriteString("System: ")
				} else if role == "user" {
					promptBuilder.WriteString("Human: ")
				} else if role == "assistant" {
					promptBuilder.WriteString("Assistant: ")
				}
				
				promptBuilder.WriteString(content)
				promptBuilder.WriteString("\n\n")
			}
		}
		
		promptBuilder.WriteString("Assistant: ")
		prompt = promptBuilder.String()
	} else {
		return nil, fmt.Errorf("missing prompt or messages parameter")
	}
	
	ai21Request["prompt"] = prompt
	
	// Add parameters
	if temperature, ok := params["temperature"].(float64); ok {
		ai21Request["temperature"] = temperature
	} else if temp, ok := config.DefaultParams["temperature"].(float64); ok {
		ai21Request["temperature"] = temp
	}
	
	if topP, ok := params["top_p"].(float64); ok {
		ai21Request["topP"] = topP
	} else if tp, ok := config.DefaultParams["top_p"].(float64); ok {
		ai21Request["topP"] = tp
	}
	
	// Set max tokens
	if maxTokens, ok := params["max_tokens"].(int); ok {
		ai21Request["maxTokens"] = maxTokens
	} else {
		ai21Request["maxTokens"] = a.config.DefaultMaxTokens
	}
	
	return json.Marshal(ai21Request)
}

// prepareCohereRequest prepares a request for Cohere models
func (a *Adapter) prepareCohereRequest(params map[string]interface{}, config ModelConfig) ([]byte, error) {
	// Convert to Cohere format
	cohereRequest := map[string]interface{}{}
	
	// Handle messages or prompt
	if contextItems, ok := params["messages"].([]mcp.ContextItem); ok {
		// Convert chat messages to prompt
		var promptBuilder strings.Builder
		var chatHistory []map[string]string
		
		for _, item := range contextItems {
			if item.Role == "user" {
				chatHistory = append(chatHistory, map[string]string{
					"role": "USER",
					"message": item.Content,
				})
			} else if item.Role == "assistant" {
				chatHistory = append(chatHistory, map[string]string{
					"role": "CHATBOT",
					"message": item.Content,
				})
			} else if item.Role == "system" {
				// Handle system message
				promptBuilder.WriteString(item.Content)
			}
		}
		
		if len(chatHistory) > 0 {
			cohereRequest["chat_history"] = chatHistory
		}
		
		if promptBuilder.Len() > 0 {
			cohereRequest["preamble"] = promptBuilder.String()
		}
		
		// Get the last user message as the message
		if len(chatHistory) > 0 && chatHistory[len(chatHistory)-1]["role"] == "USER" {
			cohereRequest["message"] = chatHistory[len(chatHistory)-1]["message"]
			// Remove from chat history
			chatHistory = chatHistory[:len(chatHistory)-1]
		}
	} else if rawPrompt, ok := params["prompt"].(string); ok {
		cohereRequest["message"] = rawPrompt
	} else if rawMessages, ok := params["messages"].([]interface{}); ok {
		// Convert interface messages
		var promptBuilder strings.Builder
		var chatHistory []map[string]string
		
		for _, msg := range rawMessages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				role, _ := msgMap["role"].(string)
				content, _ := msgMap["content"].(string)
				
				if role == "user" {
					chatHistory = append(chatHistory, map[string]string{
						"role": "USER",
						"message": content,
					})
				} else if role == "assistant" {
					chatHistory = append(chatHistory, map[string]string{
						"role": "CHATBOT",
						"message": content,
					})
				} else if role == "system" {
					promptBuilder.WriteString(content)
				}
			}
		}
		
		if len(chatHistory) > 0 {
			cohereRequest["chat_history"] = chatHistory
		}
		
		if promptBuilder.Len() > 0 {
			cohereRequest["preamble"] = promptBuilder.String()
		}
		
		// Get the last user message as the message
		if len(chatHistory) > 0 && chatHistory[len(chatHistory)-1]["role"] == "USER" {
			cohereRequest["message"] = chatHistory[len(chatHistory)-1]["message"]
			// Remove from chat history
			chatHistory = chatHistory[:len(chatHistory)-1]
		}
	} else {
		return nil, fmt.Errorf("missing prompt or messages parameter")
	}
	
	// Add parameters
	if temperature, ok := params["temperature"].(float64); ok {
		cohereRequest["temperature"] = temperature
	} else if temp, ok := config.DefaultParams["temperature"].(float64); ok {
		cohereRequest["temperature"] = temp
	}
	
	if topP, ok := params["top_p"].(float64); ok {
		cohereRequest["p"] = topP
	} else if p, ok := config.DefaultParams["p"].(float64); ok {
		cohereRequest["p"] = p
	}
	
	// Set max tokens
	if maxTokens, ok := params["max_tokens"].(int); ok {
		cohereRequest["max_tokens"] = maxTokens
	} else {
		cohereRequest["max_tokens"] = a.config.DefaultMaxTokens
	}
	
	return json.Marshal(cohereRequest)
}

// prepareMetaRequest prepares a request for Meta models
func (a *Adapter) prepareMetaRequest(params map[string]interface{}, config ModelConfig) ([]byte, error) {
	// Convert to Llama format
	llamaRequest := map[string]interface{}{}
	
	// Handle messages or prompt
	var prompt string
	
	if contextItems, ok := params["messages"].([]mcp.ContextItem); ok {
		// Convert chat messages to Llama format
		var promptBuilder strings.Builder
		
		for _, item := range contextItems {
			if item.Role == "system" {
				promptBuilder.WriteString("<s>\n")
				promptBuilder.WriteString(item.Content)
				promptBuilder.WriteString("\n</s>\n")
			} else if item.Role == "user" {
				promptBuilder.WriteString("<human>: ")
				promptBuilder.WriteString(item.Content)
				promptBuilder.WriteString("\n")
			} else if item.Role == "assistant" {
				promptBuilder.WriteString("<assistant>: ")
				promptBuilder.WriteString(item.Content)
				promptBuilder.WriteString("\n")
			}
		}
		
		// Add final assistant prompt
		promptBuilder.WriteString("<assistant>: ")
		prompt = promptBuilder.String()
	} else if rawPrompt, ok := params["prompt"].(string); ok {
		prompt = rawPrompt
	} else if rawMessages, ok := params["messages"].([]interface{}); ok {
		// Convert interface messages
		var promptBuilder strings.Builder
		
		for _, msg := range rawMessages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				role, _ := msgMap["role"].(string)
				content, _ := msgMap["content"].(string)
				
				if role == "system" {
					promptBuilder.WriteString("<s>\n")
					promptBuilder.WriteString(content)
					promptBuilder.WriteString("\n</s>\n")
				} else if role == "user" {
					promptBuilder.WriteString("<human>: ")
					promptBuilder.WriteString(content)
					promptBuilder.WriteString("\n")
				} else if role == "assistant" {
					promptBuilder.WriteString("<assistant>: ")
					promptBuilder.WriteString(content)
					promptBuilder.WriteString("\n")
				}
			}
		}
		
		// Add final assistant prompt
		promptBuilder.WriteString("<assistant>: ")
		prompt = promptBuilder.String()
	} else {
		return nil, fmt.Errorf("missing prompt or messages parameter")
	}
	
	llamaRequest["prompt"] = prompt
	
	// Add parameters
	if temperature, ok := params["temperature"].(float64); ok {
		llamaRequest["temperature"] = temperature
	} else if temp, ok := config.DefaultParams["temperature"].(float64); ok {
		llamaRequest["temperature"] = temp
	}
	
	if topP, ok := params["top_p"].(float64); ok {
		llamaRequest["top_p"] = topP
	} else if tp, ok := config.DefaultParams["top_p"].(float64); ok {
		llamaRequest["top_p"] = tp
	}
	
	// Set max tokens
	if maxTokens, ok := params["max_tokens"].(int); ok {
		llamaRequest["max_gen_len"] = maxTokens
	} else {
		llamaRequest["max_gen_len"] = a.config.DefaultMaxTokens
	}
	
	return json.Marshal(llamaRequest)
}

// prepareAmazonRequest prepares a request for Amazon models
func (a *Adapter) prepareAmazonRequest(params map[string]interface{}, config ModelConfig) ([]byte, error) {
	// Convert to Amazon Titan format
	amazonRequest := map[string]interface{}{}
	
	// Handle messages or prompt
	var inputText string
	
	if contextItems, ok := params["messages"].([]mcp.ContextItem); ok {
		// Convert chat messages to text
		var promptBuilder strings.Builder
		
		for _, item := range contextItems {
			if item.Role == "system" {
				promptBuilder.WriteString("System: ")
				promptBuilder.WriteString(item.Content)
				promptBuilder.WriteString("\n\n")
			} else if item.Role == "user" {
				promptBuilder.WriteString("Human: ")
				promptBuilder.WriteString(item.Content)
				promptBuilder.WriteString("\n\n")
			} else if item.Role == "assistant" {
				promptBuilder.WriteString("Assistant: ")
				promptBuilder.WriteString(item.Content)
				promptBuilder.WriteString("\n\n")
			}
		}
		
		promptBuilder.WriteString("Assistant: ")
		inputText = promptBuilder.String()
	} else if rawPrompt, ok := params["prompt"].(string); ok {
		inputText = rawPrompt
	} else if rawMessages, ok := params["messages"].([]interface{}); ok {
		// Convert interface messages
		var promptBuilder strings.Builder
		
		for _, msg := range rawMessages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				role, _ := msgMap["role"].(string)
				content, _ := msgMap["content"].(string)
				
				if role == "system" {
					promptBuilder.WriteString("System: ")
					promptBuilder.WriteString(content)
					promptBuilder.WriteString("\n\n")
				} else if role == "user" {
					promptBuilder.WriteString("Human: ")
					promptBuilder.WriteString(content)
					promptBuilder.WriteString("\n\n")
				} else if role == "assistant" {
					promptBuilder.WriteString("Assistant: ")
					promptBuilder.WriteString(content)
					promptBuilder.WriteString("\n\n")
				}
			}
		}
		
		promptBuilder.WriteString("Assistant: ")
		inputText = promptBuilder.String()
	} else {
		return nil, fmt.Errorf("missing prompt or messages parameter")
	}
	
	amazonRequest["inputText"] = inputText
	
	// Set up text generation config
	textGenConfig := map[string]interface{}{}
	
	// Add parameters
	if temperature, ok := params["temperature"].(float64); ok {
		textGenConfig["temperature"] = temperature
	} else if temp, ok := config.DefaultParams["temperature"].(float64); ok {
		textGenConfig["temperature"] = temp
	}
	
	if topP, ok := params["top_p"].(float64); ok {
		textGenConfig["topP"] = topP
	} else if tp, ok := config.DefaultParams["top_p"].(float64); ok {
		textGenConfig["topP"] = tp
	}
	
	// Set max tokens
	if maxTokens, ok := params["max_tokens"].(int); ok {
		textGenConfig["maxTokenCount"] = maxTokens
	} else {
		textGenConfig["maxTokenCount"] = a.config.DefaultMaxTokens
	}
	
	amazonRequest["textGenerationConfig"] = textGenConfig
	
	return json.Marshal(amazonRequest)
}

// prepareMistralRequest prepares a request for Mistral models
func (a *Adapter) prepareMistralRequest(params map[string]interface{}, config ModelConfig) ([]byte, error) {
	// Convert to Mistral format
	mistralRequest := map[string]interface{}{}
	
	// Handle messages or prompt
	if contextItems, ok := params["messages"].([]mcp.ContextItem); ok {
		// Convert chat messages to Mistral format
		messages := []map[string]string{}
		
		for _, item := range contextItems {
			// Map role names to Mistral format
			role := item.Role
			if role == "user" {
				role = "user"
			} else if role == "assistant" {
				role = "assistant"
			} else if role == "system" {
				role = "system"
			}
			
			messages = append(messages, map[string]string{
				"role":    role,
				"content": item.Content,
			})
		}
		
		mistralRequest["messages"] = messages
	} else if rawPrompt, ok := params["prompt"].(string); ok {
		// Single prompt as user message
		mistralRequest["messages"] = []map[string]string{
			{
				"role":    "user",
				"content": rawPrompt,
			},
		}
	} else if rawMessages, ok := params["messages"].([]interface{}); ok {
		// Convert interface messages
		messages := []map[string]string{}
		
		for _, msg := range rawMessages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				role, _ := msgMap["role"].(string)
				content, _ := msgMap["content"].(string)
				
				// Map role names
				if role == "user" {
					role = "user"
				} else if role == "assistant" {
					role = "assistant"
				} else if role == "system" {
					role = "system"
				}
				
				messages = append(messages, map[string]string{
					"role":    role,
					"content": content,
				})
			}
		}
		
		mistralRequest["messages"] = messages
	} else {
		return nil, fmt.Errorf("missing prompt or messages parameter")
	}
	
	// Add parameters
	if temperature, ok := params["temperature"].(float64); ok {
		mistralRequest["temperature"] = temperature
	} else if temp, ok := config.DefaultParams["temperature"].(float64); ok {
		mistralRequest["temperature"] = temp
	}
	
	if topP, ok := params["top_p"].(float64); ok {
		mistralRequest["top_p"] = topP
	} else if tp, ok := config.DefaultParams["top_p"].(float64); ok {
		mistralRequest["top_p"] = tp
	}
	
	// Set max tokens
	if maxTokens, ok := params["max_tokens"].(int); ok {
		mistralRequest["max_tokens"] = maxTokens
	} else {
		mistralRequest["max_tokens"] = a.config.DefaultMaxTokens
	}
	
	return json.Marshal(mistralRequest)
}
