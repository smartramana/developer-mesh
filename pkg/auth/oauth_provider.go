package auth

import (
	"context"
	"net/url"
	"time"
)

// OAuthToken represents an OAuth token
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// OAuthUserInfo represents user information from OAuth provider
type OAuthUserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// OAuthProvider defines the interface for OAuth providers
type OAuthProvider interface {
	// GetAuthorizationURL returns the authorization URL
	GetAuthorizationURL(state, redirectURI string) string

	// GetAuthorizationURLWithPKCE returns the authorization URL with PKCE
	GetAuthorizationURLWithPKCE(state, redirectURI, codeChallenge string) string

	// ExchangeCode exchanges an authorization code for tokens
	ExchangeCode(ctx context.Context, code, redirectURI string) (*OAuthToken, error)

	// ExchangeCodeWithPKCE exchanges an authorization code with PKCE
	ExchangeCodeWithPKCE(ctx context.Context, code, redirectURI, codeVerifier string) (*OAuthToken, error)

	// RefreshToken refreshes an access token
	RefreshToken(ctx context.Context, refreshToken string) (*OAuthToken, error)

	// ValidateToken validates an access token and returns user info
	ValidateToken(ctx context.Context, accessToken string) (*OAuthUserInfo, error)

	// ValidateState validates the state parameter
	ValidateState(providedState, expectedState string) bool
}

// BaseOAuthProvider provides common OAuth functionality
type BaseOAuthProvider struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
}

// GetAuthorizationURL returns the authorization URL
func (p *BaseOAuthProvider) GetAuthorizationURL(state, redirectURI string) string {
	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("state", state)
	params.Set("scope", "openid email profile")

	return p.AuthURL + "?" + params.Encode()
}

// GetAuthorizationURLWithPKCE returns the authorization URL with PKCE
func (p *BaseOAuthProvider) GetAuthorizationURLWithPKCE(state, redirectURI, codeChallenge string) string {
	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("state", state)
	params.Set("scope", "openid email profile")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")

	return p.AuthURL + "?" + params.Encode()
}

// ValidateState validates the state parameter
func (p *BaseOAuthProvider) ValidateState(providedState, expectedState string) bool {
	return providedState == expectedState
}
