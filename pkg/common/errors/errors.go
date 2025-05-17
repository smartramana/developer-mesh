// Package errors provides common error types and utilities for the MCP system.
package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrorType represents the type of error
type ErrorType string

const (
	// ErrorTypeNotFound indicates a resource was not found
	ErrorTypeNotFound ErrorType = "NOT_FOUND"
	
	// ErrorTypeBadRequest indicates a client error in the request
	ErrorTypeBadRequest ErrorType = "BAD_REQUEST"
	
	// ErrorTypeInternal indicates an internal server error
	ErrorTypeInternal ErrorType = "INTERNAL"
	
	// ErrorTypeUnauthorized indicates an authentication failure
	ErrorTypeUnauthorized ErrorType = "UNAUTHORIZED"
	
	// ErrorTypeForbidden indicates an authorization failure
	ErrorTypeForbidden ErrorType = "FORBIDDEN"
	
	// ErrorTypeConflict indicates a conflict with existing resources
	ErrorTypeConflict ErrorType = "CONFLICT"
	
	// ErrorTypeLimitExceeded indicates a limit (e.g. rate limit) has been exceeded
	ErrorTypeLimitExceeded ErrorType = "LIMIT_EXCEEDED"

	// GitHub specific error types
	// ErrorTypeInvalidAuthentication indicates invalid GitHub auth credentials
	ErrorTypeInvalidAuthentication ErrorType = "INVALID_AUTHENTICATION"

	// ErrorTypeInvalidWebhook indicates an invalid GitHub webhook
	ErrorTypeInvalidWebhook ErrorType = "INVALID_WEBHOOK"

	// ErrorTypeInvalidSignature indicates an invalid webhook signature
	ErrorTypeInvalidSignature ErrorType = "INVALID_SIGNATURE"

	// ErrorTypeDuplicateDelivery indicates a duplicate webhook delivery
	ErrorTypeDuplicateDelivery ErrorType = "DUPLICATE_DELIVERY"

	// ErrorTypeGraphQLRequest indicates an error with GraphQL request
	ErrorTypeGraphQLRequest ErrorType = "GRAPHQL_REQUEST"

	// ErrorTypeGraphQLResponse indicates an error with GraphQL response
	ErrorTypeGraphQLResponse ErrorType = "GRAPHQL_RESPONSE"
	
	// ErrorTypeInvalidPayload indicates an invalid payload
	ErrorTypeInvalidPayload ErrorType = "INVALID_PAYLOAD"
)

// AdapterError represents an error from an adapter
type AdapterError struct {
	AdapterType    string
	Operation      string
	OriginalError  error
	ErrorCode      string
	ErrorType      ErrorType
	Retryable      bool
	Context        map[string]interface{}
	Resource       string
	Documentation  string
}

// Error returns a string representation of the error
func (e *AdapterError) Error() string {
	if e.OriginalError != nil {
		return fmt.Sprintf("%s.%s error: %s (%s)", e.AdapterType, e.Operation, e.OriginalError.Error(), e.ErrorCode)
	}
	return fmt.Sprintf("%s.%s error (%s)", e.AdapterType, e.Operation, e.ErrorCode)
}

// New creates a new adapter error
func New(
	adapterType string,
	operation string,
	originalError error,
	errorCode string,
	errorType ErrorType,
	retryable bool,
	context map[string]interface{},
) *AdapterError {
	return &AdapterError{
		AdapterType:    adapterType,
		Operation:      operation,
		OriginalError:  originalError,
		ErrorCode:      errorCode,
		ErrorType:      errorType,
		Retryable:      retryable,
		Context:        context,
	}
}

// NewWithoutContext creates a new adapter error without context
func NewWithoutContext(
	adapterType string,
	operation string,
	originalError error,
	errorCode string,
	errorType ErrorType,
	retryable bool,
) *AdapterError {
	return New(adapterType, operation, originalError, errorCode, errorType, retryable, nil)
}

// IsNotFound returns true if the error indicates a resource was not found
func IsNotFound(err error) bool {
	if adapterErr, ok := err.(*AdapterError); ok {
		return adapterErr.ErrorType == ErrorTypeNotFound
	}
	return false
}

// IsBadRequest returns true if the error indicates a client error
func IsBadRequest(err error) bool {
	if adapterErr, ok := err.(*AdapterError); ok {
		return adapterErr.ErrorType == ErrorTypeBadRequest
	}
	return false
}

// IsInternal returns true if the error indicates an internal server error
func IsInternal(err error) bool {
	if adapterErr, ok := err.(*AdapterError); ok {
		return adapterErr.ErrorType == ErrorTypeInternal
	}
	return false
}

// IsUnauthorized returns true if the error indicates an authentication failure
func IsUnauthorized(err error) bool {
	if adapterErr, ok := err.(*AdapterError); ok {
		return adapterErr.ErrorType == ErrorTypeUnauthorized
	}
	return false
}

