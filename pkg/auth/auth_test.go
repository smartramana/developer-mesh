package auth

import (
	"context"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	logger := observability.NewLogger("test")

	t.Run("with default config", func(t *testing.T) {
		service := NewService(nil, nil, nil, logger)
		assert.NotNil(t, service)
		assert.NotNil(t, service.config)
		assert.Equal(t, 24*time.Hour, service.config.JWTExpiration)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &ServiceConfig{
			JWTSecret:     "test-secret",
			JWTExpiration: 1 * time.Hour,
			EnableAPIKeys: true,
		}

		service := NewService(config, nil, nil, logger)
		assert.NotNil(t, service)
		assert.Equal(t, "test-secret", service.config.JWTSecret)
		assert.Equal(t, 1*time.Hour, service.config.JWTExpiration)
	})
}

func TestAPIKeyValidation(t *testing.T) {
	logger := observability.NewLogger("test")
	config := &ServiceConfig{
		EnableAPIKeys: true,
		CacheEnabled:  false, // Disable cache for testing
	}
	service := NewService(config, nil, nil, logger)
	ctx := context.Background()

	// Create a test API key
	apiKey, err := service.CreateAPIKey(ctx, "test-tenant", "test-user", "Test Key", []string{"read"}, nil)
	require.NoError(t, err)
	require.NotNil(t, apiKey)

	t.Run("valid API key", func(t *testing.T) {
		user, err := service.ValidateAPIKey(ctx, apiKey.Key)
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "test-user", user.ID)
		assert.Equal(t, "test-tenant", user.TenantID)
		assert.Equal(t, []string{"read"}, user.Scopes)
		assert.Equal(t, TypeAPIKey, user.AuthType)
	})

	t.Run("invalid API key", func(t *testing.T) {
		user, err := service.ValidateAPIKey(ctx, "invalid-key")
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Equal(t, ErrInvalidAPIKey, err)
	})

	t.Run("empty API key", func(t *testing.T) {
		user, err := service.ValidateAPIKey(ctx, "")
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Equal(t, ErrNoAPIKey, err)
	})

	t.Run("expired API key", func(t *testing.T) {
		expiredTime := time.Now().Add(-1 * time.Hour)
		expiredKey, err := service.CreateAPIKey(ctx, "test-tenant", "test-user", "Expired Key", []string{"read"}, &expiredTime)
		require.NoError(t, err)

		user, err := service.ValidateAPIKey(ctx, expiredKey.Key)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Equal(t, ErrInvalidAPIKey, err)
	})

	t.Run("revoked API key", func(t *testing.T) {
		revokeKey, err := service.CreateAPIKey(ctx, "test-tenant", "test-user", "Revoke Key", []string{"read"}, nil)
		require.NoError(t, err)

		// Revoke the key
		err = service.RevokeAPIKey(ctx, revokeKey.Key)
		require.NoError(t, err)

		// Try to use revoked key
		user, err := service.ValidateAPIKey(ctx, revokeKey.Key)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Equal(t, ErrInvalidAPIKey, err)
	})
}

func TestJWTValidation(t *testing.T) {
	logger := observability.NewLogger("test")
	config := &ServiceConfig{
		JWTSecret:     "test-secret-key",
		JWTExpiration: 1 * time.Hour,
		EnableJWT:     true,
	}
	service := NewService(config, nil, nil, logger)
	ctx := context.Background()

	testUser := &User{
		ID:       "test-user",
		TenantID: "test-tenant",
		Email:    "test@example.com",
		Scopes:   []string{"read", "write"},
	}

	t.Run("valid JWT", func(t *testing.T) {
		// Generate a token
		token, err := service.GenerateJWT(ctx, testUser)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// Validate the token
		user, err := service.ValidateJWT(ctx, token)
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, testUser.ID, user.ID)
		assert.Equal(t, testUser.TenantID, user.TenantID)
		assert.Equal(t, testUser.Email, user.Email)
		assert.Equal(t, testUser.Scopes, user.Scopes)
		assert.Equal(t, TypeJWT, user.AuthType)
	})

	t.Run("invalid JWT", func(t *testing.T) {
		user, err := service.ValidateJWT(ctx, "invalid.jwt.token")
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Equal(t, ErrInvalidToken, err)
	})

	t.Run("empty JWT", func(t *testing.T) {
		user, err := service.ValidateJWT(ctx, "")
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Equal(t, ErrInvalidToken, err)
	})
}

func TestScopeAuthorization(t *testing.T) {
	logger := observability.NewLogger("test")
	service := NewService(nil, nil, nil, logger)

	testCases := []struct {
		name           string
		userScopes     []string
		requiredScopes []string
		expectError    bool
	}{
		{
			name:           "user has all required scopes",
			userScopes:     []string{"read", "write", "admin"},
			requiredScopes: []string{"read", "write"},
			expectError:    false,
		},
		{
			name:           "user missing required scope",
			userScopes:     []string{"read"},
			requiredScopes: []string{"read", "write"},
			expectError:    true,
		},
		{
			name:           "no scopes required",
			userScopes:     []string{"read"},
			requiredScopes: []string{},
			expectError:    false,
		},
		{
			name:           "user has no scopes",
			userScopes:     []string{},
			requiredScopes: []string{"read"},
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			user := &User{
				ID:       "test-user",
				TenantID: "test-tenant",
				Scopes:   tc.userScopes,
			}

			err := service.AuthorizeScopes(user, tc.requiredScopes)
			if tc.expectError {
				assert.Error(t, err)
				assert.Equal(t, ErrInsufficientScope, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPasswordHashing(t *testing.T) {
	password := "my-secure-password"

	t.Run("hash password", func(t *testing.T) {
		hash, err := HashPassword(password)
		assert.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.NotEqual(t, password, hash) // Hash should be different from password
	})

	t.Run("check valid password", func(t *testing.T) {
		hash, err := HashPassword(password)
		require.NoError(t, err)

		valid := CheckPassword(password, hash)
		assert.True(t, valid)
	})

	t.Run("check invalid password", func(t *testing.T) {
		hash, err := HashPassword(password)
		require.NoError(t, err)

		valid := CheckPassword("wrong-password", hash)
		assert.False(t, valid)
	})
}

func TestInitializeDefaultAPIKeys(t *testing.T) {
	logger := observability.NewLogger("test")
	service := NewService(nil, nil, nil, logger)

	keys := map[string]string{
		"admin-key": "admin",
		"read-key":  "read",
		"write-key": "write",
	}

	service.InitializeDefaultAPIKeys(keys)

	// Verify keys were initialized
	ctx := context.Background()

	t.Run("admin key", func(t *testing.T) {
		user, err := service.ValidateAPIKey(ctx, "admin-key")
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, []string{"read", "write", "admin"}, user.Scopes)
	})

	t.Run("read key", func(t *testing.T) {
		user, err := service.ValidateAPIKey(ctx, "read-key")
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, []string{"read"}, user.Scopes)
	})

	t.Run("write key", func(t *testing.T) {
		user, err := service.ValidateAPIKey(ctx, "write-key")
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, []string{"read", "write"}, user.Scopes)
	})
}
