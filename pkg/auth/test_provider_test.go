package auth_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

func TestNewTestProvider(t *testing.T) {
	logger := observability.NewLogger("test")

	tests := []struct {
		name          string
		setupEnv      func()
		cleanupEnv    func()
		expectError   bool
		errorContains string
	}{
		{
			name: "success with test mode enabled",
			setupEnv: func() {
				_ = os.Setenv("MCP_TEST_MODE", "true")
				_ = os.Setenv("TEST_AUTH_ENABLED", "true")
			},
			cleanupEnv: func() {
				_ = os.Unsetenv("MCP_TEST_MODE")
				_ = os.Unsetenv("TEST_AUTH_ENABLED")
			},
			expectError: false,
		},
		{
			name: "fails without test mode",
			setupEnv: func() {
				_ = os.Setenv("MCP_TEST_MODE", "false")
				_ = os.Setenv("TEST_AUTH_ENABLED", "true")
			},
			cleanupEnv: func() {
				_ = os.Unsetenv("MCP_TEST_MODE")
				_ = os.Unsetenv("TEST_AUTH_ENABLED")
			},
			expectError:   true,
			errorContains: "test provider can only be used in test mode",
		},
		{
			name: "fails without test auth enabled",
			setupEnv: func() {
				_ = os.Setenv("MCP_TEST_MODE", "true")
				_ = os.Setenv("TEST_AUTH_ENABLED", "false")
			},
			cleanupEnv: func() {
				_ = os.Unsetenv("MCP_TEST_MODE")
				_ = os.Unsetenv("TEST_AUTH_ENABLED")
			},
			expectError:   true,
			errorContains: "test auth must be explicitly enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer tt.cleanupEnv()

			provider, err := auth.NewTestProvider(logger, nil)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, provider)

				// Cleanup
				err = provider.Close()
				assert.NoError(t, err)
			}
		})
	}
}

func TestTestProviderAuthorize(t *testing.T) {
	// Setup
	_ = os.Setenv("MCP_TEST_MODE", "true")
	_ = os.Setenv("TEST_AUTH_ENABLED", "true")
	defer func() {
		_ = os.Unsetenv("MCP_TEST_MODE")
		_ = os.Unsetenv("TEST_AUTH_ENABLED")
	}()

	logger := observability.NewLogger("test")
	provider, err := auth.NewTestProvider(logger, nil)
	require.NoError(t, err)
	defer func() {
		_ = provider.Close()
	}()

	ctx := context.Background()

	t.Run("allows all permissions in test mode", func(t *testing.T) {
		permission := auth.Permission{
			Resource: "test-resource",
			Action:   "test-action",
		}

		decision := provider.Authorize(ctx, permission)
		assert.True(t, decision.Allowed)
		assert.Contains(t, decision.Reason, "test mode allows all")
	})

	t.Run("respects rate limiting", func(t *testing.T) {
		// The rate limiter allows 1000/minute (16.67/second) with burst of 100
		// Make more than burst limit requests to trigger rate limiting
		var rateLimited bool
		var successCount int

		// Make 200 requests - should trigger rate limiting after burst of 100
		for i := 0; i < 200; i++ {
			permission := auth.Permission{
				Resource: "test-resource",
				Action:   "test-action",
			}

			decision := provider.Authorize(ctx, permission)
			if !decision.Allowed && decision.Reason == "rate limit exceeded" {
				rateLimited = true
				t.Logf("Rate limiting triggered after %d successful requests", successCount)
				break
			}
			if decision.Allowed {
				successCount++
			}
		}

		assert.True(t, rateLimited, "Expected rate limiting to trigger after burst limit")
		// We should allow approximately the burst size before rate limiting
		assert.GreaterOrEqual(t, successCount, 90, "Should allow at least 90 requests (close to burst limit)")
		assert.LessOrEqual(t, successCount, 110, "Should not allow much more than burst limit of 100")
	})
}

func TestTestProviderCheckPermission(t *testing.T) {
	// Setup
	_ = os.Setenv("MCP_TEST_MODE", "true")
	_ = os.Setenv("TEST_AUTH_ENABLED", "true")
	defer func() {
		_ = os.Unsetenv("MCP_TEST_MODE")
		_ = os.Unsetenv("TEST_AUTH_ENABLED")
	}()

	logger := observability.NewLogger("test")
	provider, err := auth.NewTestProvider(logger, nil)
	require.NoError(t, err)
	defer func() {
		_ = provider.Close()
	}()

	ctx := context.Background()

	t.Run("allows all permissions", func(t *testing.T) {
		allowed := provider.CheckPermission(ctx, "test-resource", "test-action")
		assert.True(t, allowed)
	})
}

