package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// PassthroughProvider provides authentication using a token passed through from a gateway
type PassthroughProvider struct {
	BaseAuthProvider
	token string
}

// NewPassthroughProvider creates a new passthrough authentication provider
func NewPassthroughProvider(token string, logger observability.Logger) *PassthroughProvider {
	return &PassthroughProvider{
		BaseAuthProvider: BaseAuthProvider{
			authType: AuthTypeToken,
			logger:   logger,
		},
		token: token,
	}
}

// GetToken returns the passthrough token
func (p *PassthroughProvider) GetToken(ctx context.Context) (string, error) {
	if p.token == "" {
		return "", fmt.Errorf("no passthrough token available")
	}
	return p.token, nil
}

// SetAuthHeaders sets the authentication headers on an HTTP request
func (p *PassthroughProvider) SetAuthHeaders(req *http.Request) error {
	if p.token == "" {
		return fmt.Errorf("no passthrough token available")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.token))
	return nil
}

// AuthenticateRequest authenticates an HTTP request with the passthrough token
func (p *PassthroughProvider) AuthenticateRequest(req *http.Request) error {
	return p.SetAuthHeaders(req)
}

// RefreshToken is a no-op for passthrough tokens
func (p *PassthroughProvider) RefreshToken(ctx context.Context) error {
	// Passthrough tokens cannot be refreshed
	return nil
}

// IsValid checks if the passthrough token is available
func (p *PassthroughProvider) IsValid() bool {
	return p.token != ""
}

// GetAuthProviderFromContext creates an auth provider from context if passthrough token exists
func GetAuthProviderFromContext(ctx context.Context, fallback AuthProvider, logger observability.Logger) AuthProvider {
	// Check if there's a passthrough token in the context
	passthroughToken, ok := auth.GetPassthroughToken(ctx)
	if !ok || passthroughToken.Provider != "github" {
		// No GitHub passthrough token, use fallback
		return fallback
	}

	// Create a passthrough provider with the token
	return NewPassthroughProvider(passthroughToken.Token, logger)
}
