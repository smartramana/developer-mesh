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

// OpenAIProvider implements the Provider interface for OpenAI embeddings
type OpenAIProvider struct {
    apiKey     string
    baseURL    string
    httpClient *http.Client
}

// NewOpenAIProvider creates a new OpenAI embedding provider
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
    return &OpenAIProvider{
        apiKey:  apiKey,
        baseURL: "https://api.openai.com/v1",
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

// openAIRequest represents the request structure for OpenAI API
type openAIRequest struct {
    Input          string  `json:"input"`
    Model          string  `json:"model"`
    EncodingFormat string  `json:"encoding_format,omitempty"`
    Dimensions     *int    `json:"dimensions,omitempty"` // For models that support dimension reduction
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

// GenerateEmbedding generates an embedding using OpenAI API
func (p *OpenAIProvider) GenerateEmbedding(ctx context.Context, content string, model string) ([]float32, error) {
    // Prepare request
    reqBody := openAIRequest{
        Input: content,
        Model: model,
    }
    
    // Add dimensions for models that support reduction
    // For now, use default dimensions for text-embedding-3-small and text-embedding-3-large
    
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
            // OpenAI provider - best effort logging
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
        return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
    }
    
    // Parse response
    var openAIResp openAIResponse
    if err := json.Unmarshal(body, &openAIResp); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }
    
    if len(openAIResp.Data) == 0 {
        return nil, fmt.Errorf("no embedding data in response")
    }
    
    return openAIResp.Data[0].Embedding, nil
}

// GetSupportedModels returns the list of supported OpenAI models
func (p *OpenAIProvider) GetSupportedModels() []string {
    return []string{
        "text-embedding-3-small",
        "text-embedding-3-large", 
        "text-embedding-ada-002",
    }
}

// ValidateAPIKey validates the OpenAI API key
func (p *OpenAIProvider) ValidateAPIKey() error {
    // Make a simple API call to validate the key
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    // Use ada-002 as it's the cheapest
    _, err := p.GenerateEmbedding(ctx, "test", "text-embedding-ada-002")
    return err
}