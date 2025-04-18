package errors

import (
	"errors"
	"fmt"
)

// ErrorType represents the type of adapter error
type ErrorType int

const (
	// ErrorTypeUnknown represents an unknown error
	ErrorTypeUnknown ErrorType = iota
	
	// ErrorTypeAuthorization represents an authorization error
	ErrorTypeAuthorization
	
	// ErrorTypeRateLimit represents a rate limit error
	ErrorTypeRateLimit
	
	// ErrorTypeService represents a service error
	ErrorTypeService
	
	// ErrorTypeNetwork represents a network error
	ErrorTypeNetwork
	
	// ErrorTypeValidation represents a validation error
	ErrorTypeValidation
	
	// ErrorTypeConfiguration represents a configuration error
	ErrorTypeConfiguration
	
	// ErrorTypeTimeout represents a timeout error
	ErrorTypeTimeout
)

// Error codes for common adapter errors
const (
	// Authorization errors
	ErrCodeUnauthorized        = "UNAUTHORIZED"
	ErrCodeForbidden           = "FORBIDDEN"
	ErrCodeInvalidCredentials  = "INVALID_CREDENTIALS"
	ErrCodeTokenExpired        = "TOKEN_EXPIRED"
	
	// Rate limiting errors
	ErrCodeRateLimitExceeded   = "RATE_LIMIT_EXCEEDED"
	ErrCodeTooManyRequests     = "TOO_MANY_REQUESTS"
	
	// Service errors
	ErrCodeServiceUnavailable  = "SERVICE_UNAVAILABLE"
	ErrCodeInternalServerError = "INTERNAL_SERVER_ERROR"
	ErrCodeBadGateway          = "BAD_GATEWAY"
	
	// Network errors
	ErrCodeConnectionFailed    = "CONNECTION_FAILED"
	ErrCodeDNSResolutionFailed = "DNS_RESOLUTION_FAILED"
	ErrCodeTimeoutError        = "TIMEOUT_ERROR"
	
	// Validation errors
	ErrCodeInvalidRequest      = "INVALID_REQUEST"
	ErrCodeInvalidParameter    = "INVALID_PARAMETER"
	ErrCodeResourceNotFound    = "RESOURCE_NOT_FOUND"
	
	// Configuration errors
	ErrCodeInvalidConfiguration = "INVALID_CONFIGURATION"
	ErrCodeMissingConfiguration = "MISSING_CONFIGURATION"
	
	// Unknown errors
	ErrCodeUnknown             = "UNKNOWN_ERROR"
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
}

// Error implements the error interface
func (e *AdapterError) Error() string {
	return fmt.Sprintf("adapter %s operation %s failed: %v (code: %s, type: %v, retryable: %v)",
		e.AdapterType, e.Operation, e.OriginalError, e.ErrorCode, e.ErrorType, e.Retryable)
}

// Unwrap implements the errors.Unwrap interface
func (e *AdapterError) Unwrap() error {
	return e.OriginalError
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
	if context == nil {
		context = make(map[string]interface{})
	}
	
	return &AdapterError{
		AdapterType:   adapterType,
		Operation:     operation,
		OriginalError: originalError,
		ErrorCode:     errorCode,
		ErrorType:     errorType,
		Retryable:     retryable,
		Context:       context,
	}
}

// NewUnauthorizedError creates a new unauthorized error
func NewUnauthorizedError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeUnauthorized, ErrorTypeAuthorization, false, context)
}

// NewForbiddenError creates a new forbidden error
func NewForbiddenError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeForbidden, ErrorTypeAuthorization, false, context)
}

// NewInvalidCredentialsError creates a new invalid credentials error
func NewInvalidCredentialsError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeInvalidCredentials, ErrorTypeAuthorization, false, context)
}

// NewTokenExpiredError creates a new token expired error
func NewTokenExpiredError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeTokenExpired, ErrorTypeAuthorization, true, context)
}

// NewRateLimitExceededError creates a new rate limit exceeded error
func NewRateLimitExceededError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeRateLimitExceeded, ErrorTypeRateLimit, true, context)
}

// NewTooManyRequestsError creates a new too many requests error
func NewTooManyRequestsError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeTooManyRequests, ErrorTypeRateLimit, true, context)
}

// NewServiceUnavailableError creates a new service unavailable error
func NewServiceUnavailableError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeServiceUnavailable, ErrorTypeService, true, context)
}

// NewInternalServerError creates a new internal server error
func NewInternalServerError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeInternalServerError, ErrorTypeService, true, context)
}

