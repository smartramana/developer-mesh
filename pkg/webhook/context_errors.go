package webhook

import (
	"fmt"
)

// ContextLockError indicates a distributed lock acquisition failure
type ContextLockError struct {
	ContextID string
	TenantID  string
	Reason    string
}

func (e *ContextLockError) Error() string {
	return fmt.Sprintf("failed to lock context %s/%s: %s", e.TenantID, e.ContextID, e.Reason)
}

// StorageTransitionError indicates a storage tier transition failure
type StorageTransitionError struct {
	ContextID string
	FromState ContextState
	ToState   ContextState
	Err       error
}

func (e *StorageTransitionError) Error() string {
	return fmt.Sprintf("failed to transition context %s from %s to %s: %v",
		e.ContextID, e.FromState, e.ToState, e.Err)
}

// ColdStorageError indicates a failure accessing cold storage
type ColdStorageError struct {
	Operation string
	Path      string
	Err       error
}

func (e *ColdStorageError) Error() string {
	return fmt.Sprintf("cold storage %s operation failed for path %s: %v",
		e.Operation, e.Path, e.Err)
}

// BatchProcessingError indicates a batch operation failure
type BatchProcessingError struct {
	BatchSize    int
	FailureCount int
	Errors       []error
}

func (e *BatchProcessingError) Error() string {
	return fmt.Sprintf("batch processing failed: %d out of %d operations failed",
		e.FailureCount, e.BatchSize)
}
