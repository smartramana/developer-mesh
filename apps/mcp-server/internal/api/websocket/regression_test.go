package websocket

import (
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWebSocketAuthServicePointerRegression verifies the fix for the nil pointer dereference
// bug that occurred when the WebSocket server used a value type for auth.Service instead
// of a pointer type.
//
// Bug: The WebSocket server was storing auth.Service as a value type, which caused
// struct copying when passed from server.go. This led to nil pointer dereferences
// when the auth service tried to access its config field.
//
// Fix: Changed the WebSocket server to use *auth.Service (pointer type) to avoid
// unnecessary copying and ensure the same auth service instance is used throughout
// the application.
func TestWebSocketAuthServicePointerRegression(t *testing.T) {
	// Create an auth service with minimal initialization
	authService := &auth.Service{}
	
	// Create WebSocket server - this should not panic even with uninitialized auth service
	logger := NewTestLogger()
	config := Config{
		MaxConnections:  10,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	
	// This used to panic when auth was a value type
	server := NewServer(authService, nil, logger, config)
	
	// Verify the server was created successfully
	require.NotNil(t, server)
	require.NotNil(t, server.auth)
	
	// Verify the auth service is the same instance (pointer equality)
	assert.Equal(t, authService, server.auth, "auth service should be the same instance")
	
	// The auth service ValidateAPIKey should handle nil config gracefully
	// This test ensures the defensive programming we added works
	ctx := testContext()
	user, err := server.auth.ValidateAPIKey(ctx, "test-key")
	
	// Should return error (invalid key) but not panic
	assert.Error(t, err)
	assert.Nil(t, user)
}

// TestAuthServiceDefensiveProgramming verifies that the auth service
// handles nil fields gracefully to prevent panics.
func TestAuthServiceDefensiveProgramming(t *testing.T) {
	// Create auth service with nil config and logger
	authService := &auth.Service{
		// config: nil,
		// logger: nil,
	}
	
	ctx := testContext()
	
	// ValidateAPIKey should not panic even with nil config
	user, err := authService.ValidateAPIKey(ctx, "test-key")
	assert.Error(t, err)
	assert.Nil(t, user)
	
	// ValidateJWT should also handle nil config
	claims, err := authService.ValidateJWT(ctx, "test-token")
	assert.Error(t, err)
	assert.Nil(t, claims)
}