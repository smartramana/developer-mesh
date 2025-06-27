package embedding

import (
	"fmt"
	"os"
	"strings"
)

// ProviderCapability describes what a provider can do
type ProviderCapability struct {
	SupportsEmbeddings bool
	EmbeddingModels    []ModelInfo
	DefaultModel       string
}

// ModelInfo contains model metadata
type ModelInfo struct {
	ModelID    string
	Dimensions int
	MaxTokens  int
	CostPer1M  float64
	Notes      string
}

// ProviderCapabilities defines what each provider supports
var ProviderCapabilities = map[string]ProviderCapability{
	"openai": {
		SupportsEmbeddings: true,
		DefaultModel:       "text-embedding-3-small",
		EmbeddingModels: []ModelInfo{
			{ModelID: "text-embedding-3-small", Dimensions: 1536, MaxTokens: 8191, CostPer1M: 0.02},
			{ModelID: "text-embedding-3-large", Dimensions: 3072, MaxTokens: 8191, CostPer1M: 0.13},
			{ModelID: "text-embedding-ada-002", Dimensions: 1536, MaxTokens: 8191, CostPer1M: 0.10},
		},
	},
	"bedrock": {
		SupportsEmbeddings: true,
		DefaultModel:       "amazon.titan-embed-text-v2:0",
		EmbeddingModels: []ModelInfo{
			{ModelID: "amazon.titan-embed-text-v1", Dimensions: 1536, MaxTokens: 8192, CostPer1M: 0.02},
			{ModelID: "amazon.titan-embed-text-v2:0", Dimensions: 1024, MaxTokens: 8192, CostPer1M: 0.02},
			{ModelID: "cohere.embed-english-v3", Dimensions: 1024, MaxTokens: 0, CostPer1M: 0.10},
			{ModelID: "cohere.embed-multilingual-v3", Dimensions: 1024, MaxTokens: 0, CostPer1M: 0.10},
		},
	},
	"anthropic": {
		SupportsEmbeddings: false, // Direct API doesn't support embeddings yet
		EmbeddingModels:    []ModelInfo{},
	},
	"voyage": {
		SupportsEmbeddings: true,
		DefaultModel:       "voyage-2",
		EmbeddingModels: []ModelInfo{
			{ModelID: "voyage-2", Dimensions: 1024, MaxTokens: 0, CostPer1M: 0.10},
			{ModelID: "voyage-large-2", Dimensions: 1024, MaxTokens: 0, CostPer1M: 0.12},
			{ModelID: "voyage-code-2", Dimensions: 1024, MaxTokens: 0, CostPer1M: 0.10},
		},
	},
}

// EmbeddingProviderSelector intelligently selects and validates embedding providers
type EmbeddingProviderSelector struct {
	// Explicit configuration overrides
	PreferredProvider string
	PreferredModel    string

	// Auto-detection settings
	EnableAutoDetection bool
	ValidationMode      string // "strict" or "permissive"

	// Available credentials
	availableProviders map[string]bool
}

// NewEmbeddingProviderSelector creates a new selector with auto-detection
func NewEmbeddingProviderSelector() *EmbeddingProviderSelector {
	selector := &EmbeddingProviderSelector{
		EnableAutoDetection: true,
		ValidationMode:      "strict",
		availableProviders:  make(map[string]bool),
	}

	// Check environment for overrides
	if provider := os.Getenv("EMBEDDING_PROVIDER"); provider != "" {
		selector.PreferredProvider = provider
	}
	if model := os.Getenv("EMBEDDING_MODEL"); model != "" {
		selector.PreferredModel = model
	}

	// Detect available providers
	selector.detectAvailableProviders()

	return selector
}

// detectAvailableProviders checks which providers have credentials
func (s *EmbeddingProviderSelector) detectAvailableProviders() {
	// OpenAI
	if os.Getenv("OPENAI_API_KEY") != "" {
		s.availableProviders["openai"] = true
	}

	// AWS Bedrock (check multiple credential sources)
	if s.hasAWSCredentials() {
		s.availableProviders["bedrock"] = true
	}

	// Voyage AI
	if os.Getenv("VOYAGE_API_KEY") != "" {
		s.availableProviders["voyage"] = true
	}

	// Google Vertex AI
	if os.Getenv("GOOGLE_PROJECT_ID") != "" && os.Getenv("GOOGLE_API_KEY") != "" {
		s.availableProviders["google"] = true
	}
}

