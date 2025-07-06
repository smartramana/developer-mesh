package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPassthroughTokenContext(t *testing.T) {
	tests := []struct {
		name    string
		token   PassthroughToken
		wantGet bool
	}{
		{
			name: "store and retrieve passthrough token",
			token: PassthroughToken{
				Provider: "github",
				Token:    "ghp_test123",
				Scopes:   []string{"repo", "read:user"},
			},
			wantGet: true,
		},
		{
			name: "retrieve from empty context",
			token: PassthroughToken{
				Provider: "github",
				Token:    "ghp_test123",
			},
			wantGet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			if tt.wantGet {
				// Store token in context
				ctx = WithPassthroughToken(ctx, tt.token)
			}

			// Retrieve token from context
			retrieved, ok := GetPassthroughToken(ctx)

			if tt.wantGet {
				assert.True(t, ok)
				assert.NotNil(t, retrieved)
				assert.Equal(t, tt.token.Provider, retrieved.Provider)
				assert.Equal(t, tt.token.Token, retrieved.Token)
				assert.Equal(t, tt.token.Scopes, retrieved.Scopes)
			} else {
				assert.False(t, ok)
				assert.Nil(t, retrieved)
			}
		})
	}
}

func TestTokenProviderContext(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantGet  bool
	}{
		{
			name:     "store and retrieve provider",
			provider: "github",
			wantGet:  true,
		},
		{
			name:     "retrieve from empty context",
			provider: "",
			wantGet:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			if tt.wantGet {
				ctx = WithTokenProvider(ctx, tt.provider)
			}

			retrieved, ok := GetTokenProvider(ctx)

			if tt.wantGet {
				assert.True(t, ok)
				assert.Equal(t, tt.provider, retrieved)
			} else {
				assert.False(t, ok)
				assert.Empty(t, retrieved)
			}
		})
	}
}

func TestGatewayIDContext(t *testing.T) {
	tests := []struct {
		name      string
		gatewayID string
		wantGet   bool
	}{
		{
			name:      "store and retrieve gateway ID",
			gatewayID: "gw_123456",
			wantGet:   true,
		},
		{
			name:      "retrieve from empty context",
			gatewayID: "",
			wantGet:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			if tt.wantGet {
				ctx = WithGatewayID(ctx, tt.gatewayID)
			}

			retrieved, ok := GetGatewayID(ctx)

			if tt.wantGet {
				assert.True(t, ok)
				assert.Equal(t, tt.gatewayID, retrieved)
			} else {
				assert.False(t, ok)
				assert.Empty(t, retrieved)
			}
		})
	}
}

func TestValidateProviderAllowed(t *testing.T) {
	tests := []struct {
		name            string
		provider        string
		allowedServices []string
		want            bool
	}{
		{
			name:            "provider in allowed list",
			provider:        "github",
			allowedServices: []string{"github", "gitlab"},
			want:            true,
		},
		{
			name:            "provider not in allowed list",
			provider:        "bitbucket",
			allowedServices: []string{"github", "gitlab"},
			want:            false,
		},
		{
			name:            "empty allowed list",
			provider:        "github",
			allowedServices: []string{},
			want:            false,
		},
		{
			name:            "nil allowed list",
			provider:        "github",
			allowedServices: nil,
			want:            false,
		},
		{
			name:            "exact match required",
			provider:        "github",
			allowedServices: []string{"GitHub", "gitlab"},
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateProviderAllowed(tt.provider, tt.allowedServices)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractAllowedServices(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		want     []string
	}{
		{
			name: "extract string slice",
			metadata: map[string]interface{}{
				"allowed_services": []string{"github", "gitlab"},
			},
			want: []string{"github", "gitlab"},
		},
		{
			name: "extract interface slice",
			metadata: map[string]interface{}{
				"allowed_services": []interface{}{"github", "gitlab", "bitbucket"},
			},
			want: []string{"github", "gitlab", "bitbucket"},
		},
		{
			name: "no allowed_services key",
			metadata: map[string]interface{}{
				"other_key": "value",
			},
			want: nil,
		},
		{
			name:     "nil metadata",
			metadata: nil,
			want:     nil,
		},
		{
			name: "invalid type for allowed_services",
			metadata: map[string]interface{}{
				"allowed_services": "github,gitlab",
			},
			want: nil,
		},
		{
			name: "mixed types in interface slice",
			metadata: map[string]interface{}{
				"allowed_services": []interface{}{"github", 123, "gitlab", nil},
			},
			want: []string{"github", "gitlab"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAllowedServices(tt.metadata)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContextChaining(t *testing.T) {
	// Test that multiple context values can be stored and retrieved
	ctx := context.Background()

	token := PassthroughToken{
		Provider: "github",
		Token:    "ghp_test",
		Scopes:   []string{"repo"},
	}

	// Add all context values
	ctx = WithPassthroughToken(ctx, token)
	ctx = WithTokenProvider(ctx, "github")
	ctx = WithGatewayID(ctx, "gw_12345")

	// Verify all values are retrievable
	retrievedToken, ok1 := GetPassthroughToken(ctx)
	assert.True(t, ok1)
	assert.Equal(t, token.Provider, retrievedToken.Provider)

	retrievedProvider, ok2 := GetTokenProvider(ctx)
	assert.True(t, ok2)
	assert.Equal(t, "github", retrievedProvider)

	retrievedGateway, ok3 := GetGatewayID(ctx)
	assert.True(t, ok3)
	assert.Equal(t, "gw_12345", retrievedGateway)
}
