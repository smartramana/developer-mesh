package types

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Transaction represents a database transaction
type Transaction interface {
	Execute(ctx context.Context, fn func(ctx context.Context) error) error
	Savepoint(ctx context.Context, name string) error
	RollbackToSavepoint(ctx context.Context, name string) error
	Commit() error
	Rollback() error
}

// TxOptions configures transaction behavior
type TxOptions struct {
	Isolation IsolationLevel
	ReadOnly  bool
	Timeout   time.Duration
}

// IsolationLevel represents transaction isolation levels
type IsolationLevel int

const (
	IsolationDefault IsolationLevel = iota
	IsolationReadUncommitted
	IsolationReadCommitted
	IsolationRepeatableRead
	IsolationSerializable
)

// Common errors
var (
	ErrNotFound       = NewRepositoryError("NOT_FOUND", "entity not found")
	ErrAlreadyExists  = NewRepositoryError("ALREADY_EXISTS", "entity already exists")
	ErrOptimisticLock = NewRepositoryError("OPTIMISTIC_LOCK", "version mismatch")
	ErrInvalidInput   = NewRepositoryError("INVALID_INPUT", "invalid input parameters")
	ErrConstraintViolation = NewRepositoryError("CONSTRAINT_VIOLATION", "constraint violation")
)

// RepositoryError represents a repository-specific error
type RepositoryError struct {
	Code    string
	Message string
	Cause   error
}

func NewRepositoryError(code, message string) *RepositoryError {
	return &RepositoryError{
		Code:    code,
		Message: message,
	}
}

func (e *RepositoryError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *RepositoryError) WithCause(cause error) *RepositoryError {
	return &RepositoryError{
		Code:    e.Code,
		Message: e.Message,
		Cause:   cause,
	}
}

// PageInfo contains pagination metadata
type PageInfo struct {
	TotalCount int64
	HasMore    bool
	NextCursor string
	PrevCursor string
}

// SortOrder represents sort direction
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// TimeRange represents a time interval
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// IDList is a helper type for batch operations
type IDList []uuid.UUID

// StringList is a helper type for string arrays
type StringList []string

// IntegrityReport contains results of integrity checks
type IntegrityReport struct {
	CheckedAt        time.Time
	TotalChecked     int64
	IssuesFound      int64
	Issues           []IntegrityIssue
	Recommendations  []string
}

// IntegrityIssue represents a data integrity problem
type IntegrityIssue struct {
	Type        string
	Severity    string
	EntityID    uuid.UUID
	Description string
	FixScript   string
}

// TaskFilters contains filtering options for task queries
type TaskFilters struct {
	TenantID      *uuid.UUID
	Status        []string
	Priority      []string
	Types         []string
	AssignedTo    *string
	CreatedBy     *string
	ParentTaskID  *uuid.UUID
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	UpdatedAfter  *time.Time
	UpdatedBefore *time.Time
	
	// Pagination
	Limit      int
	Offset     int
	Cursor     string
	
	// Sorting
	SortBy     string
	SortOrder  string
}

// TaskStats contains statistics about tasks
type TaskStats struct {
	TotalTasks      int64
	CompletedTasks  int64
	FailedTasks     int64
	PendingTasks    int64
	AverageTime     time.Duration
	SuccessRate     float64
}

// WorkflowFilters contains filtering options for workflow queries
type WorkflowFilters struct {
	TenantID      *uuid.UUID
	Status        []string
	CreatedBy     *string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	Active        *bool
	
	// Pagination
	Limit      int
	Offset     int
	
	// Sorting
	SortBy     string
	SortOrder  string
}

// DocumentFilters contains filtering options for document queries
type DocumentFilters struct {
	WorkspaceID   *uuid.UUID
	Type          []string
	CreatedBy     *string
	LockedBy      *string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	
	// Pagination
	Limit      int
	Offset     int
	
	// Sorting
	SortBy     string
	SortOrder  string
}