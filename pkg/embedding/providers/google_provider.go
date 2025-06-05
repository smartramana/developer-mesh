package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GoogleProvider implements Provider interface for Google Vertex AI
type GoogleProvider struct {
	config     ProviderConfig
	httpClient *http.Client
	models     map[string]ModelInfo
	projectID  string
	location   string
}

// googleEmbeddingRequest represents the request structure for Google's embedding API
type googleEmbeddingRequest struct {
	Instances []googleInstance `json:"instances"`
}

type googleInstance struct {
	Content  string `json:"content"`
	TaskType string `json:"task_type,omitempty"`
}

// googleEmbeddingResponse represents the response from Google's API
type googleEmbeddingResponse struct {
	Predictions []googlePrediction `json:"predictions"`
	Metadata    struct {
		BillableCharacterCount int `json:"billableCharacterCount"`
	} `json:"metadata,omitempty"`
}

type googlePrediction struct {
	Embeddings struct {
		Values []float32 `json:"values"`
	} `json:"embeddings"`
}

// NewGoogleProvider creates a new Google Vertex AI provider
func NewGoogleProvider(config ProviderConfig) (*GoogleProvider, error) {
	// Extract Google-specific config from ExtraParams
	projectID, _ := config.ExtraParams["google_project_id"].(string)
	if projectID == "" {
		return nil, fmt.Errorf("google project ID is required (set in ExtraParams['google_project_id'])")
	}

	location, _ := config.ExtraParams["google_location"].(string)
	if location == "" {
		location = "us-central1" // Default location
	}

	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}

	p := &GoogleProvider{
		config:    config,
		projectID: projectID,
		location:  location,
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
	}

	// Initialize supported models
	p.models = map[string]ModelInfo{
		"textembedding-gecko@003": {
			Name:               "textembedding-gecko@003",
			DisplayName:        "Google Text Embedding Gecko 003",
			Dimensions:         768,
			MaxTokens:          3072,
			CostPer1MTokens:    0.025,
			SupportedTaskTypes: []string{"RETRIEVAL_QUERY", "RETRIEVAL_DOCUMENT", "SEMANTIC_SIMILARITY", "CLASSIFICATION", "CLUSTERING"},
			IsActive:           true,
		},
		"textembedding-gecko-multilingual@001": {
			Name:               "textembedding-gecko-multilingual@001",
			DisplayName:        "Google Text Embedding Gecko Multilingual",
			Dimensions:         768,
			MaxTokens:          3072,
			CostPer1MTokens:    0.025,
			SupportedTaskTypes: []string{"RETRIEVAL_QUERY", "RETRIEVAL_DOCUMENT", "SEMANTIC_SIMILARITY", "CLASSIFICATION", "CLUSTERING", "multilingual"},
			IsActive:           true,
		},
		"text-embedding-preview-0815": {
			Name:                       "text-embedding-preview-0815",
			DisplayName:                "Google Text Embedding Preview",
			Dimensions:                 768,
			MaxTokens:                  8192,
			CostPer1MTokens:            0.025,
			SupportedTaskTypes:         []string{"RETRIEVAL_QUERY", "RETRIEVAL_DOCUMENT", "SEMANTIC_SIMILARITY", "CLASSIFICATION", "CLUSTERING", "QUESTION_ANSWERING", "FACT_VERIFICATION"},
			SupportsDimensionReduction: true,
			MinDimensions:              256,
			IsActive:                   true,
		},
		"text-multilingual-embedding-002": {
			Name:               "text-multilingual-embedding-002",
			DisplayName:        "Google Text Multilingual Embedding v2",
			Dimensions:         768,
			MaxTokens:          2048,
			CostPer1MTokens:    0.025,
			SupportedTaskTypes: []string{"RETRIEVAL_QUERY", "RETRIEVAL_DOCUMENT", "SEMANTIC_SIMILARITY", "CLASSIFICATION", "CLUSTERING", "multilingual"},
			IsActive:           true,
		},
	}

	return p, nil
}

// Name returns the provider name
func (p *GoogleProvider) Name() string {
	return "google"
}

// GenerateEmbedding generates an embedding for the given text
func (p *GoogleProvider) GenerateEmbedding(ctx context.Context, req GenerateEmbeddingRequest) (*EmbeddingResponse, error) {
	_, err := p.GetModel(req.Model)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	// Determine task type
	taskType := "RETRIEVAL_DOCUMENT"
	if tt, ok := req.Metadata["task_type"].(string); ok {
		taskType = tt
	}

	// Prepare request
	googleReq := googleEmbeddingRequest{
		Instances: []googleInstance{
			{
				Content:  req.Text,
				TaskType: taskType,
			},
		},
	}

	// Make the API call
	resp, err := p.doRequest(ctx, req.Model, googleReq)
	if err != nil {
		return nil, err
	}

	if len(resp.Predictions) == 0 || len(resp.Predictions[0].Embeddings.Values) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}

	latency := time.Since(start)

	// Calculate tokens (Google uses characters, estimate tokens)
	tokensUsed := resp.Metadata.BillableCharacterCount / 4 // Rough estimate

	return &EmbeddingResponse{
		Embedding:  resp.Predictions[0].Embeddings.Values,
		Model:      req.Model,
		Dimensions: len(resp.Predictions[0].Embeddings.Values),
		TokensUsed: tokensUsed,
		Metadata:   req.Metadata,
		ProviderInfo: ProviderMetadata{
			Provider:  "google",
			LatencyMs: latency.Milliseconds(),
		},
	}, nil
}

