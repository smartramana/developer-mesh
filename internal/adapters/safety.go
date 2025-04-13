package adapters

import "errors"

// Common safety errors
var (
	// ErrOperationNotAllowed indicates an operation is not allowed by the adapter
	ErrOperationNotAllowed = errors.New("operation not allowed by security policy")
	
	// ErrMissingPermission indicates insufficient permissions for the operation
	ErrMissingPermission = errors.New("insufficient permissions for this operation")
)

// SafetyChecker defines the interface for adapter operation safety checks
type SafetyChecker interface {
	// IsSafeOperation determines if an operation is safe to perform based on security policies
	IsSafeOperation(operation string, params map[string]interface{}) (bool, error)
}

// DefaultSafetyCheck provides a default implementation of safety checks
func DefaultSafetyCheck(operation string, params map[string]interface{}) (bool, error) {
	// Default allows all operations
	return true, nil
}
