package embedding

import (
	"errors"
	"fmt"
)

// List of supported OpenAI embedding models
var supportedOpenAIModels = map[string]int{
	"text-embedding-3-small": 1536,
	"text-embedding-3-large": 3072,
	"text-embedding-ada-002": 1536,
}

// List of supported AWS Bedrock embedding models
var supportedBedrockModels = map[string]int{
	// Amazon Titan models
	"amazon.titan-embed-text-v1":     1536,
	"amazon.titan-embed-text-v2:0":   1024, // The newer v2 model has 1024 dimensions
	"amazon.titan-embed-image-v1":    1024, // Image embedding model
	
	// Cohere models
	"cohere.embed-english-v3":        1024,
	"cohere.embed-multilingual-v3":   1024,
	"cohere.embed-english-v3:0":      1024, // Versioned format
	"cohere.embed-multilingual-v3:0": 1024, // Versioned format
	
	// Anthropic models
	// Claude 3.0 Family
	"anthropic.claude-3-haiku-20240307-v1:0":   3072,
	"anthropic.claude-3-sonnet-20240229-v1:0":  4096,
	"anthropic.claude-3-opus-20240229-v1:0":    4096,
	// Claude 3.5 Family
	"anthropic.claude-3-5-haiku-20250531-v1:0":   4096,
	// Claude 3.7 Family
	"anthropic.claude-3-7-sonnet-20250531-v1:0":  8192,
	
	// Meta models
	"meta.llama3-8b-embedding-v1:0":           4096,
	"meta.llama3-70b-embedding-v1:0":          4096,
	
	// Placeholders for future Nova embedding models
	// These will be uncommented when the models become available
	// "amazon.nova-embed-text-v1:0":          4096, // Placeholder based on expected dimensions
	// "amazon.nova-embed-multilingual-v1:0":  4096, // Placeholder based on expected dimensions
}

// List of supported Anthropic API embedding models
var supportedAnthropicModels = map[string]int{
	// Claude 3 Models
	"claude-3-haiku-20240307":   3072,
	"claude-3-sonnet-20240229":  4096,
	"claude-3-opus-20240229":    4096,
	// Claude 3.5 Family
	"claude-3-5-haiku-20250531":   4096,
	// Claude 3.7 Family 
	"claude-3-7-sonnet-20250531":  8192,
}

// ValidateEmbeddingModel validates an embedding model name
func ValidateEmbeddingModel(modelType ModelType, modelName string) error {
	if modelName == "" {
		return errors.New("model name is required")
	}
	
	switch modelType {
	case ModelTypeOpenAI:
		_, found := supportedOpenAIModels[modelName]
		if !found {
			return fmt.Errorf("unsupported OpenAI model: %s", modelName)
		}
	case ModelTypeBedrock:
		_, found := supportedBedrockModels[modelName]
		if !found {
			return fmt.Errorf("unsupported AWS Bedrock model: %s", modelName)
		}
	case ModelTypeAnthropic:
		_, found := supportedAnthropicModels[modelName]
		if !found {
			return fmt.Errorf("unsupported Anthropic model: %s", modelName)
		}
	case ModelTypeHuggingFace, ModelTypeCustom:
		// For now, we don't validate these models specifically
		// but we acknowledge them as valid types
		return nil
	default:
		return fmt.Errorf("unsupported model type: %s", modelType)
	}
	
	return nil
}

// GetEmbeddingModelDimensions returns the dimensions for a given model
func GetEmbeddingModelDimensions(modelType ModelType, modelName string) (int, error) {
	err := ValidateEmbeddingModel(modelType, modelName)
	if err != nil {
		return 0, err
	}
	
	switch modelType {
	case ModelTypeOpenAI:
		return supportedOpenAIModels[modelName], nil
	case ModelTypeBedrock:
		return supportedBedrockModels[modelName], nil
	case ModelTypeAnthropic:
		return supportedAnthropicModels[modelName], nil
	case ModelTypeHuggingFace, ModelTypeCustom:
		// For custom models, we don't know the dimensions in advance
		// The caller must specify them
		return 0, errors.New("dimensions must be explicitly specified for this model type")
	default:
		return 0, fmt.Errorf("unsupported model type: %s", modelType)
	}
}
