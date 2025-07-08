package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/config"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/S-Corkum/devops-mcp/pkg/services"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestGatewayKeyPassthrough tests the gateway API key token passthrough functionality
func TestGatewayKeyPassthrough(t *testing.T) {
	// Skip in CI if no database URL is provided
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping gateway integration test")
	}

	// Connect to database
	db, err := sqlx.Connect("postgres", databaseURL)
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()

	// Initialize components
	ctx := context.Background()
	logger := observability.NewLogger("test", nil)
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		encryptionKey = "test-encryption-key-32-bytes-long"
	}

	// Create repositories
	apiKeyRepo := repository.NewAPIKeyRepository(db, logger)
	tenantRepo := repository.NewTenantRepository(db, logger)
	tenantConfigRepo := repository.NewTenantConfigRepository(db, logger)

	// Create auth service
	authConfig := &config.AuthConfig{
		SecretKey:            "test-secret-key",
		TokenExpiry:          time.Hour,
		RefreshTokenExpiry:   24 * time.Hour,
		MaxSessionsPerAPIKey: 10,
		MaxConcurrentAPIKeys: 100,
		RateLimitRequests:    60,
		RateLimitWindow:      time.Minute,
		EncryptionKey:        encryptionKey,
	}

	authService := auth.NewService(authConfig, apiKeyRepo, tenantRepo, logger, nil)

	// Create tenant config service
	tenantConfigService := services.NewTenantConfigService(
		tenantConfigRepo,
		logger,
		nil, // no cache for test
		encryptionKey,
	)

	// Create tenant-aware auth service
	tenantAwareAuth := services.NewTenantAwareAuthService(authService, tenantConfigService, logger)

	// Test scenario 1: Create gateway key with allowed services
	t.Run("CreateGatewayKeyWithAllowedServices", func(t *testing.T) {
		// First create a test tenant
		tenant := &models.Tenant{
			Name:      "Gateway Test Tenant",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := tenantRepo.Create(ctx, tenant)
		require.NoError(t, err)

		// Create tenant configuration with service tokens
		serviceTokens := map[string]string{
			"github":    "ghp_test_token_123",
			"gitlab":    "glpat_test_token_456",
			"bitbucket": "bb_test_token_789",
		}

		tenantConfig := &models.TenantConfig{
			TenantID: tenant.ID,
			RateLimitConfig: models.RateLimitConfig{
				DefaultRequestsPerMinute: 60,
				DefaultRequestsPerHour:   1000,
				DefaultRequestsPerDay:    10000,
			},
			ServiceTokens:  serviceTokens,
			AllowedOrigins: []string{"https://example.com"},
			Features: map[string]interface{}{
				"github_integration":    true,
				"gitlab_integration":    true,
				"bitbucket_integration": false,
			},
		}

		err = tenantConfigService.Create(ctx, tenantConfig)
		require.NoError(t, err)

		// Create gateway API key
		gatewayKeyReq := auth.CreateAPIKeyRequest{
			Name:            "Test Gateway Key",
			TenantID:        tenant.ID,
			KeyType:         auth.KeyTypeGateway,
			AllowedServices: []string{"github", "gitlab"},
		}

		gatewayKey, err := authService.CreateAPIKeyWithType(ctx, gatewayKeyReq)
		require.NoError(t, err)
		require.NotNil(t, gatewayKey)
		require.Equal(t, auth.KeyTypeGateway, gatewayKey.KeyType)
		require.Equal(t, []string{"github", "gitlab"}, gatewayKey.AllowedServices)

		// Test token passthrough for allowed service (GitHub)
		tokenCtx := context.WithValue(ctx, auth.APIKeyContextKey, gatewayKey.Key)
		githubToken, err := tenantAwareAuth.GetServiceToken(tokenCtx, "github")
		require.NoError(t, err)
		require.Equal(t, "ghp_test_token_123", githubToken)

		// Test token passthrough for allowed service (GitLab)
		gitlabToken, err := tenantAwareAuth.GetServiceToken(tokenCtx, "gitlab")
		require.NoError(t, err)
		require.Equal(t, "glpat_test_token_456", gitlabToken)

		// Test token passthrough for non-allowed service (Bitbucket)
		_, err = tenantAwareAuth.GetServiceToken(tokenCtx, "bitbucket")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not allowed")
	})

	// Test scenario 2: Gateway key with parent key relationship
	t.Run("GatewayKeyWithParentKey", func(t *testing.T) {
		// Create a test tenant
		tenant := &models.Tenant{
			Name:      "Parent Key Test Tenant",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := tenantRepo.Create(ctx, tenant)
		require.NoError(t, err)

		// Create admin (parent) key
		adminKeyReq := auth.CreateAPIKeyRequest{
			Name:     "Admin Parent Key",
			TenantID: tenant.ID,
			KeyType:  auth.KeyTypeAdmin,
		}
		adminKey, err := authService.CreateAPIKeyWithType(ctx, adminKeyReq)
		require.NoError(t, err)

		// Get the admin key ID from database
		var adminKeyID string
		err = db.Get(&adminKeyID, "SELECT id FROM mcp.api_keys WHERE key_prefix = $1", adminKey.KeyPrefix)
		require.NoError(t, err)

		// Create gateway key with parent relationship
		gatewayKeyReq := auth.CreateAPIKeyRequest{
			Name:            "Child Gateway Key",
			TenantID:        tenant.ID,
			KeyType:         auth.KeyTypeGateway,
			ParentKeyID:     &adminKeyID,
			AllowedServices: []string{"github"},
		}
		gatewayKey, err := authService.CreateAPIKeyWithType(ctx, gatewayKeyReq)
		require.NoError(t, err)
		require.NotNil(t, gatewayKey)
		require.NotNil(t, gatewayKey.ParentKeyID)
		require.Equal(t, adminKeyID, *gatewayKey.ParentKeyID)

		// Validate parent key exists
		parentKey, err := authService.ValidateAPIKey(ctx, adminKey.Key)
		require.NoError(t, err)
		require.Equal(t, auth.KeyTypeAdmin, parentKey.KeyType)

		// Validate child key exists and has correct parent
		childKey, err := authService.ValidateAPIKey(ctx, gatewayKey.Key)
		require.NoError(t, err)
		require.Equal(t, auth.KeyTypeGateway, childKey.KeyType)
		require.Equal(t, adminKeyID, *childKey.ParentKeyID)
	})

	// Test scenario 3: Mock external service calls
	t.Run("MockExternalServiceCalls", func(t *testing.T) {
		// Create mock GitHub API server
		githubMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify authorization header
			authHeader := r.Header.Get("Authorization")
			require.Equal(t, "Bearer ghp_test_token_123", authHeader)

			// Return mock response
			response := map[string]interface{}{
				"id":         123456,
				"name":       "test-repo",
				"full_name":  "test-org/test-repo",
				"private":    false,
				"created_at": time.Now().Format(time.RFC3339),
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer githubMock.Close()

		// Create mock GitLab API server
		gitlabMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify authorization header
			authHeader := r.Header.Get("Authorization")
			require.Equal(t, "Bearer glpat_test_token_456", authHeader)

			// Return mock response
			response := map[string]interface{}{
				"id":                  789012,
				"name":                "test-project",
				"path_with_namespace": "test-group/test-project",
				"visibility":          "public",
				"created_at":          time.Now().Format(time.RFC3339),
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer gitlabMock.Close()

		// Test making requests with passthrough tokens
		client := &http.Client{Timeout: 10 * time.Second}

		// Test GitHub request
		req, err := http.NewRequest("GET", githubMock.URL+"/repos/test-org/test-repo", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer ghp_test_token_123")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var githubResponse map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&githubResponse)
		require.NoError(t, err)
		require.Equal(t, "test-repo", githubResponse["name"])

		// Test GitLab request
		req, err = http.NewRequest("GET", gitlabMock.URL+"/projects/test-group/test-project", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer glpat_test_token_456")

		resp, err = client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var gitlabResponse map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&gitlabResponse)
		require.NoError(t, err)
		require.Equal(t, "test-project", gitlabResponse["name"])
	})

	// Test scenario 4: Feature flag controls
	t.Run("FeatureFlagControls", func(t *testing.T) {
		// Create tenant with feature flags
		tenant := &models.Tenant{
			Name:      "Feature Flag Test Tenant",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := tenantRepo.Create(ctx, tenant)
		require.NoError(t, err)

		// Create tenant config with feature flags
		tenantConfig := &models.TenantConfig{
			TenantID: tenant.ID,
			RateLimitConfig: models.RateLimitConfig{
				DefaultRequestsPerMinute: 60,
			},
			ServiceTokens: map[string]string{
				"github": "ghp_feature_test_token",
			},
			Features: map[string]interface{}{
				"github_integration":   true,
				"token_passthrough":    true,
				"advanced_rate_limits": false,
			},
		}

		err = tenantConfigService.Create(ctx, tenantConfig)
		require.NoError(t, err)

		// Verify feature flags
		config, err := tenantConfigService.GetByTenantID(ctx, tenant.ID)
		require.NoError(t, err)
		require.True(t, config.IsFeatureEnabled("github_integration"))
		require.True(t, config.IsFeatureEnabled("token_passthrough"))
		require.False(t, config.IsFeatureEnabled("advanced_rate_limits"))
		require.False(t, config.IsFeatureEnabled("non_existent_feature"))
	})

	// Test scenario 5: Rate limiting for gateway keys
	t.Run("GatewayKeyRateLimiting", func(t *testing.T) {
		// Create tenant with custom rate limits
		tenant := &models.Tenant{
			Name:      "Rate Limit Test Tenant",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := tenantRepo.Create(ctx, tenant)
		require.NoError(t, err)

		// Create tenant config with custom rate limits for gateway keys
		tenantConfig := &models.TenantConfig{
			TenantID: tenant.ID,
			RateLimitConfig: models.RateLimitConfig{
				DefaultRequestsPerMinute: 60,
				DefaultRequestsPerHour:   1000,
				DefaultRequestsPerDay:    10000,
				KeyTypeOverrides: map[string]models.KeyTypeRateLimit{
					"gateway": {
						RequestsPerMinute: 120,
						RequestsPerHour:   2000,
						RequestsPerDay:    20000,
					},
					"agent": {
						RequestsPerMinute: 30,
						RequestsPerHour:   500,
						RequestsPerDay:    5000,
					},
				},
			},
		}

		err = tenantConfigService.Create(ctx, tenantConfig)
		require.NoError(t, err)

		// Verify rate limits for different key types
		config, err := tenantConfigService.GetByTenantID(ctx, tenant.ID)
		require.NoError(t, err)

		gatewayLimits := config.GetRateLimitForKeyType("gateway")
		require.Equal(t, 120, gatewayLimits.RequestsPerMinute)
		require.Equal(t, 2000, gatewayLimits.RequestsPerHour)
		require.Equal(t, 20000, gatewayLimits.RequestsPerDay)

		agentLimits := config.GetRateLimitForKeyType("agent")
		require.Equal(t, 30, agentLimits.RequestsPerMinute)
		require.Equal(t, 500, agentLimits.RequestsPerHour)
		require.Equal(t, 5000, agentLimits.RequestsPerDay)

		// Default limits for unspecified key type
		userLimits := config.GetRateLimitForKeyType("user")
		require.Equal(t, 60, userLimits.RequestsPerMinute)
		require.Equal(t, 1000, userLimits.RequestsPerHour)
		require.Equal(t, 10000, userLimits.RequestsPerDay)
	})
}

// TestGatewayKeyValidation tests gateway key validation and restrictions
func TestGatewayKeyValidation(t *testing.T) {
	// Skip in CI if no database URL is provided
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping gateway validation test")
	}

	// Connect to database
	db, err := sqlx.Connect("postgres", databaseURL)
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()

	// Initialize components
	ctx := context.Background()
	logger := observability.NewLogger("test", nil)

	// Create repositories
	apiKeyRepo := repository.NewAPIKeyRepository(db, logger)
	tenantRepo := repository.NewTenantRepository(db, logger)

	// Create auth service
	authConfig := &config.AuthConfig{
		SecretKey:            "test-secret-key",
		TokenExpiry:          time.Hour,
		RefreshTokenExpiry:   24 * time.Hour,
		MaxSessionsPerAPIKey: 10,
		MaxConcurrentAPIKeys: 100,
		RateLimitRequests:    60,
		RateLimitWindow:      time.Minute,
	}

	authService := auth.NewService(authConfig, apiKeyRepo, tenantRepo, logger, nil)

	// Test scenario: Gateway key cannot create other keys
	t.Run("GatewayKeyCannotCreateKeys", func(t *testing.T) {
		// Create test tenant
		tenant := &models.Tenant{
			Name:      "Gateway Restriction Test",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := tenantRepo.Create(ctx, tenant)
		require.NoError(t, err)

		// Create gateway key
		gatewayKeyReq := auth.CreateAPIKeyRequest{
			Name:            "Restricted Gateway Key",
			TenantID:        tenant.ID,
			KeyType:         auth.KeyTypeGateway,
			AllowedServices: []string{"github"},
		}
		gatewayKey, err := authService.CreateAPIKeyWithType(ctx, gatewayKeyReq)
		require.NoError(t, err)

		// Attempt to use gateway key to create another key (should fail)
		gatewayCtx := context.WithValue(ctx, auth.APIKeyContextKey, gatewayKey.Key)

		// First validate the gateway key
		validatedKey, err := authService.ValidateAPIKey(gatewayCtx, gatewayKey.Key)
		require.NoError(t, err)
		require.Equal(t, auth.KeyTypeGateway, validatedKey.KeyType)

		// Check if gateway key has permission to create keys (it shouldn't)
		canCreateKeys := false
		for _, scope := range validatedKey.Scopes {
			if scope == "api_keys:create" || scope == "admin:all" {
				canCreateKeys = true
				break
			}
		}
		require.False(t, canCreateKeys, "Gateway key should not have permission to create API keys")
	})
}

// BenchmarkGatewayKeyPassthrough benchmarks the performance of token passthrough
func BenchmarkGatewayKeyPassthrough(b *testing.B) {
	// Skip if no database URL
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		b.Skip("DATABASE_URL not set, skipping benchmark")
	}

	// Setup
	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		b.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	logger := observability.NewLogger("benchmark", nil)
	encryptionKey := "benchmark-encryption-key-32bytes"

	// Create services
	tenantConfigRepo := repository.NewTenantConfigRepository(db, logger)
	tenantConfigService := services.NewTenantConfigService(
		tenantConfigRepo,
		logger,
		nil,
		encryptionKey,
	)

	// Create test data
	tenantID := fmt.Sprintf("benchmark-tenant-%d", time.Now().Unix())
	tenantConfig := &models.TenantConfig{
		TenantID: tenantID,
		ServiceTokens: map[string]string{
			"github": "ghp_benchmark_token",
			"gitlab": "glpat_benchmark_token",
		},
	}
	err = tenantConfigService.Create(ctx, tenantConfig)
	if err != nil {
		b.Fatalf("Failed to create tenant config: %v", err)
	}

	// Benchmark token retrieval
	b.ResetTimer()
	b.Run("GetServiceToken", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			config, _ := tenantConfigService.GetByTenantID(ctx, tenantID)
			if config != nil {
				config.GetServiceToken("github")
			}
		}
	})
}