// NewBadGatewayError creates a new bad gateway error
func NewBadGatewayError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeBadGateway, ErrorTypeService, true, context)
}

// NewConnectionFailedError creates a new connection failed error
func NewConnectionFailedError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeConnectionFailed, ErrorTypeNetwork, true, context)
}

// NewDNSResolutionFailedError creates a new DNS resolution failed error
func NewDNSResolutionFailedError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeDNSResolutionFailed, ErrorTypeNetwork, true, context)
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeTimeoutError, ErrorTypeTimeout, true, context)
}

// NewInvalidRequestError creates a new invalid request error
func NewInvalidRequestError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeInvalidRequest, ErrorTypeValidation, false, context)
}

// NewInvalidParameterError creates a new invalid parameter error
func NewInvalidParameterError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeInvalidParameter, ErrorTypeValidation, false, context)
}

// NewResourceNotFoundError creates a new resource not found error
func NewResourceNotFoundError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeResourceNotFound, ErrorTypeValidation, false, context)
}

// NewInvalidConfigurationError creates a new invalid configuration error
func NewInvalidConfigurationError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeInvalidConfiguration, ErrorTypeConfiguration, false, context)
}

// NewMissingConfigurationError creates a new missing configuration error
func NewMissingConfigurationError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeMissingConfiguration, ErrorTypeConfiguration, false, context)
}

// NewUnknownError creates a new unknown error
func NewUnknownError(adapterType string, operation string, originalError error, context map[string]interface{}) *AdapterError {
	return New(adapterType, operation, originalError, ErrCodeUnknown, ErrorTypeUnknown, true, context)
}

// IsRetryable checks if the error is retryable
func IsRetryable(err error) bool {
	var adapterErr *AdapterError
	if errors.As(err, &adapterErr) {
		return adapterErr.Retryable
	}
	// Default to true for unknown errors to be safe
	return true
}

// GetErrorType gets the error type
func GetErrorType(err error) ErrorType {
	var adapterErr *AdapterError
	if errors.As(err, &adapterErr) {
		return adapterErr.ErrorType
	}
	return ErrorTypeUnknown
}

// GetErrorCode gets the error code
func GetErrorCode(err error) string {
	var adapterErr *AdapterError
	if errors.As(err, &adapterErr) {
		return adapterErr.ErrorCode
	}
	return ErrCodeUnknown
}

// GetAdapterType gets the adapter type
func GetAdapterType(err error) string {
	var adapterErr *AdapterError
	if errors.As(err, &adapterErr) {
		return adapterErr.AdapterType
	}
	return ""
}

// GetOperation gets the operation
func GetOperation(err error) string {
	var adapterErr *AdapterError
	if errors.As(err, &adapterErr) {
		return adapterErr.Operation
	}
	return ""
}

// GetContext gets the error context
func GetContext(err error) map[string]interface{} {
	var adapterErr *AdapterError
	if errors.As(err, &adapterErr) {
		return adapterErr.Context
	}
	return nil
}

// IsSpecificErrorCode checks if the error has a specific error code
func IsSpecificErrorCode(err error, errorCode string) bool {
	return GetErrorCode(err) == errorCode
}

// IsSpecificErrorType checks if the error has a specific error type
func IsSpecificErrorType(err error, errorType ErrorType) bool {
	return GetErrorType(err) == errorType
}

// IsAuthorizationError checks if the error is an authorization error
func IsAuthorizationError(err error) bool {
	return IsSpecificErrorType(err, ErrorTypeAuthorization)
}

// IsRateLimitError checks if the error is a rate limit error
func IsRateLimitError(err error) bool {
	return IsSpecificErrorType(err, ErrorTypeRateLimit)
}

// IsServiceError checks if the error is a service error
func IsServiceError(err error) bool {
	return IsSpecificErrorType(err, ErrorTypeService)
}

// IsNetworkError checks if the error is a network error
func IsNetworkError(err error) bool {
	return IsSpecificErrorType(err, ErrorTypeNetwork)
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	return IsSpecificErrorType(err, ErrorTypeValidation)
}

// IsConfigurationError checks if the error is a configuration error
func IsConfigurationError(err error) bool {
	return IsSpecificErrorType(err, ErrorTypeConfiguration)
}

// IsTimeoutError checks if the error is a timeout error
func IsTimeoutError(err error) bool {
	return IsSpecificErrorType(err, ErrorTypeTimeout)
}
