package interfaces

import "errors"

// Common repository errors
var (
	ErrNotFound       = errors.New("entity not found")
	ErrDuplicate      = errors.New("entity already exists")
	ErrValidation     = errors.New("validation failed")
	ErrOptimisticLock = errors.New("optimistic lock failed")
)