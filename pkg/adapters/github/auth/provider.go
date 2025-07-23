package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/errors"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/golang-jwt/jwt/v4"
)

// AuthType defines the type of authentication
type AuthType string

const (
	// AuthTypeNone represents no authentication
	AuthTypeNone AuthType = "none"

	// AuthTypeToken represents personal access token authentication
	AuthTypeToken AuthType = "token"

	// AuthTypeApp represents GitHub App authentication
	AuthTypeApp AuthType = "app"

	// AuthTypeOAuth represents OAuth token authentication
	AuthTypeOAuth AuthType = "oauth"
)

// AuthProvider defines the interface for GitHub authentication providers
type AuthProvider interface {
	// Type returns the authentication type
	Type() AuthType

	// GetToken returns a valid authentication token
	GetToken(ctx context.Context) (string, error)

	// SetAuthHeaders sets the authentication headers on an HTTP request
	SetAuthHeaders(req *http.Request) error

	// AuthenticateRequest authenticates an HTTP request with the appropriate credentials
	AuthenticateRequest(req *http.Request) error

	// RefreshToken refreshes the authentication token if needed
	RefreshToken(ctx context.Context) error

	// IsValid checks if the authentication is valid
	IsValid() bool
}

// BaseAuthProvider provides base functionality for authentication providers
type BaseAuthProvider struct {
	authType AuthType
	logger   observability.Logger // Changed from pointer to interface type
}

// Type returns the authentication type
func (p *BaseAuthProvider) Type() AuthType {
	return p.authType
}

// NewAuthProvider creates a new authentication provider based on configuration
func NewAuthProvider(config *Config, logger observability.Logger) (AuthProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("auth config cannot be nil")
	}

	// Select provider based on configuration
	switch {
	case config.Token != "":
		return NewTokenProvider(config.Token, logger), nil
	case config.AppID != "" && config.AppPrivateKey != "":
		return NewAppProvider(config.AppID, config.AppPrivateKey, config.AppInstallationID, logger)
	case config.OAuthToken != "":
		return NewOAuthProvider(config.OAuthToken, config.OAuthClientID, config.OAuthClientSecret, logger), nil
	default:
		// Default to no authentication
		return NewNoAuthProvider(logger), nil
	}
}

// Config holds authentication configuration
type Config struct {
	// Personal Access Token
	Token string

	// GitHub App
	AppID             string
	AppPrivateKey     string
	AppInstallationID string

	// OAuth
	OAuthToken        string
	OAuthClientID     string
	OAuthClientSecret string
}

// NoAuthProvider provides no authentication
type NoAuthProvider struct {
	BaseAuthProvider
}

// NewNoAuthProvider creates a new provider with no authentication
func NewNoAuthProvider(logger observability.Logger) *NoAuthProvider {
	return &NoAuthProvider{
		BaseAuthProvider: BaseAuthProvider{
			authType: AuthTypeNone,
			logger:   logger,
		},
	}
}

// GetToken returns an empty token
func (p *NoAuthProvider) GetToken(ctx context.Context) (string, error) {
	return "", nil
}

// SetAuthHeaders does nothing for no authentication
func (p *NoAuthProvider) SetAuthHeaders(req *http.Request) error {
	// No authentication headers to set
	return nil
}

// AuthenticateRequest does nothing for no authentication
func (p *NoAuthProvider) AuthenticateRequest(req *http.Request) error {
	// No authentication needed
	return nil
}

// RefreshToken does nothing for no authentication
func (p *NoAuthProvider) RefreshToken(ctx context.Context) error {
	return nil
}

// IsValid returns false for no authentication
func (p *NoAuthProvider) IsValid() bool {
	return false
}

// TokenProvider provides authentication using a personal access token
type TokenProvider struct {
	BaseAuthProvider
	token string
}

// NewTokenProvider creates a new provider with token authentication
func NewTokenProvider(token string, logger observability.Logger) *TokenProvider {
	return &TokenProvider{
		BaseAuthProvider: BaseAuthProvider{
			authType: AuthTypeToken,
			logger:   logger,
		},
		token: token,
	}
}

// GetToken returns the token
func (p *TokenProvider) GetToken(ctx context.Context) (string, error) {
	if p.token == "" {
		return "", fmt.Errorf("token is empty")
	}
	return p.token, nil
}

