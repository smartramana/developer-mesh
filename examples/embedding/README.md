# AWS Bedrock Embedding Examples

This directory contains examples demonstrating how to use AWS Bedrock for generating embeddings within the embedding package.

## Features

- **Multiple Model Support**: Includes support for Amazon, Cohere, and Anthropic models
- **Graceful Fallbacks**: Automatically uses mock embeddings in test environments
- **AWS Credentials Management**: Proper handling of AWS credentials from environment variables
- **Batch Processing**: Handles large batches efficiently

## Supported Models

| Provider | Model ID | Dimensions | Description |
|----------|----------|------------|-------------|
| Amazon   | amazon.titan-embed-text-v1 | 1536 | Amazon's Titan text embedding model |
| Cohere   | cohere.embed-english-v3 | 1024 | Cohere's English embedding model v3 |
| Cohere   | cohere.embed-multilingual-v3 | 1024 | Cohere's multilingual embedding model v3 |
| Anthropic | anthropic.claude-3-haiku-20240307-v1:0 | 1536 | Anthropic's Claude 3 Haiku embedding model |
| Anthropic | anthropic.claude-3-sonnet-20240229-v1:0 | 1536 | Anthropic's Claude 3 Sonnet embedding model |

## Usage

### Production Use

To use AWS Bedrock embeddings in production:

```go
// Set up AWS credentials (via environment variables or instance profile)
config := &embedding.BedrockConfig{
    Region:           "us-west-2",
    AccessKeyID:      os.Getenv("AWS_ACCESS_KEY_ID"),      // Optional if using instance profile
    SecretAccessKey:  os.Getenv("AWS_SECRET_ACCESS_KEY"),  // Optional if using instance profile
    SessionToken:     os.Getenv("AWS_SESSION_TOKEN"),      // Optional
    ModelID:          "amazon.titan-embed-text-v1",
    UseMockEmbeddings: false,
}

// Create the embedding service
service, err := embedding.NewBedrockEmbeddingService(config)
if err != nil {
    log.Fatalf("Failed to create service: %v", err)
}

// Generate embeddings
embeddings, err := service.BatchGenerateEmbeddings(ctx, texts, "text/plain", contentIDs)
```

### Testing Without AWS Credentials

For testing without actual AWS credentials:

```go
// Option 1: Use the mock constructor
service, err := embedding.NewMockBedrockEmbeddingService("amazon.titan-embed-text-v1")
if err != nil {
    log.Fatalf("Failed to create mock service: %v", err)
}

// Option 2: Configure a regular service to use mock embeddings
config := &embedding.BedrockConfig{
    Region:           "us-west-2",
    ModelID:          "amazon.titan-embed-text-v1",
    UseMockEmbeddings: true,  // This forces mock embedding generation
}

service, err := embedding.NewBedrockEmbeddingService(config)
```

### Factory Integration

To use AWS Bedrock via the embedding factory:

```go
factoryConfig := &embedding.EmbeddingFactoryConfig{
    ServiceType: embedding.ServiceTypeBedrock,
    ModelName:   "amazon.titan-embed-text-v1",
    Parameters: map[string]interface{}{
        "region":            "us-west-2",
        "access_key_id":     os.Getenv("AWS_ACCESS_KEY_ID"),
        "secret_access_key": os.Getenv("AWS_SECRET_ACCESS_KEY"),
        "session_token":     os.Getenv("AWS_SESSION_TOKEN"),
        "use_mock":          false,
    },
}

factory, err := embedding.NewEmbeddingFactory(factoryConfig)
if err != nil {
    log.Fatalf("Failed to create factory: %v", err)
}

service, err := factory.CreateEmbeddingService()
if err != nil {
    log.Fatalf("Failed to create service: %v", err)
}
```

## Running the Examples

```bash
# Set AWS credentials in your environment
export AWS_ACCESS_KEY_ID=your_access_key
export AWS_SECRET_ACCESS_KEY=your_secret_key
export AWS_REGION=us-west-2

# Run the example
go run bedrock_example.go
```

Or to use mock embeddings (no AWS credentials needed):

```bash
# Set environment variable to use mock embeddings
export USE_MOCK_EMBEDDINGS=true

# Run the example
go run bedrock_example.go
```