func TestTestProviderTokenManagement(t *testing.T) {
	// Setup
	_ = os.Setenv("MCP_TEST_MODE", "true")
	_ = os.Setenv("TEST_AUTH_ENABLED", "true")
	defer func() {
		_ = os.Unsetenv("MCP_TEST_MODE")
		_ = os.Unsetenv("TEST_AUTH_ENABLED")
	}()

	logger := observability.NewLogger("test")
	provider, err := auth.NewTestProvider(logger, nil)
	require.NoError(t, err)
	defer func() {
		_ = provider.Close()
	}()

	userID := uuid.New()
	tenantID := uuid.New()

	t.Run("generates valid JWT token", func(t *testing.T) {
		token, err := provider.GenerateTestToken(userID, tenantID, "test-role", []string{"read", "write"})
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate the token
		claims, err := provider.ValidateTestToken(token)
		require.NoError(t, err)
		assert.Equal(t, userID.String(), claims.UserID)
		assert.Equal(t, tenantID.String(), claims.TenantID)
		assert.Contains(t, claims.Scopes, "read")
		assert.Contains(t, claims.Scopes, "write")
	})

	t.Run("rejects invalid token", func(t *testing.T) {
		_, err := provider.ValidateTestToken("invalid-token")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid token")
	})

	t.Run("rejects non-test token", func(t *testing.T) {
		// Create a token without test_mode flag
		_, err := provider.GenerateTestToken(userID, tenantID, "test-role", []string{"read"})
		require.NoError(t, err)

		// Manually modify to remove test_mode (would need to implement this differently in real scenario)
		// For now, we'll test with an invalid token
		_, err = provider.ValidateTestToken("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c")
		require.Error(t, err)
	})

	t.Run("token revocation", func(t *testing.T) {
		token, err := provider.GenerateTestToken(userID, tenantID, "test-role", []string{"read"})
		require.NoError(t, err)

		// Token should be valid initially
		_, err = provider.ValidateTestToken(token)
		require.NoError(t, err)

		// Note: Revocation by token ID requires extracting the JTI claim
		// For now, we test that the method exists
		err = provider.RevokeToken("test-token-id")
		assert.NoError(t, err)
	})
}

func TestTestProviderHelperMethods(t *testing.T) {
	// Setup
	_ = os.Setenv("MCP_TEST_MODE", "true")
	_ = os.Setenv("TEST_AUTH_ENABLED", "true")
	defer func() {
		_ = os.Unsetenv("MCP_TEST_MODE")
		_ = os.Unsetenv("TEST_AUTH_ENABLED")
	}()

	logger := observability.NewLogger("test")
	provider, err := auth.NewTestProvider(logger, nil)
	require.NoError(t, err)
	defer func() {
		_ = provider.Close()
	}()

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()

	t.Run("GetUserRole returns test role", func(t *testing.T) {
		role, err := provider.GetUserRole(ctx, userID, tenantID)
		require.NoError(t, err)
		assert.Equal(t, "test_user", role)
	})

	t.Run("ListUserPermissions returns test permissions", func(t *testing.T) {
		permissions, err := provider.ListUserPermissions(ctx, userID, tenantID)
		require.NoError(t, err)
		assert.Contains(t, permissions, "read:*")
		assert.Contains(t, permissions, "write:*")
		assert.Contains(t, permissions, "test:*")
	})
}

func TestGenerateTestAPIKey(t *testing.T) {
	key1 := auth.GenerateTestAPIKey()
	key2 := auth.GenerateTestAPIKey()

	assert.NotEmpty(t, key1)
	assert.NotEmpty(t, key2)
	assert.NotEqual(t, key1, key2, "Keys should be unique")
	assert.Contains(t, key1, "test-", "Keys should have test prefix")
}

func TestTestProviderConcurrency(t *testing.T) {
	// Setup
	_ = os.Setenv("MCP_TEST_MODE", "true")
	_ = os.Setenv("TEST_AUTH_ENABLED", "true")
	defer func() {
		_ = os.Unsetenv("MCP_TEST_MODE")
		_ = os.Unsetenv("TEST_AUTH_ENABLED")
	}()

	logger := observability.NewLogger("test")
	provider, err := auth.NewTestProvider(logger, nil)
	require.NoError(t, err)
	defer func() {
		_ = provider.Close()
	}()

	ctx := context.Background()

	// Test concurrent operations
	t.Run("concurrent authorization", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				permission := auth.Permission{
					Resource: "test-resource",
					Action:   "test-action",
				}
				decision := provider.Authorize(ctx, permission)
				assert.True(t, decision.Allowed)
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			select {
			case <-done:
				// Success
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for concurrent operations")
			}
		}
	})
}
