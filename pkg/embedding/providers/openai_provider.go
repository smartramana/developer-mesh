package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// OpenAIProvider implements Provider interface for OpenAI embeddings
type OpenAIProvider struct {
	config     ProviderConfig
	httpClient *http.Client
	models     map[string]ModelInfo
	mu         sync.RWMutex
}

// openAIRequest represents the request structure for OpenAI API
type openAIRequest struct {
	Input          interface{} `json:"input"` // Can be string or []string
	Model          string      `json:"model"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
	Dimensions     *int        `json:"dimensions,omitempty"` // For models that support dimension reduction
	User           string      `json:"user,omitempty"`
}

// openAIResponse represents the response from OpenAI API
type openAIResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// openAIErrorResponse represents an error response from OpenAI
type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   string `json:"param,omitempty"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(config ProviderConfig) (*OpenAIProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	if config.Endpoint == "" {
		config.Endpoint = "https://api.openai.com/v1"
	}

	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	if config.RetryDelayBase == 0 {
		config.RetryDelayBase = time.Second
	}

	p := &OpenAIProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
	}

	// Initialize supported models
	p.models = map[string]ModelInfo{
		"text-embedding-3-small": {
			Name:                       "text-embedding-3-small",
			DisplayName:                "OpenAI Text Embedding 3 Small",
			Dimensions:                 1536,
			MaxTokens:                  8191,
			CostPer1MTokens:            0.02,
			SupportedTaskTypes:         []string{"default", "general_qa", "multilingual"},
			SupportsDimensionReduction: true,
			MinDimensions:              512,
			IsActive:                   true,
		},
		"text-embedding-3-large": {
			Name:                       "text-embedding-3-large",
			DisplayName:                "OpenAI Text Embedding 3 Large",
			Dimensions:                 3072,
			MaxTokens:                  8191,
			CostPer1MTokens:            0.13,
			SupportedTaskTypes:         []string{"default", "general_qa", "multilingual", "research"},
			SupportsDimensionReduction: true,
			MinDimensions:              256,
			IsActive:                   true,
		},
		"text-embedding-ada-002": {
			Name:               "text-embedding-ada-002",
			DisplayName:        "OpenAI Ada v2",
			Dimensions:         1536,
			MaxTokens:          8191,
			CostPer1MTokens:    0.10,
			SupportedTaskTypes: []string{"default", "general_qa"},
			IsActive:           true,
			DeprecatedAt:       parseTime("2025-12-31"), // Expected deprecation
		},
	}

	return p, nil
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// GenerateEmbedding generates an embedding for the given text
func (p *OpenAIProvider) GenerateEmbedding(ctx context.Context, req GenerateEmbeddingRequest) (*EmbeddingResponse, error) {
	model, err := p.GetModel(req.Model)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	// Prepare OpenAI request
	openAIReq := openAIRequest{
		Input: req.Text,
		Model: req.Model,
		User:  req.RequestID, // Use request ID as user identifier for tracking
	}

	// Handle dimension reduction for supported models
	if dims, ok := req.Metadata["dimensions"].(int); ok && model.SupportsDimensionReduction {
		if dims >= model.MinDimensions && dims <= model.Dimensions {
			openAIReq.Dimensions = &dims
		}
	}

	// Make the API call with retry logic
	var resp *openAIResponse
	var lastErr error

	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := p.calculateRetryDelay(attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, lastErr = p.doRequest(ctx, openAIReq)
		if lastErr == nil {
			break
		}

		// Check if error is retryable
		if !p.isRetryableError(lastErr) {
			break
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}

	latency := time.Since(start)

	// Build response
	dimensions := model.Dimensions
	if openAIReq.Dimensions != nil {
		dimensions = *openAIReq.Dimensions
	}

	return &EmbeddingResponse{
		Embedding:  resp.Data[0].Embedding,
		Model:      resp.Model,
		Dimensions: dimensions,
		TokensUsed: resp.Usage.TotalTokens,
		Metadata:   req.Metadata,
		ProviderInfo: ProviderMetadata{
			Provider:      "openai",
			LatencyMs:     latency.Milliseconds(),
			RateLimitInfo: p.extractRateLimitInfo(nil), // Would need response headers
		},
	}, nil
}

// BatchGenerateEmbeddings generates embeddings for multiple texts
func (p *OpenAIProvider) BatchGenerateEmbeddings(ctx context.Context, req BatchGenerateEmbeddingRequest) (*BatchEmbeddingResponse, error) {
	model, err := p.GetModel(req.Model)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	// OpenAI supports batch input
	openAIReq := openAIRequest{
		Input: req.Texts,
		Model: req.Model,
		User:  req.RequestID,
	}

	// Handle dimension reduction
	if dims, ok := req.Metadata["dimensions"].(int); ok && model.SupportsDimensionReduction {
		if dims >= model.MinDimensions && dims <= model.Dimensions {
			openAIReq.Dimensions = &dims
		}
	}

	// Make the API call with retry
	var resp *openAIResponse
	var lastErr error

	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := p.calculateRetryDelay(attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, lastErr = p.doRequest(ctx, openAIReq)
		if lastErr == nil {
			break
		}

		if !p.isRetryableError(lastErr) {
			break
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}

	latency := time.Since(start)

	// Extract embeddings
	embeddings := make([][]float32, len(resp.Data))
	for _, data := range resp.Data {
		embeddings[data.Index] = data.Embedding
	}

	dimensions := model.Dimensions
	if openAIReq.Dimensions != nil {
		dimensions = *openAIReq.Dimensions
	}

	return &BatchEmbeddingResponse{
		Embeddings:  embeddings,
		Model:       resp.Model,
		Dimensions:  dimensions,
		TotalTokens: resp.Usage.TotalTokens,
		Metadata:    req.Metadata,
		ProviderInfo: ProviderMetadata{
			Provider:  "openai",
			LatencyMs: latency.Milliseconds(),
		},
	}, nil
}

// GetSupportedModels returns the list of supported models
func (p *OpenAIProvider) GetSupportedModels() []ModelInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	models := make([]ModelInfo, 0, len(p.models))
	for _, model := range p.models {
		models = append(models, model)
	}
	return models
}

// GetModel returns information about a specific model
func (p *OpenAIProvider) GetModel(modelName string) (ModelInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	model, exists := p.models[modelName]
	if !exists {
		return ModelInfo{}, &ProviderError{
			Provider:   "openai",
			Code:       "MODEL_NOT_FOUND",
			Message:    fmt.Sprintf("model %s not found", modelName),
			StatusCode: 404,
		}
	}
	return model, nil
}

// HealthCheck verifies the provider is accessible
func (p *OpenAIProvider) HealthCheck(ctx context.Context) error {
	// Use a minimal request to check API availability
	req := openAIRequest{
		Input: "health check",
		Model: "text-embedding-ada-002", // Cheapest model
	}

	_, err := p.doRequest(ctx, req)
	return err
}

// Close cleans up resources
func (p *OpenAIProvider) Close() error {
	// HTTP client doesn't need explicit cleanup
	return nil
}

// Private methods

func (p *OpenAIProvider) doRequest(ctx context.Context, reqBody openAIRequest) (*openAIResponse, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint+"/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	// Add custom headers if any
	for k, v := range p.config.CustomHeaders {
		req.Header.Set(k, v)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Provider:    "openai",
			Code:        "REQUEST_FAILED",
			Message:     err.Error(),
			IsRetryable: true,
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		var errResp openAIErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, &ProviderError{
				Provider:    "openai",
				Code:        "UNKNOWN_ERROR",
				Message:     string(body),
				StatusCode:  resp.StatusCode,
				IsRetryable: p.isRetryableStatusCode(resp.StatusCode),
			}
		}

		retryAfter := p.parseRetryAfter(resp.Header.Get("Retry-After"))

		return nil, &ProviderError{
			Provider:    "openai",
			Code:        errResp.Error.Code,
			Message:     errResp.Error.Message,
			StatusCode:  resp.StatusCode,
			RetryAfter:  retryAfter,
			IsRetryable: p.isRetryableStatusCode(resp.StatusCode),
		}
	}

	// Parse successful response
	var openAIResp openAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(openAIResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}

	return &openAIResp, nil
}

func (p *OpenAIProvider) calculateRetryDelay(attempt int) time.Duration {
	delay := p.config.RetryDelayBase * time.Duration(1<<uint(attempt-1))
	if delay > p.config.RetryDelayMax {
		delay = p.config.RetryDelayMax
	}
	return delay
}

func (p *OpenAIProvider) isRetryableError(err error) bool {
	if provErr, ok := err.(*ProviderError); ok {
		return provErr.IsRetryable
	}
	// Network errors are generally retryable
	return true
}

func (p *OpenAIProvider) isRetryableStatusCode(code int) bool {
	switch code {
	case 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

func (p *OpenAIProvider) parseRetryAfter(header string) *time.Duration {
	if header == "" {
		return nil
	}

	// Try to parse as seconds
	if seconds, err := strconv.Atoi(header); err == nil {
		duration := time.Duration(seconds) * time.Second
		return &duration
	}

	// Try to parse as HTTP date
	if t, err := http.ParseTime(header); err == nil {
		duration := time.Until(t)
		if duration > 0 {
			return &duration
		}
	}

	return nil
}

func (p *OpenAIProvider) extractRateLimitInfo(headers http.Header) RateLimitInfo {
	info := RateLimitInfo{}

	// OpenAI rate limit headers (would need actual response headers)
	// x-ratelimit-limit-requests
	// x-ratelimit-limit-tokens
	// x-ratelimit-remaining-requests
	// x-ratelimit-remaining-tokens
	// x-ratelimit-reset-requests
	// x-ratelimit-reset-tokens

	return info
}

func parseTime(s string) *time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}
