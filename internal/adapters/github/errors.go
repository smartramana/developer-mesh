package github

import (
	"errors"
	
	adapterrors "github.com/S-Corkum/mcp-server/internal/adapters/errors"
	commonerrors "github.com/S-Corkum/mcp-server/internal/common/errors"
)

// Re-export functions from the commonerrors package
var (
	NewGitHubError            = commonerrors.NewGitHubError
	FromHTTPError             = commonerrors.FromHTTPError
	FromWebhookError          = commonerrors.FromWebhookError
	IsGitHubRateLimitError    = commonerrors.IsGitHubRateLimitError
	IsGitHubNotFoundError     = commonerrors.IsGitHubNotFoundError
	IsGitHubAuthenticationError = commonerrors.IsGitHubAuthenticationError
	IsGitHubPermissionError   = commonerrors.IsGitHubPermissionError
	IsGitHubValidationError   = commonerrors.IsGitHubValidationError
	IsGitHubServerError       = commonerrors.IsGitHubServerError
)

// GitHubError is an alias for commonerrors.GitHubError
type GitHubError = commonerrors.GitHubError

// GitHubErrorProvider interface defines methods for providing GitHub errors
type GitHubErrorProvider interface {
	NewError(err error, statusCode int, message string) *GitHubError
	FromHTTPError(statusCode int, message, documentationURL string) *GitHubError
	FromWebhookError(err error, eventType string) *GitHubError
}

// errorProvider implements GitHubErrorProvider
type errorProvider struct{}

// NewGitHubErrorProvider returns a new GitHub error provider
func NewGitHubErrorProvider() GitHubErrorProvider {
	return &errorProvider{}
}

// NewError creates a new GitHub error
func (p *errorProvider) NewError(err error, statusCode int, message string) *GitHubError {
	return NewGitHubError(err, statusCode, message)
}

// FromHTTPError creates a GitHub error from an HTTP error
func (p *errorProvider) FromHTTPError(statusCode int, message, documentationURL string) *GitHubError {
	return FromHTTPError(statusCode, message, documentationURL)
}

// FromWebhookError creates a GitHub error from a webhook error
func (p *errorProvider) FromWebhookError(err error, eventType string) *GitHubError {
	return FromWebhookError(err, eventType)
}