// SetAuthHeaders sets the token authentication header
func (p *TokenProvider) SetAuthHeaders(req *http.Request) error {
	if p.token == "" {
		return fmt.Errorf("token is empty")
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s", p.token))
	return nil
}

// AuthenticateRequest authenticates an HTTP request with the token
func (p *TokenProvider) AuthenticateRequest(req *http.Request) error {
	// Similar to SetAuthHeaders, this sets the token in the request headers
	return p.SetAuthHeaders(req)
}

// RefreshToken does nothing for token authentication
func (p *TokenProvider) RefreshToken(ctx context.Context) error {
	// Personal access tokens don't need to be refreshed
	return nil
}

// IsValid checks if the token is non-empty
func (p *TokenProvider) IsValid() bool {
	return p.token != ""
}

// AppProvider provides authentication using a GitHub App
type AppProvider struct {
	BaseAuthProvider
	appID          string
	privateKey     *rsa.PrivateKey
	installationID string
	token          string
	tokenExpiry    time.Time
	mutex          sync.RWMutex
}

// NewAppProvider creates a new provider with GitHub App authentication
func NewAppProvider(appID, privateKeyPEM, installationID string, logger observability.Logger) (*AppProvider, error) {
	// Validate required inputs
	if appID == "" {
		return nil, errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"GitHub App ID is required",
		)
	}

	if privateKeyPEM == "" {
		return nil, errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"GitHub App private key is required",
		)
	}

	if installationID == "" {
		logger.Warn("GitHub App installation ID not provided; only JWT authentication will be available",
			map[string]any{
				"app_id": appID,
			})
	}

	// Parse the private key from PEM format
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"failed to parse PEM block containing private key",
		)
	}

	// Try to parse the key using PKCS1 format
	var privateKey *rsa.PrivateKey
	var err error

	// Try multiple formats for better compatibility
	privateKey, err = jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return nil, errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"failed to parse private key",
		).WithContext("error", err.Error())
	}

	return &AppProvider{
		BaseAuthProvider: BaseAuthProvider{
			authType: AuthTypeApp,
			logger:   logger,
		},
		appID:          appID,
		privateKey:     privateKey,
		installationID: installationID,
		mutex:          sync.RWMutex{},
	}, nil
}

// GetToken returns a valid installation token
func (p *AppProvider) GetToken(ctx context.Context) (string, error) {
	p.mutex.RLock()
	token := p.token
	expiry := p.tokenExpiry
	p.mutex.RUnlock()

	// Check if token is valid
	if token != "" && time.Now().Before(expiry) {
		return token, nil
	}

	// Refresh the token
	if err := p.RefreshToken(ctx); err != nil {
		return "", err
	}

	p.mutex.RLock()
	token = p.token
	p.mutex.RUnlock()

	return token, nil
}

// SetAuthHeaders sets the app authentication headers
func (p *AppProvider) SetAuthHeaders(req *http.Request) error {
	token, err := p.GetToken(req.Context())
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return nil
}

// AuthenticateRequest authenticates an HTTP request with the app token
func (p *AppProvider) AuthenticateRequest(req *http.Request) error {
	// Similar to SetAuthHeaders, gets token and sets it in the request headers
	return p.SetAuthHeaders(req)
}

// RefreshToken obtains a new installation token
func (p *AppProvider) RefreshToken(ctx context.Context) error {
	// Check if installation ID is provided
	if p.installationID == "" {
		return errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"installation ID is required to refresh token",
		)
	}

	// Generate JWT for app authentication
	jwt, err := p.generateJWT()
	if err != nil {
		return errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"failed to generate JWT",
		).WithContext("error", err.Error())
	}

	// Create HTTP client with appropriate timeouts
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout: 5 * time.Second,
			IdleConnTimeout:     90 * time.Second,
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
		},
	}

	// Get installation token
	url := fmt.Sprintf("https://api.github.com/app/installations/%s/access_tokens", p.installationID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"failed to create request for installation token",
		).WithContext("error", err.Error())
	}

	// Set required headers
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"failed to get installation token",
		).WithContext("error", err.Error())
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			p.logger.Warn("Failed to close response body", map[string]interface{}{"error": err})
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"failed to read installation token response",
		).WithContext("error", err.Error())
	}

	// Check response status
	if resp.StatusCode != http.StatusCreated {
		// Try to parse error message
		var errorResp struct {
			Message          string `json:"message"`
			DocumentationURL string `json:"documentation_url"`
		}

		_ = json.Unmarshal(body, &errorResp) // Ignore unmarshaling errors

		return errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			resp.StatusCode,
			fmt.Sprintf("failed to get installation token: %s", errorResp.Message),
		).WithDocumentation(errorResp.DocumentationURL)
	}

	// Parse successful response
	var tokenResp struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"failed to parse token response",
		).WithContext("error", err.Error())
	}

	// Verify token was returned
	if tokenResp.Token == "" {
		return errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"no token returned in response",
		)
	}

	// Update token and expiry with thread safety
	p.mutex.Lock()
	p.token = tokenResp.Token
	p.tokenExpiry = tokenResp.ExpiresAt
	p.mutex.Unlock()

	// Log success (without exposing the token)
	p.logger.Info("Successfully refreshed GitHub installation token",
		map[string]any{
			"app_id":          p.appID,
			"installation_id": p.installationID,
			"expires_at":      tokenResp.ExpiresAt.Format(time.RFC3339),
		})

	return nil
}

// generateJWT generates a JWT for GitHub App authentication
func (p *AppProvider) generateJWT() (string, error) {
	// Validate that we have the necessary components
	if p.privateKey == nil {
		return "", errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"missing private key for JWT generation",
		)
	}

	if p.appID == "" {
		return "", errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"missing app ID for JWT generation",
		)
	}

	// Create token with claims
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		Issuer:    p.appID,
	}

	// Create and sign token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(p.privateKey)
	if err != nil {
		return "", errors.NewGitHubError(
			errors.ErrInvalidAuthentication,
			0,
			"failed to sign JWT",
		).WithContext("error", err.Error())
	}

	return signedToken, nil
}

