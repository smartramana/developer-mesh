package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestData holds references to test entities
type TestData struct {
	TenantID   string
	ModelIDs   []string
	AgentIDs   []string
	TaskIDs    []string
	WorkflowIDs []string
	WorkspaceIDs []string
}

// SetupTestData creates necessary test data in the database
func SetupTestData(t *testing.T) (*TestData, func()) {
	// Skip direct database connection - the MCP server and REST API
	// are already running with test data configured through API keys

	testData := &TestData{
		TenantID: "00000000-0000-0000-0000-000000000001",
	}

	// Create test data via API
	apiURL := os.Getenv("MCP_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8081"
	}
	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey == "" {
		apiKey = "dev-admin-key-1234567890"
	}

	// Create test model
	modelReq := map[string]interface{}{
		"name":        "test-model",
		"description": "Test model for functional tests",
		"provider":    "test",
		"model_type":  "language",
		"metadata":    map[string]interface{}{},
	}
	
	modelResp, err := makeAPIRequest(t, http.MethodPost, apiURL+"/api/v1/models", apiKey, modelReq)
	if err != nil {
		t.Logf("Failed to create model via API: %v", err)
		// Continue without model - tests may still work with existing data
	} else {
		if id, ok := modelResp["id"].(string); ok {
			testData.ModelIDs = append(testData.ModelIDs, id)
		}
	}

	// Create test agents with specific capabilities
	agents := []struct {
		Name         string
		Capabilities []string
	}{
		{"agent1", []string{"coding", "testing"}},
		{"agent2", []string{"documentation", "testing"}},
		{"agent3", []string{"collaboration"}},
		{"coder", []string{"coding", "debugging"}},
		{"tester", []string{"testing", "qa"}},
		{"reviewer", []string{"review", "documentation"}},
	}

	for _, agent := range agents {
		modelID := ""
		if len(testData.ModelIDs) > 0 {
			modelID = testData.ModelIDs[0]
		}
		
		agentReq := map[string]interface{}{
			"name":         agent.Name,
			"description":  fmt.Sprintf("Test agent %s", agent.Name),
			"model_id":     modelID,
			"capabilities": agent.Capabilities,
			"config":       map[string]interface{}{},
			"metadata":     map[string]interface{}{},
		}
		
		agentResp, err := makeAPIRequest(t, http.MethodPost, apiURL+"/api/v1/agents", apiKey, agentReq)
		if err != nil {
			t.Logf("Failed to create agent %s via API: %v", agent.Name, err)
			// Continue - agent may already exist
		} else {
			if id, ok := agentResp["id"].(string); ok {
				testData.AgentIDs = append(testData.AgentIDs, id)
			}
		}
	}

	// Cleanup function
	cleanup := func() {
		// Cleanup via API if needed
		// For now, we'll leave test data in place
		t.Log("Test cleanup completed")
	}

	return testData, cleanup
}

// makeAPIRequest is a helper to make API requests
func makeAPIRequest(t *testing.T, method, url, apiKey string, body interface{}) (map[string]interface{}, error) {
	var reqBody []byte
	var err error
	
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API request failed with status %d: %v", resp.StatusCode, result)
	}

	return result, nil
}

// CreateTestModel creates a test model via API
func CreateTestModel(ctx context.Context, apiURL, apiKey, tenantID string) (map[string]interface{}, error) {
	modelReq := map[string]interface{}{
		"name":        "test-model",
		"description": "Test model for functional tests",
		"provider":    "test",
		"model_type":  "language",
		"metadata":    map[string]interface{}{},
	}

	reqBody, err := json.Marshal(modelReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/api/v1/models", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var modelResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&modelResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return modelResp, nil
}

// CreateTestAgent creates a test agent via API
func CreateTestAgent(ctx context.Context, apiURL, apiKey, tenantID, modelID, name string, capabilities []string) (map[string]interface{}, error) {
	agentReq := map[string]interface{}{
		"name":         name,
		"description":  fmt.Sprintf("Test agent %s", name),
		"model_id":     modelID,
		"capabilities": capabilities,
		"config":       map[string]interface{}{},
		"metadata":     map[string]interface{}{},
	}

	reqBody, err := json.Marshal(agentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/api/v1/agents", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var agentResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&agentResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return agentResp, nil
}