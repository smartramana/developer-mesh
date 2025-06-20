package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// Default Google embedding model
	defaultGoogleModel = "text-embedding-004"
	// Default timeout for API requests
	defaultGoogleTimeout = 30 * time.Second
)

// GoogleProvider implements the Provider interface for Google Vertex AI embeddings
type GoogleProvider struct {
	projectID  string
	location   string
	apiKey     string
	httpClient *http.Client
}

// NewGoogleProvider creates a new Google Vertex AI embedding provider
func NewGoogleProvider(projectID, location, apiKey string) *GoogleProvider {
	return &GoogleProvider{
		projectID: projectID,
		location:  location,
		apiKey:    apiKey,
		httpClient: &http.Client{
			Timeout: defaultGoogleTimeout,
		},
	}
}

// googleEmbedRequest represents the request for Google embedding models
type googleEmbedRequest struct {
	Instances []googleInstance `json:"instances"`
}

type googleInstance struct {
	Content string `json:"content"`
}

// googleEmbedResponse represents the response from Google embedding models
type googleEmbedResponse struct {
	Predictions []struct {
		Embeddings struct {
			Values []float32 `json:"values"`
		} `json:"embeddings"`
	} `json:"predictions"`
}

// GenerateEmbedding generates an embedding using Google Vertex AI
func (p *GoogleProvider) GenerateEmbedding(ctx context.Context, content string, model string) ([]float32, error) {
	// Use default model if not specified
	if model == "" {
		model = defaultGoogleModel
	}

	// Map model names to endpoints
	var endpoint string
	switch model {
	case "gemini-embedding-001":
		endpoint = fmt.Sprintf(
			"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
			p.location, p.projectID, p.location, "textembedding-gecko@003",
		)
	case "text-embedding-004":
		endpoint = fmt.Sprintf(
			"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
			p.location, p.projectID, p.location, "text-embedding-004",
		)
	case "text-multilingual-embedding-002":
		endpoint = fmt.Sprintf(
			"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
			p.location, p.projectID, p.location, "text-multilingual-embedding-002",
		)
	case "multimodal-embedding":
		endpoint = fmt.Sprintf(
			"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
			p.location, p.projectID, p.location, "multimodalembedding",
		)
	default:
		return nil, fmt.Errorf("unsupported model: %s", model)
	}

	// Prepare request
	reqBody := googleEmbedRequest{
		Instances: []googleInstance{
			{Content: content},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Google provider - best effort logging
			_ = err
		}
	}()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var googleResp googleEmbedResponse
	if err := json.Unmarshal(body, &googleResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(googleResp.Predictions) == 0 || len(googleResp.Predictions[0].Embeddings.Values) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}

	return googleResp.Predictions[0].Embeddings.Values, nil
}

// GetSupportedModels returns the list of supported Google models
func (p *GoogleProvider) GetSupportedModels() []string {
	return []string{
		"gemini-embedding-001",
		"text-embedding-004",
		"text-multilingual-embedding-002",
		"multimodal-embedding",
	}
}

// ValidateAPIKey validates the Google API key
func (p *GoogleProvider) ValidateAPIKey() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := p.GenerateEmbedding(ctx, "test", "text-embedding-004")
	return err
}