// IsValid checks if the app authentication is valid
func (p *AppProvider) IsValid() bool {
	return p.appID != "" && p.privateKey != nil && p.installationID != ""
}

// OAuthProvider provides authentication using OAuth tokens
type OAuthProvider struct {
	BaseAuthProvider
	token        string
	clientID     string
	clientSecret string
	tokenExpiry  time.Time
	mutex        sync.RWMutex
}

// NewOAuthProvider creates a new provider with OAuth authentication
func NewOAuthProvider(token, clientID, clientSecret string, logger observability.Logger) *OAuthProvider {
	return &OAuthProvider{
		BaseAuthProvider: BaseAuthProvider{
			authType: AuthTypeOAuth,
			logger:   logger,
		},
		token:        token,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// GetToken returns the OAuth token
func (p *OAuthProvider) GetToken(ctx context.Context) (string, error) {
	p.mutex.RLock()
	token := p.token
	expiry := p.tokenExpiry
	p.mutex.RUnlock()

	// Check if token is valid
	if token != "" && (expiry.IsZero() || time.Now().Before(expiry)) {
		return token, nil
	}

	// If we have client credentials but no token, refresh it
	if p.clientID != "" && p.clientSecret != "" {
		if err := p.RefreshToken(ctx); err != nil {
			return "", err
		}

		p.mutex.RLock()
		token = p.token
		p.mutex.RUnlock()

		return token, nil
	}

	// If we have a token but no client credentials, just return it
	if token != "" {
		return token, nil
	}

	return "", fmt.Errorf("no valid OAuth token available")
}

// SetAuthHeaders sets the OAuth authentication header
func (p *OAuthProvider) SetAuthHeaders(req *http.Request) error {
	token, err := p.GetToken(req.Context())
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	return nil
}

// AuthenticateRequest authenticates an HTTP request with the OAuth token
func (p *OAuthProvider) AuthenticateRequest(req *http.Request) error {
	// Similar to SetAuthHeaders, gets token and sets it in the request headers
	return p.SetAuthHeaders(req)
}

// RefreshToken refreshes the OAuth token if possible
func (p *OAuthProvider) RefreshToken(ctx context.Context) error {
	// We need client credentials to refresh
	if p.clientID == "" || p.clientSecret == "" {
		return fmt.Errorf("client ID and secret required to refresh OAuth token")
	}

	// Create HTTP client
	client := &http.Client{Timeout: 10 * time.Second}

	// Build request for token refresh
	// This is a simplified example, actual implementation may vary based on your OAuth flow
	url := "https://github.com/login/oauth/access_token"
	reqBody := fmt.Sprintf("client_id=%s&client_secret=%s&grant_type=client_credentials",
		p.clientID, p.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			p.logger.Warn("Failed to close response body", map[string]interface{}{"error": err})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to refresh token: status code %d", resp.StatusCode)
	}

	// Parse response
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	// Update token and expiry
	p.mutex.Lock()
	p.token = tokenResp.AccessToken

	// Set expiry if available
	if tokenResp.ExpiresIn > 0 {
		p.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	} else {
		// Default to no expiry
		p.tokenExpiry = time.Time{}
	}

	p.mutex.Unlock()

	return nil
}

// IsValid checks if the OAuth authentication is valid
func (p *OAuthProvider) IsValid() bool {
	return p.token != ""
}

// AuthProviderFactory creates authentication providers
type AuthProviderFactory struct {
	configs map[string]*Config
	logger  observability.Logger // Changed from pointer to interface type
	cache   map[string]AuthProvider
	mutex   sync.RWMutex
}

// NewAuthProviderFactory creates a new authentication provider factory
func NewAuthProviderFactory(configs map[string]*Config, logger observability.Logger) *AuthProviderFactory {
	return &AuthProviderFactory{
		configs: configs,
		logger:  logger,
		cache:   make(map[string]AuthProvider),
	}
}

// GetProvider gets or creates an authentication provider
func (f *AuthProviderFactory) GetProvider(name string) (AuthProvider, error) {
	// First check cache
	f.mutex.RLock()
	provider, exists := f.cache[name]
	f.mutex.RUnlock()

	if exists {
		return provider, nil
	}

	// Get configuration
	config, exists := f.configs[name]
	if !exists {
		return nil, fmt.Errorf("no authentication configuration found for %s", name)
	}

	// Create provider
	provider, err := NewAuthProvider(config, f.logger)
	if err != nil {
		return nil, err
	}

	// Cache provider
	f.mutex.Lock()
	f.cache[name] = provider
	f.mutex.Unlock()

	return provider, nil
}

// AddConfig adds a new authentication configuration
func (f *AuthProviderFactory) AddConfig(name string, config *Config) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.configs[name] = config

	// Remove from cache if exists
	delete(f.cache, name)
}

// RemoveConfig removes an authentication configuration
func (f *AuthProviderFactory) RemoveConfig(name string) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	delete(f.configs, name)
	delete(f.cache, name)
}
