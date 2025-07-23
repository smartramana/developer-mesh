package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/collaboration"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository/interfaces"
)

// DocumentService manages collaborative documents with conflict resolution
type DocumentService interface {
	// Document lifecycle
	Create(ctx context.Context, doc *models.SharedDocument) error
	Get(ctx context.Context, id uuid.UUID) (*models.SharedDocument, error)
	Update(ctx context.Context, doc *models.SharedDocument) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Collaborative editing
	ApplyOperation(ctx context.Context, docID uuid.UUID, operation *collaboration.DocumentOperation) error
	GetOperations(ctx context.Context, docID uuid.UUID, since time.Time) ([]*collaboration.DocumentOperation, error)
	GetOperationsBySequence(ctx context.Context, docID uuid.UUID, fromSeq, toSeq int64) ([]*collaboration.DocumentOperation, error)

	// Locking
	AcquireLock(ctx context.Context, docID uuid.UUID, agentID string, duration time.Duration) error
	ReleaseLock(ctx context.Context, docID uuid.UUID, agentID string) error
	ExtendLock(ctx context.Context, docID uuid.UUID, agentID string, duration time.Duration) error
	GetLockInfo(ctx context.Context, docID uuid.UUID) (*models.DocumentLock, error)

	// Conflict resolution
	DetectConflicts(ctx context.Context, docID uuid.UUID) ([]*models.ConflictInfo, error)
	ResolveConflict(ctx context.Context, conflictID uuid.UUID, resolution interface{}) error
	GetConflictHistory(ctx context.Context, docID uuid.UUID) ([]*models.ConflictResolution, error)

	// Version management
	CreateSnapshot(ctx context.Context, docID uuid.UUID) (*models.DocumentSnapshot, error)
	GetSnapshot(ctx context.Context, docID uuid.UUID, version int64) (*models.DocumentSnapshot, error)
	ListSnapshots(ctx context.Context, docID uuid.UUID) ([]*models.DocumentSnapshot, error)
	RestoreSnapshot(ctx context.Context, docID uuid.UUID, version int64) error

	// Search and query
	SearchDocuments(ctx context.Context, query string, filters interfaces.DocumentFilters) ([]*models.SharedDocument, error)
	GetDocumentsByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*models.SharedDocument, error)
	GetDocumentsByCreator(ctx context.Context, createdBy string) ([]*models.SharedDocument, error)

	// Analytics
	GetDocumentStats(ctx context.Context, docID uuid.UUID) (*interfaces.DocumentStats, error)
	GetCollaborationMetrics(ctx context.Context, docID uuid.UUID, period time.Duration) (*models.CollaborationMetrics, error)

	// Real-time updates
	SubscribeToChanges(ctx context.Context, docID uuid.UUID, handler func(operation *collaboration.DocumentOperation)) (unsubscribe func())
	BroadcastChange(ctx context.Context, docID uuid.UUID, operation *collaboration.DocumentOperation) error
}