// hasAWSCredentials checks various AWS credential sources
func (s *EmbeddingProviderSelector) hasAWSCredentials() bool {
	// Check explicit credentials
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		return true
	}

	// Check for IAM role (EC2/ECS/Lambda)
	if os.Getenv("AWS_EXECUTION_ENV") != "" || os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != "" {
		return true
	}

	// Check for AWS profile
	if os.Getenv("AWS_PROFILE") != "" {
		return true
	}

	// Check for credentials file
	if _, err := os.Stat(os.ExpandEnv("$HOME/.aws/credentials")); err == nil {
		return true
	}

	return false
}

// SelectProvider returns the best available provider and model
func (s *EmbeddingProviderSelector) SelectProvider() (provider string, model string, dimensions int, err error) {
	// 1. Check if explicit provider is set and valid
	if s.PreferredProvider != "" {
		provider = s.PreferredProvider

		// Validate provider exists
		capability, exists := ProviderCapabilities[provider]
		if !exists {
			return "", "", 0, fmt.Errorf("unknown provider: %s", provider)
		}

		// Check if it supports embeddings
		if !capability.SupportsEmbeddings {
			return "", "", 0, fmt.Errorf("provider %s does not support embeddings", provider)
		}

		// Check if credentials are available
		if s.ValidationMode == "strict" && !s.availableProviders[provider] {
			return "", "", 0, fmt.Errorf("provider %s specified but credentials not found", provider)
		}

		// Select model
		if s.PreferredModel != "" {
			model = s.PreferredModel
			// Validate model exists for this provider
			dimensions = s.validateAndGetDimensions(provider, model)
			if dimensions == 0 {
				return "", "", 0, fmt.Errorf("model %s not available for provider %s", model, provider)
			}
		} else {
			// Use default model for provider
			model = capability.DefaultModel
			dimensions = s.getModelDimensions(provider, model)
		}

		return provider, model, dimensions, nil
	}

	// 2. Auto-detection mode
	if !s.EnableAutoDetection {
		return "", "", 0, fmt.Errorf("no provider specified and auto-detection disabled")
	}

	// 3. Priority order for auto-detection
	priorityOrder := []string{"openai", "bedrock", "voyage", "google"}

	for _, p := range priorityOrder {
		if s.availableProviders[p] {
			capability := ProviderCapabilities[p]
			if capability.SupportsEmbeddings {
				model = capability.DefaultModel
				dimensions = s.getModelDimensions(p, model)
				return p, model, dimensions, nil
			}
		}
	}

	return "", "", 0, fmt.Errorf("no embedding provider available. Please set OPENAI_API_KEY, configure AWS credentials, or set EMBEDDING_PROVIDER explicitly")
}

// validateAndGetDimensions checks if a model is valid for a provider
func (s *EmbeddingProviderSelector) validateAndGetDimensions(provider, model string) int {
	capability, exists := ProviderCapabilities[provider]
	if !exists {
		return 0
	}

	for _, m := range capability.EmbeddingModels {
		if m.ModelID == model {
			return m.Dimensions
		}
	}

	return 0
}

// getModelDimensions returns dimensions for a model
func (s *EmbeddingProviderSelector) getModelDimensions(provider, model string) int {
	capability := ProviderCapabilities[provider]
	for _, m := range capability.EmbeddingModels {
		if m.ModelID == model {
			return m.Dimensions
		}
	}
	return 0
}

// GetProviderSummary returns a summary of available providers
func (s *EmbeddingProviderSelector) GetProviderSummary() string {
	var summary strings.Builder

	summary.WriteString("=== Embedding Provider Configuration ===\n")

	// Show detected providers
	summary.WriteString("\nDetected Providers:\n")
	for provider := range s.availableProviders {
		capability := ProviderCapabilities[provider]
		if capability.SupportsEmbeddings {
			summary.WriteString(fmt.Sprintf("  - %s (default: %s)\n", provider, capability.DefaultModel))
		}
	}

	// Show current configuration
	if s.PreferredProvider != "" {
		summary.WriteString(fmt.Sprintf("\nConfigured Provider: %s\n", s.PreferredProvider))
		if s.PreferredModel != "" {
			summary.WriteString(fmt.Sprintf("Configured Model: %s\n", s.PreferredModel))
		}
	} else {
		summary.WriteString("\nUsing auto-detection\n")
	}

	// Show recommendation
	provider, model, dimensions, err := s.SelectProvider()
	if err == nil {
		summary.WriteString("\nSelected Configuration:\n")
		summary.WriteString(fmt.Sprintf("  Provider: %s\n", provider))
		summary.WriteString(fmt.Sprintf("  Model: %s\n", model))
		summary.WriteString(fmt.Sprintf("  Dimensions: %d\n", dimensions))
	} else {
		summary.WriteString(fmt.Sprintf("\nError: %s\n", err.Error()))
	}

	return summary.String()
}
