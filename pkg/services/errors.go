package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// Service errors
var (
	ErrRateLimitExceeded       = errors.New("rate limit exceeded")
	ErrQuotaExceeded           = errors.New("quota exceeded")
	ErrUnauthorized            = errors.New("unauthorized")
	ErrInsufficientPermissions = errors.New("insufficient permissions")
	ErrConcurrentModification  = errors.New("concurrent modification")
	ErrNoEligibleAgents        = errors.New("no eligible agents")
	ErrNoCapableAgent          = errors.New("no capable agent")
	ErrDelegationDenied        = errors.New("delegation denied")
)

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// RateLimitError provides rate limit details
type RateLimitError struct {
	Key        string
	Limit      int
	Window     time.Duration
	RetryAfter time.Duration
}

func (e RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded: %s (limit: %d per %s, retry after: %s)",
		e.Key, e.Limit, e.Window, e.RetryAfter)
}

// QuotaError provides quota details
type QuotaError struct {
	TenantID uuid.UUID
	Resource string
	Limit    int64
	Current  int64
}

func (e QuotaError) Error() string {
	return fmt.Sprintf("quota exceeded: %s for tenant %s (limit: %d, current: %d)",
		e.Resource, e.TenantID, e.Limit, e.Current)
}

// DelegationError provides delegation denial details
type DelegationError struct {
	Reason string
}

func (e DelegationError) Error() string {
	return fmt.Sprintf("delegation denied: %s", e.Reason)
}

// UnauthorizedError provides authorization failure details
type UnauthorizedError struct {
	Action string
	Reason string
}

func (e UnauthorizedError) Error() string {
	return fmt.Sprintf("unauthorized: %s - %s", e.Action, e.Reason)
}