// BatchGenerateEmbeddings generates embeddings for multiple texts
func (p *GoogleProvider) BatchGenerateEmbeddings(ctx context.Context, req BatchGenerateEmbeddingRequest) (*BatchEmbeddingResponse, error) {
	model, err := p.GetModel(req.Model)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	// Determine task type
	taskType := "RETRIEVAL_DOCUMENT"
	if tt, ok := req.Metadata["task_type"].(string); ok {
		taskType = tt
	}

	// Google supports batch requests natively
	instances := make([]googleInstance, len(req.Texts))
	for i, text := range req.Texts {
		instances[i] = googleInstance{
			Content:  text,
			TaskType: taskType,
		}
	}

	googleReq := googleEmbeddingRequest{
		Instances: instances,
	}

	// Make the API call
	resp, err := p.doRequest(ctx, req.Model, googleReq)
	if err != nil {
		return nil, err
	}

	// Extract embeddings
	embeddings := make([][]float32, len(resp.Predictions))
	for i, pred := range resp.Predictions {
		embeddings[i] = pred.Embeddings.Values
	}

	latency := time.Since(start)
	tokensUsed := resp.Metadata.BillableCharacterCount / 4

	return &BatchEmbeddingResponse{
		Embeddings:  embeddings,
		Model:       req.Model,
		Dimensions:  model.Dimensions,
		TotalTokens: tokensUsed,
		Metadata:    req.Metadata,
		ProviderInfo: ProviderMetadata{
			Provider:  "google",
			LatencyMs: latency.Milliseconds(),
		},
	}, nil
}

// GetSupportedModels returns the list of supported models
func (p *GoogleProvider) GetSupportedModels() []ModelInfo {
	models := make([]ModelInfo, 0, len(p.models))
	for _, model := range p.models {
		models = append(models, model)
	}
	return models
}

// GetModel returns information about a specific model
func (p *GoogleProvider) GetModel(modelName string) (ModelInfo, error) {
	model, exists := p.models[modelName]
	if !exists {
		return ModelInfo{}, &ProviderError{
			Provider:   "google",
			Code:       "MODEL_NOT_FOUND",
			Message:    fmt.Sprintf("model %s not found", modelName),
			StatusCode: 404,
		}
	}
	return model, nil
}

// HealthCheck verifies the provider is accessible
func (p *GoogleProvider) HealthCheck(ctx context.Context) error {
	// Use a minimal request to check API availability
	req := googleEmbeddingRequest{
		Instances: []googleInstance{
			{
				Content:  "health check",
				TaskType: "SEMANTIC_SIMILARITY",
			},
		},
	}

	_, err := p.doRequest(ctx, "textembedding-gecko@003", req)
	return err
}

// Close cleans up resources
func (p *GoogleProvider) Close() error {
	// HTTP client doesn't need explicit cleanup
	return nil
}

// Private methods

func (p *GoogleProvider) doRequest(ctx context.Context, model string, reqBody googleEmbeddingRequest) (*googleEmbeddingResponse, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL
	url := p.buildURL(model)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	
	// Add authentication
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	} else if accessToken, ok := p.config.ExtraParams["google_access_token"].(string); ok && accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	} else {
		// Use Application Default Credentials
		// This would require integrating with Google's auth library
		return nil, fmt.Errorf("authentication not configured: set APIKey or ExtraParams['google_access_token']")
	}

	// Add custom headers if any
	for k, v := range p.config.CustomHeaders {
		req.Header.Set(k, v)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Provider:    "google",
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
		var errorResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}

		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, &ProviderError{
				Provider:    "google",
				Code:        "UNKNOWN_ERROR",
				Message:     string(body),
				StatusCode:  resp.StatusCode,
				IsRetryable: p.isRetryableStatusCode(resp.StatusCode),
			}
		}

		return nil, &ProviderError{
			Provider:    "google",
			Code:        errorResp.Error.Status,
			Message:     errorResp.Error.Message,
			StatusCode:  resp.StatusCode,
			IsRetryable: p.isRetryableStatusCode(resp.StatusCode),
		}
	}

	// Parse successful response
	var googleResp googleEmbeddingResponse
	if err := json.Unmarshal(body, &googleResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &googleResp, nil
}

func (p *GoogleProvider) buildURL(model string) string {
	// Format: https://{location}-aiplatform.googleapis.com/v1/projects/{project}/locations/{location}/publishers/google/models/{model}:predict
	return fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
		p.location,
		p.projectID,
		p.location,
		model,
	)
}

func (p *GoogleProvider) isRetryableStatusCode(code int) bool {
	switch code {
	case 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}