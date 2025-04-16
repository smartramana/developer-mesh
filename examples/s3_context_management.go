package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/S-Corkum/mcp-server/pkg/client"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// Example of using S3 context storage with MCP Server
func main() {
	// Initialize MCP client
	mcpClient := client.NewClient(
		"http://localhost:8080",
		client.WithAPIKey("your-api-key"),
	)
	
	// Initialize AWS Bedrock client for model inference
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
	// The MCP server will automatically use S3 storage if configured
	conversationContext, err := mcpClient.CreateContext(ctx, &mcp.Context{
		AgentID:   "s3-storage-demo-agent",
		ModelID:   "anthropic.claude-3-sonnet-20240229-v1:0",
		SessionID: "user-456",
		// Setting a large max tokens value
		// This will be stored efficiently in S3 storage
		MaxTokens: 1000000,
		Content:   []mcp.ContextItem{},
		Metadata: map[string]interface{}{
			"user_id": "user456",
			"source":  "s3-demo",
		},
	})
	if err != nil {
		log.Fatalf("Failed to create context: %v", err)
	}
	
	log.Printf("Created context with ID: %s", conversationContext.ID)
	
	// Step 2: Generate a large amount of content to demonstrate S3 storage efficiency
	// Add system message to context
	systemMessage := mcp.ContextItem{
		Role:      "system",
		Content:   "You are a helpful assistant that provides detailed information about various topics.",
		Timestamp: time.Now(),
		Tokens:    20,
	}
	
	conversationContext.Content = append(conversationContext.Content, systemMessage)
	
	// Add user message to context
	userMessage := mcp.ContextItem{
		Role:      "user",
		Content:   "I'd like a detailed explanation of how quantum computing works, including the mathematics behind it.",
		Timestamp: time.Now(),
		Tokens:    20,
	}
	
	conversationContext.Content = append(conversationContext.Content, userMessage)
	
	// Update the context with these initial messages
	conversationContext, err = mcpClient.UpdateContext(ctx, conversationContext.ID, conversationContext, nil)
	if err != nil {
		log.Fatalf("Failed to update context: %v", err)
	}
	
	// Step 3: Generate a model response (this would be a very large response in a real scenario)
	// Prepare messages for Bedrock (Anthropic Claude format)
	anthropicMessages := []map[string]string{}
	var systemPrompt string
	
	for _, item := range conversationContext.Content {
		if item.Role == "system" {
			systemPrompt = item.Content
		} else {
			anthropicMessages = append(anthropicMessages, map[string]string{
				"role":    item.Role,
				"content": item.Content,
			})
		}
	}
	
	// Create Bedrock request
	bedrockRequest := map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		// Request a long response to demonstrate large context storage
		"max_tokens":        4000,
		"temperature":       0.7,
		"messages":          anthropicMessages,
	}
	
	if systemPrompt != "" {
		bedrockRequest["system"] = systemPrompt
	}
	
	// Convert to JSON
	requestJSON, err := json.Marshal(bedrockRequest)
	if err != nil {
		log.Fatalf("Failed to marshal request: %v", err)
	}
	
	// Send request to Bedrock
	response, err := bedrockClient.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String("anthropic.claude-3-sonnet-20240229-v1:0"),
		Body:        requestJSON,
		ContentType: aws.String("application/json"),
	})
	
	if err != nil {
		log.Fatalf("Failed to invoke model: %v", err)
	}
	
	// Parse response
	var bedrockResponse map[string]interface{}
	if err := json.Unmarshal(response.Body, &bedrockResponse); err != nil {
		log.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	// Extract assistant response (this would be a large response)
	assistantContent := ""
	if content, ok := bedrockResponse["content"].([]interface{}); ok && len(content) > 0 {
		if contentItem, ok := content[0].(map[string]interface{}); ok {
			if text, ok := contentItem["text"].(string); ok {
				assistantContent = text
			}
		}
	}
	
	log.Printf("Received assistant response of length: %d characters", len(assistantContent))
	
	// Add assistant response to context
	assistantMessage := mcp.ContextItem{
		Role:      "assistant",
		Content:   assistantContent,
		Timestamp: time.Now(),
		Tokens:    len(strings.Split(assistantContent, " ")), // Approximate token count
	}
	
	conversationContext.Content = append(conversationContext.Content, assistantMessage)
	
	// Update the context with the large assistant response
	// This will be automatically stored in S3 by the MCP server
	conversationContext, err = mcpClient.UpdateContext(ctx, conversationContext.ID, conversationContext, nil)
	if err != nil {
		log.Fatalf("Failed to update context with large response: %v", err)
	}
	
	log.Printf("Successfully stored large context in S3")
	
	// Step 4: Demonstrate retrieving the large context from S3
	retrievedContext, err := mcpClient.GetContext(ctx, conversationContext.ID)
	if err != nil {
		log.Fatalf("Failed to retrieve context from S3: %v", err)
	}
	
	log.Printf("Successfully retrieved context from S3 with %d messages", len(retrievedContext.Content))
	
	// Step 5: Simulate a conversation that builds up a very large context over time
	for i := 0; i < 10; i++ {
		// Add new user message
		userFollowup := mcp.ContextItem{
			Role:      "user",
			Content:   fmt.Sprintf("Follow-up question %d: Can you explain more about the implications for cryptography?", i+1),
			Timestamp: time.Now(),
			Tokens:    15,
		}
		
		retrievedContext.Content = append(retrievedContext.Content, userFollowup)
		retrievedContext, err = mcpClient.UpdateContext(ctx, retrievedContext.ID, retrievedContext, nil)
		if err != nil {
			log.Fatalf("Failed to update context with follow-up question: %v", err)
		}
		
		// In a real implementation, you would get a model response here and add it to the context
		// For demonstration purposes, we'll just add a dummy response
		dummyResponse := mcp.ContextItem{
			Role:      "assistant",
			Content:   fmt.Sprintf("This is a detailed response to follow-up question %d about quantum computing implications for cryptography. [Imagine 1000+ words of detailed explanation here]", i+1),
			Timestamp: time.Now(),
			Tokens:    1500, // Simulated large response
		}
		
		retrievedContext.Content = append(retrievedContext.Content, dummyResponse)
		retrievedContext, err = mcpClient.UpdateContext(ctx, retrievedContext.ID, retrievedContext, nil)
		if err != nil {
			log.Fatalf("Failed to update context with dummy response: %v", err)
		}
		
		log.Printf("Added round %d of conversation to S3 context", i+1)
	}
	
	// Step 6: Clean up by deleting the context
	err = mcpClient.DeleteContext(ctx, retrievedContext.ID)
	if err != nil {
		log.Fatalf("Failed to delete context: %v", err)
	}
	
	log.Printf("Successfully deleted context and associated S3 objects")
}
