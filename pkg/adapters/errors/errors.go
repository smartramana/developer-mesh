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
	return New(adapterType, operation, originalError, ErrCodeUnknown, ErrorTypeUnknown, false, context)
}

// IsAdapterError checks if an error is an AdapterError
func IsAdapterError(err error) bool {
	var adapterErr *AdapterError
	return errors.As(err, &adapterErr)
}

// GetAdapterError gets the AdapterError from an error
func GetAdapterError(err error) *AdapterError {
	var adapterErr *AdapterError
	if errors.As(err, &adapterErr) {
		return adapterErr
	}
	return nil
}

// IsErrorType checks if an error is of a specific adapter error type
func IsErrorType(err error, errorType ErrorType) bool {
	adapterErr := GetAdapterError(err)
	if adapterErr == nil {
		return false
	}
	return adapterErr.ErrorType == errorType
}

// IsErrorCode checks if an error is of a specific adapter error code
func IsErrorCode(err error, errorCode string) bool {
	adapterErr := GetAdapterError(err)
	if adapterErr == nil {
		return false
	}
	return adapterErr.ErrorCode == errorCode
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	adapterErr := GetAdapterError(err)
	if adapterErr == nil {
		return false
	}
	return adapterErr.Retryable
}
