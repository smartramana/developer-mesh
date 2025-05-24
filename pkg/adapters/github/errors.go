package github

import (
	"github.com/S-Corkum/devops-mcp/pkg/common/errors"
)

// GitHubErrorProvider interface defines methods for providing GitHub errors
type GitHubErrorProvider interface {
	NewError(err error, statusCode int, message string) *errors.GitHubError
	FromHTTPError(statusCode int, message, documentationURL string) *errors.GitHubError
	FromWebhookError(err error, eventType string) *errors.GitHubError
}

// errorProvider implements GitHubErrorProvider
type errorProvider struct{}

// NewGitHubErrorProvider returns a new GitHub error provider
func NewGitHubErrorProvider() GitHubErrorProvider {
	return &errorProvider{}
}

// NewError creates a new GitHub error
func (p *errorProvider) NewError(err error, statusCode int, message string) *errors.GitHubError {
	return errors.NewGitHubError(err, statusCode, message)
}

// FromHTTPError creates a GitHub error from an HTTP error
func (p *errorProvider) FromHTTPError(statusCode int, message, documentationURL string) *errors.GitHubError {
	return errors.FromHTTPError(statusCode, message, documentationURL)
}

// FromWebhookError creates a GitHub error from a webhook error
func (p *errorProvider) FromWebhookError(err error, eventType string) *errors.GitHubError {
	return errors.FromWebhookError(err, eventType)
}
