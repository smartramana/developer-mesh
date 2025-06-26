package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/lib/pq"
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
	// Get database connection info from environment
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbName := os.Getenv("DATABASE_NAME")
	if dbName == "" {
		dbName = "devops_mcp_dev"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "dbadmin"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	sslMode := os.Getenv("DATABASE_SSL_MODE")
	if sslMode == "" {
		sslMode = "require"
	}

	// Connect to database
	// Use a more compatible connection string format
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, sslMode)
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Set search path
	_, err = db.Exec("SET search_path TO mcp, public")
	if err != nil {
		t.Fatalf("Failed to set search path: %v", err)
	}

	testData := &TestData{
		TenantID: "00000000-0000-0000-0000-000000000001",
	}

	// Create models using REST API
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
		// Fallback to direct database insert
		var modelID string
		err = db.QueryRow(`
			INSERT INTO models (tenant_id, name, description, provider, model_type, metadata)
			VALUES ($1, 'test-model', 'Test model for functional tests', 'test', 'language', '{}')
			ON CONFLICT (tenant_id, name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
			RETURNING id
		`, testData.TenantID).Scan(&modelID)
		if err != nil {
			t.Fatalf("Failed to create test model: %v", err)
		}
		testData.ModelIDs = append(testData.ModelIDs, modelID)
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
		agentReq := map[string]interface{}{
			"name":         agent.Name,
			"description":  fmt.Sprintf("Test agent %s", agent.Name),
			"model_id":     testData.ModelIDs[0],
			"capabilities": agent.Capabilities,
			"config":       map[string]interface{}{},
			"metadata":     map[string]interface{}{},
		}
		
		agentResp, err := makeAPIRequest(t, http.MethodPost, apiURL+"/api/v1/agents", apiKey, agentReq)
		if err != nil {
			t.Logf("Failed to create agent %s via API: %v", agent.Name, err)
			// Fallback to direct database insert
			var agentID string
			err = db.QueryRow(`
				INSERT INTO agents (tenant_id, name, description, model_id, capabilities, config, metadata)
				VALUES ($1, $2, $3, $4, $5, '{}', '{}')
				ON CONFLICT (tenant_id, name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
				RETURNING id
			`, testData.TenantID, agent.Name, fmt.Sprintf("Test agent %s", agent.Name), 
			   testData.ModelIDs[0], pq.Array(agent.Capabilities)).Scan(&agentID)
			if err != nil {
				t.Logf("Failed to create test agent %s: %v", agent.Name, err)
			} else {
				testData.AgentIDs = append(testData.AgentIDs, agentID)
			}
		} else {
			if id, ok := agentResp["id"].(string); ok {
				testData.AgentIDs = append(testData.AgentIDs, id)
			}
		}
	}

	// Cleanup function
	cleanup := func() {
		// Clean up in reverse order of creation
		for _, taskID := range testData.TaskIDs {
			_, _ = db.Exec("DELETE FROM tasks WHERE id = $1", taskID)
		}
		for _, workspaceID := range testData.WorkspaceIDs {
			_, _ = db.Exec("DELETE FROM workspaces WHERE id = $1", workspaceID)
		}
		for _, workflowID := range testData.WorkflowIDs {
			_, _ = db.Exec("DELETE FROM workflows WHERE id = $1", workflowID)
		}
		for _, agentID := range testData.AgentIDs {
			_, _ = db.Exec("DELETE FROM agents WHERE id = $1", agentID)
		}
		for _, modelID := range testData.ModelIDs {
			_, _ = db.Exec("DELETE FROM models WHERE id = $1", modelID)
		}
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

	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

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

// WaitForAgent waits for an agent to be registered and available
func WaitForAgent(ctx context.Context, agentID string, timeout time.Duration) error {
	// This would check if the agent is properly registered in the system
	// For now, just wait a bit to ensure registration is processed
	select {
	case <-time.After(100 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}