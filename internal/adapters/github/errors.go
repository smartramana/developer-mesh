package github

import (
	"github.com/S-Corkum/mcp-server/internal/adapters/errors"
)

// Re-export error variables from the common errors package
var (
	// General GitHub errors
	ErrGitHubAPI          = errors.ErrGitHubAPI
	ErrInvalidCredentials = errors.ErrInvalidCredentials
	ErrPermissionDenied   = errors.ErrPermissionDenied
	ErrResourceNotFound   = errors.ErrResourceNotFound
	ErrRateLimitExceeded  = errors.ErrRateLimitExceeded
	ErrServerError        = errors.ErrServerError
	ErrValidationFailed   = errors.ErrValidationFailed
	ErrServiceUnavailable = errors.ErrServiceUnavailable

	// REST API specific errors
	ErrRESTRequest  = errors.ErrRESTRequest
	ErrRESTResponse = errors.ErrRESTResponse
	
	// GraphQL specific errors
	ErrGraphQLRequest  = errors.ErrGraphQLRequest
	ErrGraphQLResponse = errors.ErrGraphQLResponse
	
	// Webhook specific errors
	ErrInvalidWebhook    = errors.ErrInvalidWebhook
	ErrInvalidSignature  = errors.ErrInvalidSignature
	ErrInvalidPayload    = errors.ErrInvalidPayload
	ErrDuplicateDelivery = errors.ErrDuplicateDelivery
)

// GitHubError is an alias for errors.GitHubError
type GitHubError = errors.GitHubError

// Re-export functions from the common errors package
var (
	NewGitHubError    = errors.NewGitHubError
	FromHTTPError     = errors.FromHTTPError
	FromWebhookError  = errors.FromWebhookError
	IsRateLimitError  = errors.IsGitHubRateLimitError
	IsNotFoundError   = errors.IsGitHubNotFoundError
	IsAuthenticationError = errors.IsGitHubAuthenticationError
	IsPermissionError = errors.IsGitHubPermissionError
	IsValidationError = errors.IsGitHubValidationError
	IsServerError     = errors.IsGitHubServerError
)
