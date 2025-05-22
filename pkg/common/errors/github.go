package errors

import (
	"errors"
	"fmt"
)

// Common GitHub API error types - defined here to avoid circular dependencies
var (
	// General GitHub errors - not already defined in errors.go
	ErrGitHubAPI          = errors.New("github api error")
	ErrNilLogger          = errors.New("nil logger")
	ErrInvalidCredentials = errors.New("invalid github credentials")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrResourceNotFound   = errors.New("resource not found")
	ErrServerError        = errors.New("github server error")
	ErrValidationFailed   = errors.New("validation failed")
	ErrServiceUnavailable = errors.New("github service unavailable")

	// REST API specific errors
	ErrRESTRequest  = errors.New("rest request failed")
	ErrRESTResponse = errors.New("invalid rest response")
	
	// Note: The following errors are already defined in errors.go,
	// so we don't redefine them here:
	// - ErrRateLimitExceeded
	// - ErrInvalidAuthentication
	// - ErrGraphQLRequest
	// - ErrGraphQLResponse
	// - ErrInvalidWebhook
	// - ErrInvalidSignature
	// - ErrInvalidPayload
	// - ErrDuplicateDelivery
)

// GitHubError represents a GitHub API error with context
type GitHubError struct {
	// Original error
	Err error
	
	// HTTP status code (if applicable)
	StatusCode int
	
	// Response message (if any)
	Message string
	
	// Documentation URL
	DocumentationURL string
	
	// Context information
	Resource    string            // Resource being accessed (repo, issue, etc.)
	ResourceID  string            // ID of the resource
	Operation   string            // Operation being performed (GET, POST, etc.)
	RequestPath string            // API path that was requested
	Context     map[string]string // Additional context information
}

// Error returns the error message
func (e *GitHubError) Error() string {
	msg := fmt.Sprintf("%s", e.Err)
	
	if e.StatusCode > 0 {
		msg = fmt.Sprintf("%s (HTTP %d)", msg, e.StatusCode)
	}
	
	if e.Message != "" {
		msg = fmt.Sprintf("%s: %s", msg, e.Message)
	}
	
	if e.Resource != "" {
		if e.ResourceID != "" {
			msg = fmt.Sprintf("%s [%s: %s]", msg, e.Resource, e.ResourceID)
		} else {
			msg = fmt.Sprintf("%s [%s]", msg, e.Resource)
		}
	}
	
	return msg
}

// Unwrap returns the wrapped error
func (e *GitHubError) Unwrap() error {
	return e.Err
}

// Is checks if the target error matches this error
func (e *GitHubError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// NewLegacyGitHubError creates a new legacy GitHub error based on the struct in this file
// This function is kept for backward compatibility with code that uses the legacy GitHubError struct
// New code should use the NewGitHubError function defined in errors.go
func NewLegacyGitHubError(err error, statusCode int, message string) *GitHubError {
	return &GitHubError{
		Err:        err,
		StatusCode: statusCode,
		Message:    message,
		Context:    make(map[string]string),
	}
}

// WithResource adds resource information to the error
func (e *GitHubError) WithResource(resource, id string) *GitHubError {
	e.Resource = resource
	e.ResourceID = id
	return e
}

// WithOperation adds operation information to the error
func (e *GitHubError) WithOperation(operation, path string) *GitHubError {
	e.Operation = operation
	e.RequestPath = path
	return e
}

// WithContext adds context information to the error
func (e *GitHubError) WithContext(key, value string) *GitHubError {
	e.Context[key] = value
	return e
}

// WithDocumentation adds documentation URL to the error
func (e *GitHubError) WithDocumentation(url string) *GitHubError {
	e.DocumentationURL = url
	return e
}

// Note: The following functions are already defined in errors.go, 
// so we're commenting them out here to prevent redeclaration errors
//
// FromHTTPError creates a GitHubError from an HTTP status code and message
// func FromHTTPError(statusCode int, message, documentationURL string) *GitHubError { ... }
//
// FromWebhookError creates a GitHubError from a webhook validation error
// func FromWebhookError(err error, eventType string) *GitHubError { ... }

// IsGitHubRateLimitError checks if the error is a GitHub rate limit error
func IsGitHubRateLimitError(err error) bool {
	var githubErr *GitHubError
	if errors.As(err, &githubErr) {
		return errors.Is(githubErr.Err, ErrRateLimitExceeded)
	}
	return errors.Is(err, ErrRateLimitExceeded)
}

// IsGitHubNotFoundError checks if the error is a GitHub not found error
func IsGitHubNotFoundError(err error) bool {
	var githubErr *GitHubError
	if errors.As(err, &githubErr) {
		return errors.Is(githubErr.Err, ErrResourceNotFound)
	}
	return errors.Is(err, ErrResourceNotFound)
}

// IsGitHubAuthenticationError checks if the error is a GitHub authentication error
func IsGitHubAuthenticationError(err error) bool {
	var githubErr *GitHubError
	if errors.As(err, &githubErr) {
		return errors.Is(githubErr.Err, ErrInvalidCredentials)
	}
	return errors.Is(err, ErrInvalidCredentials)
}

// IsGitHubPermissionError checks if the error is a GitHub permission error
func IsGitHubPermissionError(err error) bool {
	var githubErr *GitHubError
	if errors.As(err, &githubErr) {
		return errors.Is(githubErr.Err, ErrPermissionDenied)
	}
	return errors.Is(err, ErrPermissionDenied)
}

// IsGitHubValidationError checks if the error is a GitHub validation error
func IsGitHubValidationError(err error) bool {
	var githubErr *GitHubError
	if errors.As(err, &githubErr) {
		return errors.Is(githubErr.Err, ErrValidationFailed)
	}
	return errors.Is(err, ErrValidationFailed)
}

// IsGitHubServerError checks if the error is a GitHub server error
func IsGitHubServerError(err error) bool {
	var githubErr *GitHubError
	if errors.As(err, &githubErr) {
		return errors.Is(githubErr.Err, ErrServerError) || 
			   errors.Is(githubErr.Err, ErrServiceUnavailable)
	}
	return errors.Is(err, ErrServerError) || 
		   errors.Is(err, ErrServiceUnavailable)
}
