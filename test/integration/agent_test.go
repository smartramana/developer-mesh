package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/pkg/client"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/stretchr/testify/assert"
)

// TestAgentWithToolAndContext tests the full workflow of an AI agent
// using both context management and tool integration
func TestAgentWithToolAndContext(t *testing.T) {
	// Only run if integration tests are enabled
	if os.Getenv("ENABLE_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set ENABLE_INTEGRATION_TESTS=true to run")
	}

	// Get server address from environment or use default
	serverAddr := os.Getenv("MCP_SERVER_ADDR")
	if serverAddr == "" {
		serverAddr = "http://localhost:8080"
	}

	// Initialize MCP client
	mcpClient := client.NewClient(
		serverAddr,
		client.WithAPIKey(os.Getenv("MCP_API_KEY")),
		client.WithWebhookSecret(os.Getenv("MCP_WEBHOOK_SECRET")),
	)

	// Create a test context
	ctx := context.Background()
	testContext := &mcp.Context{
		AgentID:   "test-agent",
		ModelID:   "test-model",
		SessionID: fmt.Sprintf("test-session-%d", time.Now().UnixNano()),
		MaxTokens: 10000,
		Content:   []mcp.ContextItem{},
		Metadata: map[string]interface{}{
			"test": true,
		},
	}

	// Step 1: Create context
	t.Log("Creating context...")
	createdContext, err := mcpClient.CreateContext(ctx, testContext)
	assert.NoError(t, err)
	assert.NotEmpty(t, createdContext.ID)
	contextID := createdContext.ID
	t.Logf("Created context with ID: %s", contextID)

	// Step 2: Add messages to context
	t.Log("Adding messages to context...")
	systemMessage := mcp.ContextItem{
		Role:      "system",
		Content:   "You are a helpful DevOps assistant.",
		Timestamp: time.Now(),
		Tokens:    14,
	}
	userMessage := mcp.ContextItem{
		Role:      "user",
		Content:   "I need help with GitHub.",
		Timestamp: time.Now(),
		Tokens:    6,
	}

	createdContext.Content = append(createdContext.Content, systemMessage, userMessage)
	updatedContext, err := mcpClient.UpdateContext(ctx, contextID, createdContext, nil)
	assert.NoError(t, err)
	assert.Len(t, updatedContext.Content, 2)

	// Step 3: List available tools
	t.Log("Listing available tools...")
	tools, err := mcpClient.ListTools(ctx)
	assert.NoError(t, err)
	t.Logf("Available tools: %v", tools)

	// Some environments might not have all tools configured,
	// so we'll check if GitHub is available before testing it
	var githubAvailable bool
	for _, tool := range tools {
		if tool == "github" {
			githubAvailable = true
			break
		}
	}

	// Step 4: Execute tool action if GitHub is available
	if githubAvailable {
		t.Log("Executing GitHub tool action...")
		// In a real test, we'd use a real GitHub repo and action
		// For this test, we'll just send a mock action that the server should handle
		// even if it can't actually create a GitHub issue
		params := map[string]interface{}{
			"owner": "test-owner",
			"repo":  "test-repo",
			"title": "Test Issue",
			"body":  "This is a test issue created by the integration test.",
		}

		result, err := mcpClient.ExecuteToolAction(ctx, contextID, "github", "create_issue", params)
		// The action might fail if GitHub isn't properly configured, but we still want to make sure
		// the API itself works, so we'll just log the error rather than failing the test
		if err != nil {
			t.Logf("GitHub action failed (may be expected in test environment): %v", err)
		} else {
			t.Logf("GitHub action result: %v", result)
		}
		
		// Test safety restrictions - try to delete a repository (should be blocked)
		t.Log("Testing safety restrictions - trying to delete a repository...")
		deleteParams := map[string]interface{}{
			"owner": "test-owner",
			"repo":  "test-repo",
		}
		
		_, err = mcpClient.ExecuteToolAction(ctx, contextID, "github", "delete_repository", deleteParams)
		// This should fail with a safety restriction error
		if err != nil {
			t.Logf("Expected error when trying to delete repository: %v", err)
			if !strings.Contains(err.Error(), "safety") && !strings.Contains(err.Error(), "restricted") {
				t.Errorf("Expected safety restriction error, got: %v", err)
			}
		} else {
			t.Errorf("Repository deletion should have been blocked by safety restrictions")
		}
		
		// Test allowed operation - try to archive a repository (should be allowed)
		t.Log("Testing allowed operation - trying to archive a repository...")
		archiveParams := map[string]interface{}{
			"owner": "test-owner",
			"repo":  "test-repo",
			"archived": true,
		}
		
		_, err = mcpClient.ExecuteToolAction(ctx, contextID, "github", "archive_repository", archiveParams)
		// The action might fail if GitHub isn't properly configured, but we should not get a safety restriction error
		if err != nil && (strings.Contains(err.Error(), "safety") || strings.Contains(err.Error(), "restricted")) {
			t.Errorf("Archive operation should be allowed but got safety restriction: %v", err)
		} else {
			t.Log("Archive operation was correctly allowed (actual GitHub error may be expected)")
		}
	} else {
		t.Log("GitHub tool not available, skipping tool action test")
	}

	// Test Artifactory restrictions
	t.Log("Testing Artifactory safety restrictions (read-only access)...")
	
	// Test allowed read operation
	readParams := map[string]interface{}{
		"repo": "test-repo",
		"path": "test/artifact.jar",
	}
	
	_, err = mcpClient.ExecuteToolAction(ctx, contextID, "artifactory", "get_artifact", readParams)
	// The action might fail if Artifactory isn't properly configured, but we should not get a safety restriction error
	if err != nil && (strings.Contains(err.Error(), "safety") || strings.Contains(err.Error(), "restricted")) {
		t.Errorf("Read operation should be allowed but got safety restriction: %v", err)
	} else {
		t.Log("Read operation was correctly allowed (actual Artifactory error may be expected)")
	}
	
	// Test restricted write operation
	writeParams := map[string]interface{}{
		"repo": "test-repo",
		"path": "test/artifact.jar",
		"content": "test content",
	}
	
	_, err = mcpClient.ExecuteToolAction(ctx, contextID, "artifactory", "upload_artifact", writeParams)
	// This should fail with a safety restriction error
	if err != nil {
		t.Logf("Expected error when trying to upload artifact: %v", err)
		if !strings.Contains(err.Error(), "safety") && !strings.Contains(err.Error(), "restricted") {
			t.Errorf("Expected safety restriction error, got: %v", err)
		}
	} else {
		t.Errorf("Artifact upload should have been blocked by safety restrictions")
	}
	
	// Step 5: Send an event
	t.Log("Sending event...")
	event := &mcp.Event{
		Source:    "test",
		Type:      "test_event",
		AgentID:   "test-agent",
		SessionID: testContext.SessionID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"context_id": contextID,
			"test":       true,
		},
	}

	err = mcpClient.SendEvent(ctx, event)
	assert.NoError(t, err)

	// Step 6: Retrieve and verify context
	t.Log("Retrieving context...")
	retrievedContext, err := mcpClient.GetContext(ctx, contextID)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedContext)
	assert.Equal(t, "test-agent", retrievedContext.AgentID)
	assert.Equal(t, "test-model", retrievedContext.ModelID)
	assert.True(t, len(retrievedContext.Content) >= 2)

	// Pretty print the retrieved context
	contextJSON, _ := json.MarshalIndent(retrievedContext, "", "  ")
	t.Logf("Retrieved context: %s", string(contextJSON))

	// Step 7: Clean up - delete context
	t.Log("Deleting context...")
	err = mcpClient.DeleteContext(ctx, contextID)
	assert.NoError(t, err)

	// Verify context is deleted
	_, err = mcpClient.GetContext(ctx, contextID)
	assert.Error(t, err)
	t.Log("Context successfully deleted")
}
