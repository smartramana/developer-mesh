package embedding

import (
    "fmt"
    "os"
)

// ProviderConfig contains configuration for creating providers
type ProviderConfig struct {
    // OpenAI configuration
    OpenAIAPIKey string
    
    // AWS/Bedrock configuration
    AWSRegion string
    
    // Google configuration
    GoogleProjectID string
    GoogleLocation  string
    GoogleAPIKey    string
    
    // Voyage AI configuration
    VoyageAPIKey string
}

// NewProviderConfigFromEnv creates provider config from environment variables
func NewProviderConfigFromEnv() *ProviderConfig {
    return &ProviderConfig{
        OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
        AWSRegion:       getEnvOrDefault("AWS_REGION", "us-east-1"),
        GoogleProjectID: os.Getenv("GOOGLE_PROJECT_ID"),
        GoogleLocation:  getEnvOrDefault("GOOGLE_LOCATION", "us-central1"),
        GoogleAPIKey:    os.Getenv("GOOGLE_API_KEY"),
        VoyageAPIKey:    os.Getenv("VOYAGE_API_KEY"),
    }
}

// CreateProviders creates all configured providers
func CreateProviders(config *ProviderConfig) (map[string]Provider, error) {
    providers := make(map[string]Provider)
    
    // Create OpenAI provider if configured
    if config.OpenAIAPIKey != "" {
        providers[ProviderOpenAI] = NewOpenAIProvider(config.OpenAIAPIKey)
    }
    
    // Create Bedrock provider if AWS is configured
    if config.AWSRegion != "" {
        bedrock, err := NewBedrockProvider(config.AWSRegion)
        if err != nil {
            return nil, fmt.Errorf("failed to create Bedrock provider: %w", err)
        }
        providers[ProviderAmazon] = bedrock
        providers[ProviderCohere] = bedrock // Cohere models are also on Bedrock
    }
    
    // Create Google provider if configured
    if config.GoogleProjectID != "" && config.GoogleAPIKey != "" {
        providers[ProviderGoogle] = NewGoogleProvider(
            config.GoogleProjectID,
            config.GoogleLocation,
            config.GoogleAPIKey,
        )
    }
    
    // Create Voyage AI provider if configured
    if config.VoyageAPIKey != "" {
        providers[ProviderVoyage] = NewVoyageProvider(config.VoyageAPIKey)
    }
    
    return providers, nil
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}