package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/S-Corkum/mcp-server/pkg/client"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// Example of using S3 storage and vector search together for advanced context management
func main() {
	// Initialize MCP client
	apiKey := "your-api-key"
	mcpClient := client.NewClient(
		"http://localhost:8080",
		client.WithAPIKey(apiKey),
	)
	
	// Initialize AWS Bedrock client for model inference and embeddings
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
	// This context will be stored in S3 if the server is configured to use S3 storage
	conversationContext, err := mcpClient.CreateContext(ctx, &mcp.Context{
		AgentID:   "advanced-demo-agent",
		ModelID:   "anthropic.claude-3-sonnet-20240229-v1:0",
		SessionID: "user-789",
		MaxTokens: 500000, // Large context that will be stored in S3
		Content:   []mcp.ContextItem{},
		Metadata: map[string]interface{}{
			"user_id": "user789",
			"source":  "advanced-demo",
			"topic":   "software engineering",
		},
	})
	if err != nil {
		log.Fatalf("Failed to create context: %v", err)
	}
	
	log.Printf("Created context with ID: %s", conversationContext.ID)
	
	// Step 2: Add some initial messages to the context
	messages := []mcp.ContextItem{
		{
			Role:      "system",
			Content:   "You are a helpful expert on software engineering topics, providing comprehensive information about best practices, design patterns, and modern techniques.",
			Timestamp: time.Now(),
			Tokens:    30,
		},
		{
			Role:      "user",
			Content:   "What are the key principles of microservice architecture?",
			Timestamp: time.Now(),
			Tokens:    10,
		},
		{
			Role:      "assistant",
			Content:   "Microservice architecture is an approach to software development where applications are built as a collection of loosely coupled, independently deployable services. The key principles include:\n\n1. **Single Responsibility**: Each microservice should focus on doing one thing well.\n\n2. **Autonomy**: Services should be independently deployable and updatable without affecting other services.\n\n3. **Domain-Driven Design**: Services are organized around business capabilities rather than technical functions.\n\n4. **Resilience**: The system should be designed to handle failures gracefully, with circuit breakers, retries, and fallback mechanisms.\n\n5. **Decentralization**: Data management and governance are distributed, with each service owning its data.\n\n6. **API-First Design**: Clear, well-defined interfaces between services are critical.\n\n7. **DevOps Culture**: Automated testing, deployment, and monitoring are essential for managing the complexity.\n\n8. **Scalability**: Services can be scaled independently based on demand.\n\n9. **Event-Driven Communication**: Using events for asynchronous communication between services.\n\n10. **Observability**: Comprehensive logging, monitoring, and tracing across services.\n\nWould you like me to elaborate on any of these principles or discuss specific implementation challenges?",
			Timestamp: time.Now(),
			Tokens:    250,
		},
		{
			Role:      "user",
			Content:   "Can you explain more about service discovery in microservices?",
			Timestamp: time.Now(),
			Tokens:    12,
		},
		{
			Role:      "assistant",
			Content:   "Service Discovery in microservices is a critical mechanism that enables services to find and communicate with each other without hardcoded locations. Here's a comprehensive explanation:\n\n## What is Service Discovery?\n\nService Discovery is the process of automatically detecting services and their network locations in a distributed system. It's essential because:\n\n- Services can be dynamically scaled up or down\n- Containers and instances can be created, destroyed, or moved\n- IP addresses and ports can change frequently\n\n## Key Components\n\n1. **Service Registry**: A database that maintains a record of available service instances, their locations, health status, and metadata.\n\n2. **Registration Process**: How services register themselves in the registry when they start up.\n\n3. **Discovery Process**: How client services find and select appropriate service instances to connect to.\n\n4. **Health Checking**: Mechanisms to detect and remove unhealthy service instances from the registry.\n\n## Service Discovery Patterns\n\n### Client-Side Discovery\n\nIn this pattern:\n- Client services query the service registry directly\n- Clients select an available instance using load balancing algorithms\n- Clients connect directly to the chosen service instance\n\n**Advantages**:\n- Fewer network hops (more efficient)\n- Client can implement custom load balancing strategies\n\n**Disadvantages**:\n- Clients are coupled to the discovery mechanism\n- Each service language/framework needs its own discovery client library\n\n### Server-Side Discovery\n\nIn this pattern:\n- A load balancer/router sits between clients and services\n- Clients connect to the router using a stable URL\n- The router consults the service registry and forwards requests\n\n**Advantages**:\n- Discovery logic is centralized and not duplicated in clients\n- Clients are simpler and language-agnostic\n\n**Disadvantages**:\n- Extra network hop\n- The router can become a bottleneck or single point of failure\n\n## Popular Service Discovery Solutions\n\n1. **Consul**: Provides service discovery, health checking, and a distributed key-value store\n2. **etcd**: Distributed key-value store often used with custom service discovery solutions\n3. **ZooKeeper**: Centralized service for maintaining configuration and naming\n4. **Eureka**: Netflix's service discovery solution for the JVM ecosystem\n5. **Kubernetes Service Discovery**: Built-in DNS-based discovery and service abstractions\n6. **AWS Cloud Map**: Service discovery for AWS resources\n\n## Implementation Considerations\n\n1. **Caching**: Clients often cache discovery results to improve performance\n2. **Circuit Breaking**: Integrate with circuit breakers to handle failures gracefully\n3. **Timeout and Retry Policies**: Configure appropriate timeouts and retries\n4. **Cross-Region/Cross-Zone Awareness**: Prefer services in the same region/zone when possible\n5. **Metadata-Based Routing**: Route based on version, capabilities, or other metadata\n\nWould you like me to elaborate on any specific aspect of service discovery or discuss how it integrates with other microservice patterns?",
			Timestamp: time.Now(),
			Tokens:    550,
		},
	}
	
	// Add these messages to the context
	conversationContext.Content = append(conversationContext.Content, messages...)
	
	// Update the context in MCP
	conversationContext, err = mcpClient.UpdateContext(ctx, conversationContext.ID, conversationContext, nil)
	if err != nil {
		log.Fatalf("Failed to update context: %v", err)
	}
	
	log.Printf("Added initial messages to context")
	
	// Step 3: Generate embeddings for each message in the context
	log.Printf("Generating embeddings for context items...")
	
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
				apiKey,
			)
			if err != nil {
				log.Printf("Failed to store embedding for message %d: %v", i, err)
				continue
			}
			
			log.Printf("Stored embedding for message %d", i)
		}
	}
	
	// Step 4: Simulate a user asking a new question that's semantically related to previous content
	log.Printf("Simulating a new user question...")
	
	userQuestion := "What are some patterns for handling data consistency between microservices?"
	
	// Add the question to the context
	newUserMessage := mcp.ContextItem{
		Role:      "user",
		Content:   userQuestion,
		Timestamp: time.Now(),
		Tokens:    12,
	}
	
	conversationContext.Content = append(conversationContext.Content, newUserMessage)
	conversationContext, err = mcpClient.UpdateContext(ctx, conversationContext.ID, conversationContext, nil)
	if err != nil {
		log.Fatalf("Failed to update context with new question: %v", err)
	}
	
	// Step 5: Use vector search to find relevant context from previous messages
	// Generate embedding for the question
	questionEmbedding, err := generateEmbedding(ctx, bedrockClient, userQuestion)
	if err != nil {
		log.Fatalf("Failed to generate embedding for question: %v", err)
	}
	
	// Store the embedding for the new question
	err = storeEmbedding(
		conversationContext.ID,
		len(conversationContext.Content)-1,
		userQuestion,
		questionEmbedding,
		"amazon.titan-embed-text-v1",
		apiKey,
	)
	if err != nil {
		log.Printf("Failed to store embedding for question: %v", err)
	}
	
	// Search for similar embeddings in the context
	similarEmbeddings, err := searchEmbeddings(
		conversationContext.ID,
		questionEmbedding,
		3, // Find top 3 most relevant items
		apiKey,
	)
	if err != nil {
		log.Fatalf("Failed to search embeddings: %v", err)
	}
	
	// Extract the relevant context items
	relevantContextItems := []mcp.ContextItem{}
	relevantContextIndices := []int{}
	
	log.Printf("Found %d semantically relevant context items:", len(similarEmbeddings))
	for i, emb := range similarEmbeddings {
		if emb.ContentIndex < len(conversationContext.Content) {
			relevantContextItems = append(relevantContextItems, conversationContext.Content[emb.ContentIndex])
			relevantContextIndices = append(relevantContextIndices, emb.ContentIndex)
			log.Printf("%d. [%s] (score: %.4f): %s...", i+1, 
				conversationContext.Content[emb.ContentIndex].Role,
				emb.Similarity,
				truncateText(conversationContext.Content[emb.ContentIndex].Content, 50))
		}
	}
	
	// Step 6: Prepare a Bedrock request that includes the relevant context
	// In a real implementation, you would format the relevant context appropriately
	// For this example, we'll just prepare a standard model request
	
	// Prepare messages for Bedrock (Anthropic Claude format)
	anthropicMessages := []map[string]string{}
	var systemPrompt string
	
	// Add all messages from the context to maintain the conversation history
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
		"max_tokens":        1500,
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
	
	// Extract assistant response
	assistantContent := ""
	if content, ok := bedrockResponse["content"].([]interface{}); ok && len(content) > 0 {
		if contentItem, ok := content[0].(map[string]interface{}); ok {
			if text, ok := contentItem["text"].(string); ok {
				assistantContent = text
			}
		}
	}
	
	log.Printf("Received assistant response of length: %d characters", len(assistantContent))
	log.Printf("First 100 characters of response: %s...", truncateText(assistantContent, 100))
	
	// Add assistant response to context
	assistantMessage := mcp.ContextItem{
		Role:      "assistant",
		Content:   assistantContent,
		Timestamp: time.Now(),
		Tokens:    len(strings.Split(assistantContent, " ")), // Approximate token count
	}
	
	conversationContext.Content = append(conversationContext.Content, assistantMessage)
	
	// Update the context with the response
	conversationContext, err = mcpClient.UpdateContext(ctx, conversationContext.ID, conversationContext, nil)
	if err != nil {
		log.Fatalf("Failed to update context with response: %v", err)
	}
	
	// Generate embedding for the assistant response
	responseEmbedding, err := generateEmbedding(ctx, bedrockClient, assistantContent)
	if err != nil {
		log.Printf("Failed to generate embedding for response: %v", err)
	} else {
		// Store the embedding in MCP
		err = storeEmbedding(
			conversationContext.ID,
			len(conversationContext.Content)-1,
			assistantContent,
			responseEmbedding,
			"amazon.titan-embed-text-v1",
			apiKey,
		)
		if err != nil {
			log.Printf("Failed to store embedding for response: %v", err)
		} else {
			log.Printf("Stored embedding for assistant response")
		}
	}
	
	log.Printf("Advanced context management demonstration complete")
	log.Printf("Context ID: %s", conversationContext.ID)
	log.Printf("Total messages in context: %d", len(conversationContext.Content))
}

// Helper function to truncate text for display
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[0:maxLen] + "..."
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
	Similarity   float32   `json:"similarity,omitempty"`
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
