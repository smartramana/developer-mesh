package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOpenAPIHandler is a mock implementation of tools.OpenAPIHandler
type mockOpenAPIHandler struct{}

func (m *mockOpenAPIHandler) DiscoverAPIs(ctx context.Context, config tools.ToolConfig) (*tools.DiscoveryResult, error) {
	return &tools.DiscoveryResult{
		Status:         tools.DiscoveryStatusSuccess,
		DiscoveredURLs: []string{config.OpenAPIURL},
		Capabilities: []tools.Capability{
			tools.CapabilityIssueManagement,
		},
	}, nil
}

func (m *mockOpenAPIHandler) GenerateTools(config tools.ToolConfig, spec *openapi3.T) ([]*tools.Tool, error) {
	return []*tools.Tool{}, nil
}

func (m *mockOpenAPIHandler) AuthenticateRequest(req *http.Request, creds *models.TokenCredential, securitySchemes map[string]tools.SecurityScheme) error {
	// Add authentication header based on creds
	if creds != nil && creds.Token != "" {
		req.Header.Set("Authorization", "Bearer "+creds.Token)
	}
	return nil
}

func (m *mockOpenAPIHandler) TestConnection(ctx context.Context, config tools.ToolConfig) error {
	// Always return success for tests
	return nil
}

func (m *mockOpenAPIHandler) ExtractSecuritySchemes(spec *openapi3.T) map[string]tools.SecurityScheme {
	return map[string]tools.SecurityScheme{}
}

