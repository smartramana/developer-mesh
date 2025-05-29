package github

import (
	"github.com/S-Corkum/devops-mcp/pkg/common/errors"
)

// GitHubErrorProvider interface defines methods for providing GitHub errors
type GitHubErrorProvider interface {
	NewError(err error, statusCode int, message string) *errors.AdapterError
	FromHTTPError(statusCode int, message, documentationURL string) *errors.AdapterError
	FromWebhookError(err error, eventType string) *errors.AdapterError
}

// errorProvider implements GitHubErrorProvider
type errorProvider struct{}

// NewGitHubErrorProvider returns a new GitHub error provider
func NewGitHubErrorProvider() GitHubErrorProvider {
	return &errorProvider{}
}

// NewError creates a new GitHub error
func (p *errorProvider) NewError(err error, statusCode int, message string) *errors.AdapterError {
	return errors.NewGitHubError(err, statusCode, message)
}

// FromHTTPError creates a GitHub error from an HTTP error
func (p *errorProvider) FromHTTPError(statusCode int, message, documentationURL string) *errors.AdapterError {
	return errors.FromHTTPError(statusCode, message, documentationURL)
}

// FromWebhookError creates a GitHub error from a webhook error
func (p *errorProvider) FromWebhookError(err error, eventType string) *errors.AdapterError {
	// Since FromWebhookError doesn't exist for AdapterError, use NewGitHubError
	return errors.NewGitHubError(err, 0, "webhook error: "+eventType)
}
