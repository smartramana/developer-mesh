package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// Default OpenAI API endpoint
	defaultOpenAIEndpoint = "https://api.openai.com/v1/embeddings"
	// Default OpenAI model
	defaultOpenAIModel = "text-embedding-3-small"
	// Default dimensions for text-embedding-3-small
	defaultOpenAIDimensions = 1536
	// Default timeout for API requests
	defaultTimeout = 30 * time.Second
	// Maximum batch size for OpenAI API
	maxOpenAIBatchSize = 16
)

// OpenAIEmbeddingRequest represents a request to the OpenAI embeddings API
type OpenAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// OpenAIEmbeddingResponse represents a response from the OpenAI embeddings API
type OpenAIEmbeddingResponse struct {
	Data  []OpenAIEmbeddingData `json:"data"`
	Model string                `json:"model"`
	Usage OpenAIUsage           `json:"usage"`
}

// OpenAIEmbeddingData represents embedding data in an OpenAI API response
type OpenAIEmbeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// OpenAIUsage represents usage information in an OpenAI API response
type OpenAIUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// OpenAIEmbeddingService implements EmbeddingService using OpenAI's API
type OpenAIEmbeddingService struct {
	// Configuration for the embedding model
	config ModelConfig
	// HTTP client for API requests
	client *http.Client
}

// The validation functions and model dimensions have been moved to models.go

// NewOpenAIEmbeddingService creates a new OpenAI embedding service
func NewOpenAIEmbeddingService(apiKey string, modelName string, dimensions int) (*OpenAIEmbeddingService, error) {
	if apiKey == "" {
		return nil, errors.New("API key is required for OpenAI embeddings")
	}

	// Use default model if not specified
	if modelName == "" {
		return nil, errors.New("model name is required")
	}

	// Validate model name
	err := ValidateEmbeddingModel(ModelTypeOpenAI, modelName)
	if err != nil {
		return nil, err
	}

	// Get dimensions for the model if not specified
	if dimensions <= 0 {
		dimensions = supportedOpenAIModels[modelName]
	}

	config := ModelConfig{
		Type:       ModelTypeOpenAI,
		Name:       modelName,
		APIKey:     apiKey,
		Dimensions: dimensions,
		Endpoint:   defaultOpenAIEndpoint,
	}

	client := &http.Client{
		Timeout: defaultTimeout,
	}

	return &OpenAIEmbeddingService{
		config: config,
		client: client,
	}, nil
}

// GenerateEmbedding creates an embedding for a single text
func (s *OpenAIEmbeddingService) GenerateEmbedding(ctx context.Context, text string, contentType string, contentID string) (*EmbeddingVector, error) {
	// Validate text is not empty
	if text == "" {
		return nil, errors.New("content cannot be empty")
	}

	embeddings, err := s.BatchGenerateEmbeddings(ctx, []string{text}, contentType, []string{contentID})
	if err != nil {
		// Check if this is an API error and format it accordingly
		if strings.Contains(err.Error(), "API request failed") {
			return nil, fmt.Errorf("failed to generate embedding: OpenAI API error: %w", err)
		}
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, errors.New("no embeddings generated")
	}

	return embeddings[0], nil
}

// BatchGenerateEmbeddings creates embeddings for multiple texts
func (s *OpenAIEmbeddingService) BatchGenerateEmbeddings(ctx context.Context, texts []string, contentType string, contentIDs []string) ([]*EmbeddingVector, error) {
	if len(texts) == 0 {
		return nil, errors.New("no texts provided for embedding generation")
	}

	if len(texts) != len(contentIDs) {
		return nil, errors.New("number of texts must match number of content IDs")
	}

	// Process in batches if needed
	if len(texts) > maxOpenAIBatchSize {
		return s.processBatches(ctx, texts, contentType, contentIDs)
	}

	reqBody := OpenAIEmbeddingRequest{
		Model: s.config.Name,
		Input: texts,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.config.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response OpenAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	result := make([]*EmbeddingVector, len(response.Data))
	for i, data := range response.Data {
		result[i] = &EmbeddingVector{
			Vector:      data.Embedding,
			Dimensions:  len(data.Embedding),
			ModelID:     response.Model,
			ContentType: contentType,
			ContentID:   contentIDs[i],
			Metadata: map[string]interface{}{
				"prompt_tokens": response.Usage.PromptTokens / len(texts),
				"provider":      "openai",
			},
		}
	}

	return result, nil
}

// processBatches breaks down a large batch into smaller batches for API processing
func (s *OpenAIEmbeddingService) processBatches(ctx context.Context, texts []string, contentType string, contentIDs []string) ([]*EmbeddingVector, error) {
	var allEmbeddings []*EmbeddingVector

	for i := 0; i < len(texts); i += maxOpenAIBatchSize {
		end := i + maxOpenAIBatchSize
		if end > len(texts) {
			end = len(texts)
		}

		batchTexts := texts[i:end]
		batchIDs := contentIDs[i:end]

		embeddings, err := s.BatchGenerateEmbeddings(ctx, batchTexts, contentType, batchIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to process batch %d-%d: %w", i, end, err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// GetModelConfig returns the model configuration
func (s *OpenAIEmbeddingService) GetModelConfig() ModelConfig {
	return s.config
}

// GetModelDimensions returns the dimensions of the embeddings generated by this model
func (s *OpenAIEmbeddingService) GetModelDimensions() int {
	return s.config.Dimensions
}