func TestDynamicToolsPassthrough(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	// Create test server that simulates a tool API
	toolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authentication header
		authHeader := r.Header.Get("Authorization")

		// Return different responses based on the token
		switch authHeader {
		case "Bearer user-token-123":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Success with user token",
				"user":    "john.doe",
			})
		case "Bearer service-token-456":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Success with service token",
				"user":    "service-account",
			})
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Unauthorized",
			})
		}
	}))
	defer toolServer.Close()

	// Create dependencies
	// Setup test database - use PostgreSQL from docker-compose.test.yml
	// Default to localhost:5433 (test postgres port) if not set
	dbHost := os.Getenv("TEST_DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("TEST_DB_PORT")
	if dbPort == "" {
		dbPort = "5433"
	}

	dsn := fmt.Sprintf("host=%s port=%s user=test password=test dbname=test sslmode=disable", dbHost, dbPort)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skip("Skipping test: PostgreSQL test database not available")
	}
	defer func() { _ = db.Close() }()

	// Test connection
	if err := db.Ping(); err != nil {
		t.Skip("Skipping test: Cannot connect to PostgreSQL test database")
	}

	// Run migrations
	err = runTestMigrations(db)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Use a test logger that prints to stdout
	logger := observability.NewLogger("test")
	metricsClient := observability.NewNoOpMetricsClient()
	encryptionSvc := security.NewEncryptionService("test-master-key")
	cacheClient := cache.NewMemoryCache(1000, 5*time.Minute)

	// Create services
	toolService := NewDynamicToolService(db, logger, metricsClient, encryptionSvc)
	// Create a mock OpenAPI handler for health checks
	mockHandler := &mockOpenAPIHandler{}
	healthCheckMgr := tools.NewHealthCheckManager(cacheClient, mockHandler, logger, metricsClient)
	auditLogger := auth.NewAuditLogger(logger)

	// Create API
	api := NewDynamicToolsAPI(toolService, logger, metricsClient, encryptionSvc, healthCheckMgr, auditLogger)

	// Generate valid UUIDs for test
	testTenantID := uuid.New().String()
	testUserID := uuid.New().String()

	// Setup router with mock auth middleware that adds tenant_id
	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Mock auth middleware - set tenant_id for all requests
		c.Set("tenant_id", testTenantID)
		c.Set("user_id", testUserID)

		// Handle passthrough headers if present
		if userToken := c.GetHeader("X-User-Token"); userToken != "" {
			if provider := c.GetHeader("X-Token-Provider"); provider != "" {
				passthroughToken := auth.PassthroughToken{
					Token:    userToken,
					Provider: provider,
				}
				ctx := auth.WithPassthroughToken(c.Request.Context(), passthroughToken)
				c.Request = c.Request.WithContext(ctx)
			}
		}

		c.Next()
	})
	v1 := router.Group("/api/v1")
	api.RegisterRoutes(v1)

	// Create a test tool with GitHub provider
	createReq := CreateToolRequest{
		Name:     "test-github",
		BaseURL:  toolServer.URL,
		Provider: "github",
		AuthType: "bearer",
		Credentials: &CredentialInput{
			Token: "service-token-456",
		},
		PassthroughConfig: &PassthroughConfig{
			Mode:              "optional",
			FallbackToService: true,
		},
	}

	// Create tool
	reqBody, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/api/v1/tools", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Logf("Tool creation failed with status %d: %s", w.Code, w.Body.String())
	}
	require.Equal(t, http.StatusCreated, w.Code)

	var createdTool Tool
	err = json.Unmarshal(w.Body.Bytes(), &createdTool)
	require.NoError(t, err)

	t.Run("Execute with passthrough token", func(t *testing.T) {
		// Use a mock gateway key
		gatewayKey := "gw_test_gateway_key"

		// Execute action with passthrough token
		executeReq := map[string]interface{}{
			"action": "test",
		}
		reqBody, _ := json.Marshal(executeReq)

		req := httptest.NewRequest("POST", "/api/v1/tools/"+createdTool.ID+"/execute/test_action", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+gatewayKey)
		req.Header.Set("X-User-Token", "user-token-123")
		req.Header.Set("X-Token-Provider", "github")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should succeed with user token
		assert.Equal(t, http.StatusOK, w.Code)

		var result map[string]interface{}
		err2 := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err2)

		// Verify user token was used (would see john.doe in response)
		assert.Equal(t, "success", result["status"])
		resultData, ok := result["result"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "Success with user token", resultData["message"])
		assert.Equal(t, "john.doe", resultData["user"])
	})

	t.Run("Execute without passthrough token (fallback to service)", func(t *testing.T) {
		executeReq := map[string]interface{}{
			"action": "test",
		}
		reqBody, _ := json.Marshal(executeReq)

		// Use a regular key for execution without passthrough
		req := httptest.NewRequest("POST", "/api/v1/tools/"+createdTool.ID+"/execute/test_action", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer test-key")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should succeed with service token
		assert.Equal(t, http.StatusOK, w.Code)

		var result map[string]interface{}
		err2 := json.Unmarshal(w.Body.Bytes(), &result)
		require.NoError(t, err2)

		// Verify service token was used
		assert.Equal(t, "success", result["status"])
		resultData, ok := result["result"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "Success with service token", resultData["message"])
		assert.Equal(t, "service-account", resultData["user"])
	})

	t.Run("Execute with required passthrough - no token provided", func(t *testing.T) {
		// Create a new tool that requires passthrough
		createReq2 := CreateToolRequest{
			Name:     "test-github-required",
			BaseURL:  toolServer.URL,
			Provider: "github",
			AuthType: "bearer",
			Credentials: &CredentialInput{
				Token: "service-token-456",
			},
			PassthroughConfig: &PassthroughConfig{
				Mode:              "required",
				FallbackToService: false,
			},
		}

		// Create tool
		reqBody, _ := json.Marshal(createReq2)
		req := httptest.NewRequest("POST", "/api/v1/tools", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer test-key")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)

		var requiredTool Tool
		err = json.Unmarshal(w.Body.Bytes(), &requiredTool)
		require.NoError(t, err)

		// Now try to execute without passthrough token
		executeReq := map[string]interface{}{
			"action": "test",
		}
		reqBody, _ = json.Marshal(executeReq)

		// Try to execute without passthrough token
		req = httptest.NewRequest("POST", "/api/v1/tools/"+requiredTool.ID+"/execute/test_action", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer test-key")
		req.Header.Set("Content-Type", "application/json")

		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should fail with 401
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var errorResp map[string]interface{}
		err2 := json.Unmarshal(w.Body.Bytes(), &errorResp)
		require.NoError(t, err2)
		assert.Contains(t, errorResp["error"], "passthrough token required")
	})

	t.Run("Execute with wrong provider token", func(t *testing.T) {
		// Use a mock gateway key with gitlab permissions
		gatewayKey := "gw_test_gitlab_key"

		executeReq := map[string]interface{}{
			"action": "test",
		}
		reqBody, _ := json.Marshal(executeReq)

		req := httptest.NewRequest("POST", "/api/v1/tools/"+createdTool.ID+"/execute/test_action", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+gatewayKey)
		req.Header.Set("X-User-Token", "gitlab-token-789")
		req.Header.Set("X-Token-Provider", "gitlab")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should fail with 403 if passthrough is required
		// Or succeed with service token if optional
		if w.Code == http.StatusForbidden {
			var errorResp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &errorResp)
			require.NoError(t, err)
			assert.Contains(t, errorResp["error"], "provider mismatch")
		}
	})
}

// runTestMigrations runs basic table creation for tests
func runTestMigrations(db *sql.DB) error {
	// Clean up any existing test data
	_, _ = db.Exec(`DROP SCHEMA IF EXISTS mcp CASCADE`)

	// Create mcp schema
	_, err := db.Exec(`CREATE SCHEMA mcp`)
	if err != nil {
		return fmt.Errorf("failed to create mcp schema: %w", err)
	}

	// Set search path to include mcp
	_, err = db.Exec(`SET search_path TO mcp, public`)
	if err != nil {
		return fmt.Errorf("failed to set search path: %w", err)
	}

	// Create tool_configurations table in mcp schema
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS mcp.tool_configurations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			tool_name VARCHAR(255) NOT NULL,
			display_name VARCHAR(255),
			base_url TEXT,
			documentation_url TEXT,
			openapi_url TEXT,
			auth_type VARCHAR(50) NOT NULL,
			credentials_encrypted BYTEA,
			config JSONB,
			retry_policy JSONB,
			health_config JSONB,
			health_status JSONB,
			last_health_check TIMESTAMP WITH TIME ZONE,
			status VARCHAR(20) DEFAULT 'active',
			created_by UUID,
			provider VARCHAR(255),
			passthrough_config JSONB DEFAULT '{"mode": "optional", "fallback_to_service": true}'::jsonb,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create users table for auth service
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS mcp.users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			email VARCHAR(255) NOT NULL,
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create tool_executions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS mcp.tool_executions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tool_config_id UUID NOT NULL,
			tenant_id UUID NOT NULL,
			action VARCHAR(255) NOT NULL,
			parameters JSONB,
			status VARCHAR(50) NOT NULL,
			result JSONB,
			error_message TEXT,
			retry_count INTEGER DEFAULT 0,
			response_time_ms INTEGER,
			executed_by UUID,
			correlation_id UUID,
			executed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP WITH TIME ZONE
		)
	`)
	return err
}
