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

// VoyageProvider implements the Provider interface for Voyage AI embeddings
type VoyageProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewVoyageProvider creates a new Voyage AI embedding provider
func NewVoyageProvider(apiKey string) *VoyageProvider {
	return &VoyageProvider{
		apiKey:  apiKey,
		baseURL: "https://api.voyageai.com/v1",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// voyageRequest represents the request structure for Voyage AI API
type voyageRequest struct {
	Input     []string `json:"input"`
	Model     string   `json:"model"`
	InputType string   `json:"input_type,omitempty"`
}

// voyageResponse represents the response from Voyage AI API
type voyageResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// GenerateEmbedding generates an embedding using Voyage AI API
func (p *VoyageProvider) GenerateEmbedding(ctx context.Context, content string, model string) ([]float32, error) {
	// Determine input type based on model
	inputType := "document"
	if model == "voyage-code-2" || model == "voyage-code-3" {
		inputType = "code"
	}

	// Prepare request
	reqBody := voyageRequest{
		Input:     []string{content},
		Model:     model,
		InputType: inputType,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/embeddings", bytes.NewBuffer(jsonData))
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
			// Voyage provider - best effort logging
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
		return nil, fmt.Errorf("voyage AI API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var voyageResp voyageResponse
	if err := json.Unmarshal(body, &voyageResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(voyageResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}

	return voyageResp.Data[0].Embedding, nil
}

// GetSupportedModels returns the list of supported Voyage AI models
func (p *VoyageProvider) GetSupportedModels() []string {
	return []string{
		"voyage-large-2",
		"voyage-code-3",
		"voyage-2",
		"voyage-code-2",
	}
}

// ValidateAPIKey validates the Voyage AI API key
func (p *VoyageProvider) ValidateAPIKey() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use voyage-2 as the test model
	_, err := p.GenerateEmbedding(ctx, "test", "voyage-2")
	return err
}
