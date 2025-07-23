package interfaces

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository/types"
	"github.com/google/uuid"
)

// DocumentFilters defines filtering options for document queries
type DocumentFilters struct {
	WorkspaceID   *uuid.UUID
	Type          []string
	CreatedBy     *string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	UpdatedAfter  *time.Time
	ContentType   []string
	Limit         int
	Offset        int
	SortBy        string
	SortOrder     types.SortOrder
}

// DocumentStats represents document usage statistics
type DocumentStats struct {
	TotalOperations     int64
	UniqueCollaborators int64
	ConflictCount       int64
	SizeBytes           int64
	VersionCount        int64
	LastModifiedAt      time.Time
}

// DocumentRepository defines the interface for collaborative document persistence
type DocumentRepository interface {
	// Transaction support
	WithTx(tx types.Transaction) DocumentRepository
	BeginTx(ctx context.Context, opts *types.TxOptions) (types.Transaction, error)

	// Basic CRUD operations
	Create(ctx context.Context, document *models.SharedDocument) error
	Get(ctx context.Context, id uuid.UUID) (*models.SharedDocument, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (*models.SharedDocument, error)
	Update(ctx context.Context, document *models.SharedDocument) error
	UpdateWithLock(ctx context.Context, document *models.SharedDocument, lockOwner string) error
	Delete(ctx context.Context, id uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// Query operations
	List(ctx context.Context, tenantID uuid.UUID, filters DocumentFilters) ([]*models.SharedDocument, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*models.SharedDocument, error)
	ListByCreator(ctx context.Context, createdBy string) ([]*models.SharedDocument, error)
	SearchDocuments(ctx context.Context, query string, filters DocumentFilters) ([]*models.SharedDocument, error)

	// Locking operations
	AcquireLock(ctx context.Context, documentID uuid.UUID, agentID string, duration time.Duration) error
	ReleaseLock(ctx context.Context, documentID uuid.UUID, agentID string) error
	ExtendLock(ctx context.Context, documentID uuid.UUID, agentID string, duration time.Duration) error
	GetLockInfo(ctx context.Context, documentID uuid.UUID) (*models.DocumentLock, error)
	ForceReleaseLock(ctx context.Context, documentID uuid.UUID) error

	// Operation tracking
	RecordOperation(ctx context.Context, operation *models.DocumentOperation) error
	GetOperations(ctx context.Context, documentID uuid.UUID, since time.Time) ([]*models.DocumentOperation, error)
	GetOperationsBySequence(ctx context.Context, documentID uuid.UUID, fromSeq, toSeq int64) ([]*models.DocumentOperation, error)
	GetPendingOperations(ctx context.Context, documentID uuid.UUID) ([]*models.DocumentOperation, error)
	MarkOperationApplied(ctx context.Context, operationID uuid.UUID) error

	// Conflict resolution
	RecordConflict(ctx context.Context, conflict *models.ConflictResolution) error
	GetConflicts(ctx context.Context, documentID uuid.UUID) ([]*models.ConflictResolution, error)
	GetUnresolvedConflicts(ctx context.Context, documentID uuid.UUID) ([]*models.ConflictResolution, error)
	ResolveConflict(ctx context.Context, conflictID uuid.UUID, resolution map[string]interface{}) error

	// Version management
	CreateSnapshot(ctx context.Context, documentID uuid.UUID, version int64) error
	GetSnapshot(ctx context.Context, documentID uuid.UUID, version int64) (*models.DocumentSnapshot, error)
	ListSnapshots(ctx context.Context, documentID uuid.UUID) ([]*models.DocumentSnapshot, error)

	// Analytics
	GetDocumentStats(ctx context.Context, documentID uuid.UUID) (*DocumentStats, error)
	GetCollaborationMetrics(ctx context.Context, documentID uuid.UUID, period time.Duration) (*models.CollaborationMetrics, error)

	// Maintenance
	CompactOperations(ctx context.Context, documentID uuid.UUID, beforeSeq int64) error
	PurgeOldSnapshots(ctx context.Context, documentID uuid.UUID, keepLast int) error
	ValidateDocumentIntegrity(ctx context.Context, documentID uuid.UUID) error
}