// IsForbidden returns true if the error indicates an authorization failure
func IsForbidden(err error) bool {
	if adapterErr, ok := err.(*AdapterError); ok {
		return adapterErr.ErrorType == ErrorTypeForbidden
	}
	return false
}

// IsConflict returns true if the error indicates a conflict
func IsConflict(err error) bool {
	if adapterErr, ok := err.(*AdapterError); ok {
		return adapterErr.ErrorType == ErrorTypeConflict
	}
	return false
}

// IsRetryable returns true if the error is retryable
func IsRetryable(err error) bool {
	if adapterErr, ok := err.(*AdapterError); ok {
		return adapterErr.Retryable
	}
	return false
}

// GitHub specific error constants
var (
	// ErrInvalidAuthentication represents an authentication error
	ErrInvalidAuthentication = &AdapterError{
		AdapterType:   "GitHub",
		Operation:     "Authentication",
		ErrorCode:     "INVALID_CREDENTIALS",
		ErrorType:     ErrorTypeInvalidAuthentication,
		Retryable:     false,
	}

	// ErrInvalidWebhook represents an invalid webhook error
	ErrInvalidWebhook = &AdapterError{
		AdapterType:   "GitHub",
		Operation:     "WebhookValidation",
		ErrorCode:     "INVALID_WEBHOOK",
		ErrorType:     ErrorTypeInvalidWebhook,
		Retryable:     false,
	}

	// ErrInvalidSignature represents an invalid webhook signature error
	ErrInvalidSignature = &AdapterError{
		AdapterType:   "GitHub",
		Operation:     "WebhookValidation",
		ErrorCode:     "INVALID_SIGNATURE",
		ErrorType:     ErrorTypeInvalidSignature,
		Retryable:     false,
	}

	// ErrDuplicateDelivery represents a duplicate webhook delivery error
	ErrDuplicateDelivery = &AdapterError{
		AdapterType:   "GitHub",
		Operation:     "WebhookDelivery",
		ErrorCode:     "DUPLICATE_DELIVERY",
		ErrorType:     ErrorTypeDuplicateDelivery,
		Retryable:     false,
	}

	// ErrGraphQLRequest represents a GraphQL request error
	ErrGraphQLRequest = &AdapterError{
		AdapterType:   "GitHub",
		Operation:     "GraphQLRequest",
		ErrorCode:     "GRAPHQL_REQUEST_ERROR",
		ErrorType:     ErrorTypeGraphQLRequest,
		Retryable:     false,
	}

	// ErrGraphQLResponse represents a GraphQL response error
	ErrGraphQLResponse = &AdapterError{
		AdapterType:   "GitHub",
		Operation:     "GraphQLResponse",
		ErrorCode:     "GRAPHQL_RESPONSE_ERROR",
		ErrorType:     ErrorTypeGraphQLResponse,
		Retryable:     false,
	}

	// ErrInvalidPayload represents an invalid payload error
	ErrInvalidPayload = &AdapterError{
		AdapterType:   "GitHub",
		Operation:     "PayloadValidation",
		ErrorCode:     "INVALID_PAYLOAD",
		ErrorType:     ErrorTypeInvalidPayload,
		Retryable:     false,
	}
)

// NewGitHubError creates a new GitHub-specific error
func NewGitHubError(errTypeOrError interface{}, statusCode int, message string) *AdapterError {
	var err *AdapterError

	switch et := errTypeOrError.(type) {
	case *AdapterError:
		// If an AdapterError is passed directly
		err = &AdapterError{
			AdapterType:    "GitHub",
			Operation:      et.Operation,
			OriginalError:  fmt.Errorf("%s", message),
			ErrorCode:      et.ErrorCode,
			ErrorType:      et.ErrorType,
			Retryable:      et.Retryable,
			Context:        make(map[string]interface{}),
		}
	case error:
		// If a regular error is passed
		err = &AdapterError{
			AdapterType:    "GitHub",
			Operation:      "Operation",
			OriginalError:  et,
			ErrorCode:      "GITHUB_ERROR",
			ErrorType:      ErrorTypeInternal,
			Retryable:      false,
			Context:        make(map[string]interface{}),
		}
		err.Context["error_message"] = message
	default:
		// Default case for any other input
		err = &AdapterError{
			AdapterType:    "GitHub",
			Operation:      "Unknown",
			OriginalError:  fmt.Errorf("%s", message),
			ErrorCode:      "GITHUB_ERROR",
			ErrorType:      ErrorTypeInternal,
			Retryable:      false,
			Context:        make(map[string]interface{}),
		}
	}

	if statusCode > 0 {
		err.Context["status_code"] = statusCode
	}

	return err
}

