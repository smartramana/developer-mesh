package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/S-Corkum/devops-mcp/internal/embedding"
)

// This example demonstrates how to use the AWS Bedrock embedding service 
// with proper credential handling and environment configurations
func main() {
	// Create a context with timeout
	ctx := context.Background()

	// Get AWS credentials from environment variables
	awsRegion := getEnvOrDefault("AWS_REGION", "us-west-2")
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	awsSessionToken := os.Getenv("AWS_SESSION_TOKEN")

	// Determine which model to use
	bedrockModel := getEnvOrDefault("BEDROCK_MODEL", "amazon.titan-embed-text-v1")

	// Create AWS Bedrock configuration
	config := &embedding.BedrockConfig{
		Region:           awsRegion,
		AccessKeyID:      awsAccessKey,
		SecretAccessKey:  awsSecretKey,
		SessionToken:     awsSessionToken,
		ModelID:          bedrockModel,
		UseMockEmbeddings: false, // Set to true for testing without real AWS credentials
	}

	// Create a new Bedrock embedding service
	service, err := embedding.NewBedrockEmbeddingService(config)
	if err != nil {
		log.Fatalf("Failed to create Bedrock embedding service: %v", err)
	}

	// Check if we're using mock embeddings (for logging purposes)
	if service.GetModelConfig().Name != bedrockModel {
		log.Printf("Using model: %s", service.GetModelConfig().Name)
	}

	// Sample text to embed
	textToEmbed := []string{
		"AWS Bedrock is a fully managed service that offers a choice of high-performing foundation models.",
		"These models can be accessed through an API and fine-tuned with your data.",
		"AWS Bedrock also offers various capabilities to build generative AI applications.",
	}

	// Generate content IDs based on the text (in production, use a more robust ID generation method)
	contentIDs := make([]string, len(textToEmbed))
	for i := range textToEmbed {
		contentIDs[i] = fmt.Sprintf("doc-%d", i)
	}

	// Generate embeddings for the text
	embeddings, err := service.BatchGenerateEmbeddings(ctx, textToEmbed, "text/plain", contentIDs)
	if err != nil {
		log.Fatalf("Failed to generate embeddings: %v", err)
	}

	// Print results
	fmt.Println("Successfully generated embeddings:")
	for i, embedding := range embeddings {
		// Print first few dimensions as a sample
		vectorPreview := fmt.Sprintf("%v", embedding.Vector[:5])
		fmt.Printf(
			"Text: %s\nModel: %s\nDimensions: %d\nVector Preview: %s...\n\n",
			truncateString(textToEmbed[i], 40),
			embedding.ModelID,
			embedding.Dimensions,
			vectorPreview,
		)
	}
}

// Helper to get environment variable with default value
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Helper to truncate strings for display
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
