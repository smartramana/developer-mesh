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
	ErrNotFound                = errors.New("resource not found")
	ErrWorkflowNotActive       = errors.New("workflow is not active")
	ErrDocumentLocked          = errors.New("document is locked")
	ErrMergeConflict           = errors.New("merge conflict detected")

	// Additional errors
	ErrInvalidID        = errors.New("invalid ID format")
	ErrResourceNotFound = errors.New("resource not found")
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

// QuotaExceededError represents a quota exceeded error (alternative structure)
type QuotaExceededError struct {
	Resource string
	Used     int64
	Limit    int64
}

func (e QuotaExceededError) Error() string {
	return fmt.Sprintf("quota exceeded for %s: %d/%d used", e.Resource, e.Used, e.Limit)
}

// ConcurrentModificationError represents a concurrent modification conflict
type ConcurrentModificationError struct {
	Resource string
	ID       uuid.UUID
	Version  int
}

func (e ConcurrentModificationError) Error() string {
	return fmt.Sprintf("concurrent modification of %s %s (version %d)", e.Resource, e.ID, e.Version)
}

// NoEligibleAgentsError represents no eligible agents found
type NoEligibleAgentsError struct {
	TaskType     string
	Requirements []string
}

func (e NoEligibleAgentsError) Error() string {
	return fmt.Sprintf("no eligible agents found for task type '%s'", e.TaskType)
}

// Workflow-specific errors

// WorkflowNotActiveError represents an inactive workflow
type WorkflowNotActiveError struct {
	WorkflowID uuid.UUID
}

func (e WorkflowNotActiveError) Error() string {
	return fmt.Sprintf("workflow %s is not active", e.WorkflowID)
}

// StepNotFoundError represents a missing workflow step
type StepNotFoundError struct {
	WorkflowID uuid.UUID
	StepID     string
}

func (e StepNotFoundError) Error() string {
	return fmt.Sprintf("step %s not found in workflow %s", e.StepID, e.WorkflowID)
}

// StepDependencyError represents a step dependency failure
type StepDependencyError struct {
	StepID       string
	DependencyID string
	Status       string
}

func (e StepDependencyError) Error() string {
	return fmt.Sprintf("step %s depends on %s which is in status %s", e.StepID, e.DependencyID, e.Status)
}

// Workspace-specific errors

// WorkspaceFullError represents a full workspace
type WorkspaceFullError struct {
	WorkspaceID uuid.UUID
	MaxAgents   int
}

func (e WorkspaceFullError) Error() string {
	return fmt.Sprintf("workspace %s is full (max %d agents)", e.WorkspaceID, e.MaxAgents)
}

// AgentNotInWorkspaceError represents agent not in workspace
type AgentNotInWorkspaceError struct {
	AgentID     string
	WorkspaceID uuid.UUID
}

func (e AgentNotInWorkspaceError) Error() string {
	return fmt.Sprintf("agent %s is not in workspace %s", e.AgentID, e.WorkspaceID)
}

// Document-specific errors

// DocumentLockedError represents a locked document
type DocumentLockedError struct {
	DocumentID uuid.UUID
	LockedBy   string
}

func (e DocumentLockedError) Error() string {
	return fmt.Sprintf("document %s is locked by %s", e.DocumentID, e.LockedBy)
}

// MergeConflictError represents a document merge conflict
type MergeConflictError struct {
	DocumentID uuid.UUID
	Conflicts  []string
}

func (e MergeConflictError) Error() string {
	return fmt.Sprintf("merge conflict in document %s: %d conflicts", e.DocumentID, len(e.Conflicts))
}

// VersionMismatchError represents a document version mismatch
type VersionMismatchError struct {
	DocumentID      uuid.UUID
	ExpectedVersion int
	ActualVersion   int
}

func (e VersionMismatchError) Error() string {
	return fmt.Sprintf("version mismatch for document %s: expected %d, got %d",
		e.DocumentID, e.ExpectedVersion, e.ActualVersion)
}
