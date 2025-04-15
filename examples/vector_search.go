package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	
	"github.com/S-Corkum/mcp-server/pkg/client"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// Example of how an agent would use the vector functionality in the MCP server
func main() {
	// Initialize MCP client
	mcpClient := client.NewClient(
		"http://localhost:8080",
		client.WithAPIKey("your-api-key"),
	)
	
	// Initialize AWS Bedrock client for generating embeddings
	cfg, err := config.LoadDefaultConfig(context.Background(), 
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	
	bedrockClient := bedrockruntime.NewFromConfig(cfg)
	
	// Create a context for a conversation
	ctx := context.Background()
	
	// Step 1: Create a new conversation context in MCP
	conversationContext, err := mcpClient.CreateContext(ctx, &mcp.Context{
		AgentID:   "demo-agent",
		ModelID:   "anthropic.claude-3-sonnet-20240229-v1:0",
		SessionID: "user-123",
		MaxTokens: 100000,
		Content:   []mcp.ContextItem{},
		Metadata: map[string]interface{}{
			"user_id": "user123",
			"source":  "demo",
		},
	})
	if err != nil {
		log.Fatalf("Failed to create context: %v", err)
	}
	
	log.Printf("Created context with ID: %s", conversationContext.ID)
	
	// Step 2: Add some messages to the context
	messages := []mcp.ContextItem{
		{
			Role:    "system",
			Content: "You are a helpful DevOps assistant.",
			Tokens:  12,
		},
		{
			Role:    "user",
			Content: "What's the best way to set up continuous deployment for a Go application?",
			Tokens:  15,
		},
		{
			Role:    "assistant",
			Content: "For a Go application, I recommend using GitHub Actions with a proper workflow that builds, tests, and deploys your application. You can set up a simple workflow that runs go test and then deploys to your target environment when tests pass.",
			Tokens:  40,
		},
		{
			Role:    "user",
			Content: "How should I handle secrets in the CI/CD pipeline?",
			Tokens:  11,
		},
		{
			Role:    "assistant",
			Content: "You should use GitHub Secrets or environment variables in your CI/CD system to store sensitive information. Never hardcode secrets in your workflow files or application code. For production systems, consider using a dedicated secrets management solution like HashiCorp Vault or AWS Secrets Manager.",
			Tokens:  45,
		},
	}
	
	// Add these messages to the context
	conversationContext.Content = append(conversationContext.Content, messages...)
	
	// Update the context in MCP
	conversationContext, err = mcpClient.UpdateContext(ctx, conversationContext.ID, conversationContext, nil)
	if err != nil {
		log.Fatalf("Failed to update context: %v", err)
	}
	
	// Step 3: Generate embeddings for each message in the context
	for i, message := range conversationContext.Content {
		if message.Role == "user" || message.Role == "assistant" {
			// Generate embedding for the message text
			embedding, err := generateEmbedding(ctx, bedrockClient, message.Content)
			if err != nil {
				log.Printf("Failed to generate embedding for message %d: %v", i, err)
				continue
			}
			
			// Store the embedding in MCP
			err = storeEmbedding(
				conversationContext.ID,
				i,
				message.Content,
				embedding,
				"amazon.titan-embed-text-v1",
				"your-api-key",
			)
			if err != nil {
				log.Printf("Failed to store embedding for message %d: %v", i, err)
				continue
			}
			
			log.Printf("Stored embedding for message %d", i)
		}
	}
	
	// Step 4: Later, when the user asks a new question, search for relevant context
	userQuestion := "How do I configure GitHub secrets for my pipeline?"
	
	// Generate embedding for the question
	questionEmbedding, err := generateEmbedding(ctx, bedrockClient, userQuestion)
	if err != nil {
		log.Fatalf("Failed to generate embedding for question: %v", err)
	}
	
	// Search for similar embeddings in the context
	similarEmbeddings, err := searchEmbeddings(
		conversationContext.ID,
		questionEmbedding,
		5,
		"your-api-key",
	)
	if err != nil {
		log.Fatalf("Failed to search embeddings: %v", err)
	}
	
	// Extract the relevant context items
	relevantContextItems := []mcp.ContextItem{}
	for _, emb := range similarEmbeddings {
		if emb.ContentIndex < len(conversationContext.Content) {
			relevantContextItems = append(relevantContextItems, conversationContext.Content[emb.ContentIndex])
		}
	}
	
	// Print the relevant context items
	fmt.Println("\nUser question:", userQuestion)
	fmt.Println("\nRelevant context items:")
	for i, item := range relevantContextItems {
		fmt.Printf("%d. [%s]: %s\n", i+1, item.Role, item.Content)
	}
	
	// Step 5: Use the relevant context to enhance the next model call
	// This would typically involve calling a model like Claude with the relevant context
	
	fmt.Println("\nThe agent would now use these relevant context items to provide a more focused response")
}

// generateEmbedding generates an embedding for text using Bedrock
func generateEmbedding(ctx context.Context, client *bedrockruntime.Client, text string) ([]float32, error) {
	// Use the Amazon Titan Embeddings model
	modelID := "amazon.titan-embed-text-v1"
	
	// Prepare the request
	request := map[string]interface{}{
		"inputText": text,
	}
	
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	
	input := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		Body:        jsonBody,
		ContentType: aws.String("application/json"),
	}
	
	response, err := client.InvokeModel(ctx, input)
	if err != nil {
		return nil, err
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal(response.Body, &result); err != nil {
		return nil, err
	}
	
	// Extract the embedding
	embeddingRaw, ok := result["embedding"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid embedding in response")
	}
	
	embedding := make([]float32, len(embeddingRaw))
	for i, v := range embeddingRaw {
		if f, ok := v.(float64); ok {
			embedding[i] = float32(f)
		}
	}
	
	return embedding, nil
}

// storeEmbedding stores an embedding in the MCP server
func storeEmbedding(contextID string, contentIndex int, text string, embedding []float32, modelID, apiKey string) error {
	// Prepare the request
	reqBody := struct {
		ContextID    string    `json:"context_id"`
		ContentIndex int       `json:"content_index"`
		Text         string    `json:"text"`
		Embedding    []float32 `json:"embedding"`
		ModelID      string    `json:"model_id"`
	}{
		ContextID:    contextID,
		ContentIndex: contentIndex,
		Text:         text,
		Embedding:    embedding,
		ModelID:      modelID,
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	
	// Send the request to the MCP server
	req, err := http.NewRequest("POST", "http://localhost:8080/api/v1/vectors/store", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to store embedding: %s", resp.Status)
	}
	
	return nil
}

// Embedding represents the response from the MCP server
type Embedding struct {
	ID           string    `json:"id"`
	ContextID    string    `json:"context_id"`
	ContentIndex int       `json:"content_index"`
	Text         string    `json:"text"`
	ModelID      string    `json:"model_id"`
}

// searchEmbeddings searches for similar embeddings in the MCP server
func searchEmbeddings(contextID string, queryEmbedding []float32, limit int, apiKey string) ([]Embedding, error) {
	// Prepare the request
	reqBody := struct {
		ContextID      string    `json:"context_id"`
		QueryEmbedding []float32 `json:"query_embedding"`
		Limit          int       `json:"limit"`
	}{
		ContextID:      contextID,
		QueryEmbedding: queryEmbedding,
		Limit:          limit,
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	
	// Send the request to the MCP server
	req, err := http.NewRequest("POST", "http://localhost:8080/api/v1/vectors/search", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to search embeddings: %s", resp.Status)
	}
	
	// Parse the response
	var searchResult struct {
		Embeddings []Embedding `json:"embeddings"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, err
	}
	
	return searchResult.Embeddings, nil
}
