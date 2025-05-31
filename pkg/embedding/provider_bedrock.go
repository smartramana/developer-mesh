package embedding

import (
    "context"
    "encoding/json"
    "fmt"
    
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// BedrockProvider implements the Provider interface for Amazon Bedrock embeddings
type BedrockProvider struct {
    client *bedrockruntime.Client
    region string
}

// NewBedrockProvider creates a new Bedrock embedding provider
func NewBedrockProvider(region string) (*BedrockProvider, error) {
    cfg, err := config.LoadDefaultConfig(context.Background(),
        config.WithRegion(region),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }
    
    client := bedrockruntime.NewFromConfig(cfg)
    
    return &BedrockProvider{
        client: client,
        region: region,
    }, nil
}

// titanEmbedRequest represents the request for Titan embedding models
type titanEmbedRequest struct {
    InputText string `json:"inputText"`
}

// titanEmbedResponse represents the response from Titan embedding models
type titanEmbedResponse struct {
    Embedding []float32 `json:"embedding"`
}

// cohereEmbedRequest represents the request for Cohere embedding models
type cohereEmbedRequest struct {
    Texts     []string `json:"texts"`
    InputType string   `json:"input_type,omitempty"`
}

// cohereEmbedResponse represents the response from Cohere embedding models
type cohereEmbedResponse struct {
    Embeddings [][]float32 `json:"embeddings"`
}

// GenerateEmbedding generates an embedding using Amazon Bedrock
func (p *BedrockProvider) GenerateEmbedding(ctx context.Context, content string, model string) ([]float32, error) {
    var modelID string
    var requestBody []byte
    var err error
    
    switch model {
    case "titan-embed-text-v2":
        modelID = "amazon.titan-embed-text-v2:0"
        req := titanEmbedRequest{InputText: content}
        requestBody, err = json.Marshal(req)
    case "embed-english-v3", "embed-multilingual-v3":
        modelID = fmt.Sprintf("cohere.%s", model)
        req := cohereEmbedRequest{
            Texts:     []string{content},
            InputType: "search_document",
        }
        requestBody, err = json.Marshal(req)
    // Note: As of 2024, Anthropic's Claude models on Bedrock don't support embeddings
    // Claude models are text generation only. For Anthropic embeddings, use Voyage AI
    default:
        return nil, fmt.Errorf("unsupported model: %s", model)
    }
    
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }
    
    // Invoke the model
    output, err := p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
        ModelId:     aws.String(modelID),
        ContentType: aws.String("application/json"),
        Body:        requestBody,
    })
    
    if err != nil {
        return nil, fmt.Errorf("failed to invoke model: %w", err)
    }
    
    // Parse response based on model type
    if model == "titan-embed-text-v2" {
        var resp titanEmbedResponse
        if err := json.Unmarshal(output.Body, &resp); err != nil {
            return nil, fmt.Errorf("failed to parse Titan response: %w", err)
        }
        return resp.Embedding, nil
    } else {
        var resp cohereEmbedResponse
        if err := json.Unmarshal(output.Body, &resp); err != nil {
            return nil, fmt.Errorf("failed to parse Cohere response: %w", err)
        }
        if len(resp.Embeddings) == 0 {
            return nil, fmt.Errorf("no embeddings in response")
        }
        return resp.Embeddings[0], nil
    }
}

// GetSupportedModels returns the list of supported Bedrock models
func (p *BedrockProvider) GetSupportedModels() []string {
    return []string{
        "titan-embed-text-v2",
        "embed-english-v3",
        "embed-multilingual-v3",
    }
}

// ValidateAPIKey validates AWS credentials
func (p *BedrockProvider) ValidateAPIKey() error {
    // AWS SDK handles credential validation
    // We can make a simple ListFoundationModels call to validate
    ctx := context.Background()
    _, err := p.GenerateEmbedding(ctx, "test", "titan-embed-text-v2")
    return err
}