// NewAdapterError creates a new adapter error with a common structure
func NewAdapterError(adapterType string, statusCode int, message string) *AdapterError {
	return &AdapterError{
		AdapterType:    adapterType,
		Operation:      "Unknown",
		OriginalError:  fmt.Errorf("%s", message),
		ErrorCode:      "ADAPTER_ERROR",
		ErrorType:      ErrorTypeInternal,
		Retryable:      false,
		Context:        map[string]interface{}{"status_code": statusCode},
	}
}

// WithContext adds context to the error and returns it
func (e *AdapterError) WithContext(key string, value interface{}) *AdapterError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithResource adds resource information to an adapter error
// Can be called with a key-value pair or just a resource name
func (e *AdapterError) WithResource(resource string, value ...string) *AdapterError {
	if len(value) > 0 {
		e.Resource = fmt.Sprintf("%s:%s", resource, value[0])
	} else {
		e.Resource = resource
	}
	return e
}

// WithOperation adds operation information to an adapter error
// Can be called with a key-value pair or just an operation name
func (e *AdapterError) WithOperation(operation string, value ...string) *AdapterError {
	if len(value) > 0 {
		e.Operation = fmt.Sprintf("%s:%s", operation, value[0])
	} else {
		e.Operation = operation
	}
	return e
}

// ErrRateLimitExceeded represents a rate limit exceeded error
var ErrRateLimitExceeded = &AdapterError{
	AdapterType:   "GitHub",
	Operation:     "RateLimit",
	ErrorCode:     "RATE_LIMIT_EXCEEDED",
	ErrorType:     ErrorTypeLimitExceeded,
	Retryable:     true,
	Context:       map[string]interface{}{"retry_after": "60s"},
}

// GitHubError represents a GitHub API error response
type GitHubError struct {
	Code            string `json:"code"`
	Message         string `json:"message"`
	DocumentationURL string `json:"documentation_url,omitempty"`
	Status          int    `json:"status,omitempty"`
	Resource        string `json:"-"`
	ResourceID      string `json:"-"`
}

// Error returns the error message to implement the error interface
func (e *GitHubError) Error() string {
	if e.DocumentationURL != "" {
		return fmt.Sprintf("%s (docs: %s)", e.Message, e.DocumentationURL)
	}
	return e.Message
}

// FromWebhookError converts a webhook error to an AdapterError
func FromWebhookError(err error, statusCode int) *AdapterError {
	if err == nil {
		return nil
	}
	
	ghErr := &GitHubError{}
	if e, ok := err.(*AdapterError); ok {
		return e
	}
	
	// Check if it's already a GitHub error from JSON
	if jsonErr := json.Unmarshal([]byte(err.Error()), ghErr); jsonErr == nil && ghErr.Message != "" {
		return NewGitHubError(ErrInvalidWebhook, statusCode, ghErr.Message)
	}
	
	return NewGitHubError(ErrInvalidWebhook, statusCode, err.Error())
}

// FromHTTPError converts an HTTP error to an AdapterError
// Supports both http.Response + error and statusCode + message + doc formats
func FromHTTPError(respOrStatus interface{}, errOrMessage interface{}, docURL ...string) *AdapterError {
	// Case 1: Called with (http.Response, error)
	if resp, ok := respOrStatus.(*http.Response); ok {
		err, _ := errOrMessage.(error)
		if err != nil {
			return NewGitHubError("http_error", resp.StatusCode, err.Error())
		}
		
		ghErr := &GitHubError{}
		if err := json.NewDecoder(resp.Body).Decode(ghErr); err != nil {
			return NewGitHubError("http_error", resp.StatusCode, fmt.Sprintf("Error decoding response: %s", err.Error()))
		}
		
		return NewGitHubError("github_api_error", resp.StatusCode, ghErr.Message)
	}
	
	// Case 2: Called with (statusCode, message, docURL)
	if statusCode, ok := respOrStatus.(int); ok {
		message, _ := errOrMessage.(string)
		adapterErr := NewGitHubError("github_api_error", statusCode, message)
		
		// Add documentation URL if provided
		if len(docURL) > 0 && docURL[0] != "" {
			adapterErr = adapterErr.WithDocumentation(docURL[0])
		}
		
		return adapterErr
	}
	
	// Fallback case - shouldn't happen but don't crash
	return NewGitHubError("github_api_error", 500, "Invalid parameters to FromHTTPError")
}

// WithDocumentation adds documentation URL to the error
func (e *AdapterError) WithDocumentation(docURL string) *AdapterError {
	e.Documentation = docURL
	return e
}

// WithResource adds resource information to the error
func (e *GitHubError) WithResource(resourceType, resourceID string) *GitHubError {
	e.Resource = resourceType
	e.ResourceID = resourceID
	return e
}
