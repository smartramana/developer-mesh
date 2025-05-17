package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ErrorCode represents a standardized error code
type ErrorCode string

// Standard error codes
const (
	// General errors
	ErrBadRequest       ErrorCode = "BAD_REQUEST"
	ErrUnauthorized     ErrorCode = "UNAUTHORIZED"
	ErrForbidden        ErrorCode = "FORBIDDEN"
	ErrNotFound         ErrorCode = "NOT_FOUND"
	ErrMethodNotAllowed ErrorCode = "METHOD_NOT_ALLOWED"
	ErrConflict         ErrorCode = "CONFLICT"
	ErrTooManyRequests  ErrorCode = "TOO_MANY_REQUESTS"
	ErrInternalServer   ErrorCode = "INTERNAL_SERVER_ERROR"
	ErrServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"

	// Domain-specific errors
	ErrContextNotFound  ErrorCode = "CONTEXT_NOT_FOUND"
	ErrContextTooLarge  ErrorCode = "CONTEXT_TOO_LARGE"
	ErrContextInvalid   ErrorCode = "CONTEXT_INVALID"
	
	// Vector/embedding errors
	ErrEmbeddingFailed  ErrorCode = "EMBEDDING_FAILED"
	ErrEmbeddingInvalid ErrorCode = "EMBEDDING_INVALID"
	
	// Tool-specific errors
	ErrToolNotFound     ErrorCode = "TOOL_NOT_FOUND"
	ErrActionNotFound   ErrorCode = "ACTION_NOT_FOUND"
	ErrActionFailed     ErrorCode = "ACTION_FAILED"
	ErrActionInvalid    ErrorCode = "ACTION_INVALID"
	
	// Model-specific errors
	ErrModelNotFound    ErrorCode = "MODEL_NOT_FOUND"
	ErrModelInvalid     ErrorCode = "MODEL_INVALID"
	
	// Validation errors
	ErrValidationFailed ErrorCode = "VALIDATION_FAILED"
)

// ErrorResponse is the standard response format for errors
type ErrorResponse struct {
	Code     ErrorCode   `json:"code"`
	Message  string      `json:"message"`
	Details  interface{} `json:"details,omitempty"`
	TraceID  string      `json:"trace_id,omitempty"`
}

// APIError represents an API error with associated metadata
type APIError struct {
	Code     ErrorCode
	Message  string
	Details  interface{}
	HTTPCode int
	Err      error
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *APIError) Unwrap() error {
	return e.Err
}

// NewAPIError creates a new API error
func NewAPIError(code ErrorCode, message string, httpCode int, err error) *APIError {
	return &APIError{
		Code:     code,
		Message:  message,
		HTTPCode: httpCode,
		Err:      err,
	}
}

// WithDetails adds details to the error
func (e *APIError) WithDetails(details interface{}) *APIError {
	e.Details = details
	return e
}

// NewBadRequestError creates a bad request error
func NewBadRequestError(message string, err error) *APIError {
	return NewAPIError(ErrBadRequest, message, http.StatusBadRequest, err)
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string, err error) *APIError {
	return NewAPIError(ErrUnauthorized, message, http.StatusUnauthorized, err)
}

// NewForbiddenError creates a forbidden error
func NewForbiddenError(message string, err error) *APIError {
	return NewAPIError(ErrForbidden, message, http.StatusForbidden, err)
}

// NewNotFoundError creates a not found error
func NewNotFoundError(message string, err error) *APIError {
	return NewAPIError(ErrNotFound, message, http.StatusNotFound, err)
}

// NewContextNotFoundError creates a context not found error
func NewContextNotFoundError(contextID string, err error) *APIError {
	return NewAPIError(ErrContextNotFound, fmt.Sprintf("Context with ID %s not found", contextID), http.StatusNotFound, err)
}

// NewInternalServerError creates an internal server error
func NewInternalServerError(message string, err error) *APIError {
	return NewAPIError(ErrInternalServer, message, http.StatusInternalServerError, err)
}

// NewValidationError creates a validation error
func NewValidationError(message string, details interface{}) *APIError {
	return NewAPIError(ErrValidationFailed, message, http.StatusBadRequest, nil).WithDetails(details)
}

// HandleValidationErrors converts field validation errors to a standardized format
func HandleValidationErrors(err error) *APIError {
	validationErrors := strings.Split(err.Error(), "\n")
	details := make(map[string]string)
	
	for _, e := range validationErrors {
		parts := strings.SplitN(e, ":", 2)
		if len(parts) == 2 {
			field := strings.TrimSpace(parts[0])
			message := strings.TrimSpace(parts[1])
			details[field] = message
		}
	}
	
	return NewValidationError("Validation failed", details)
}

// ErrorHandlerMiddleware catches and formats errors consistently
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process the request
		c.Next()
		
		// If there were errors, handle them
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			
			// Generate a trace ID for error tracking
			traceID := uuid.New().String()
			
			// Handle the error based on type
			var response ErrorResponse
			var httpCode int
			
			switch e := err.(type) {
			case *APIError:
				// Use the APIError directly
				httpCode = e.HTTPCode
				response = ErrorResponse{
					Code:    e.Code,
					Message: e.Message,
					Details: e.Details,
					TraceID: traceID,
				}
			default:
				// For unknown errors, return a generic internal server error
				httpCode = http.StatusInternalServerError
				response = ErrorResponse{
					Code:    ErrInternalServer,
					Message: "An internal server error occurred",
					TraceID: traceID,
				}
			}
			
			// Log the error with the trace ID for tracking
			c.Request.Context()
			// You would typically use a structured logger here
			// logger.WithField("trace_id", traceID).WithError(err).Error("API error")
			
			// Return the error response
			c.AbortWithStatusJSON(httpCode, response)
		}
	}
}
