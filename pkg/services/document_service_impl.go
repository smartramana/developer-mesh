package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/developer-mesh/developer-mesh/pkg/collaboration"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/events"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository/interfaces"
)

// documentService implements DocumentService with production features
type documentService struct {
	BaseService

	// Repositories
	documentRepo interfaces.DocumentRepository

	// Caching
	cache         cache.Cache
	cachePrefix   string
	cacheDuration time.Duration

	// Locking
	lockService DocumentLockService
	locks       sync.Map // documentID -> *documentLock (deprecated, use lockService)
	lockTimeout time.Duration

	// Metrics
	metricsPrefix string

	// Event publishing
	eventPublisher events.EventPublisher

	// Real-time subscriptions
	subscriptions sync.Map // documentID -> map[subscriptionID]*subscription
}

type documentLock struct {
	agentID   string
	expiresAt time.Time
	lockType  string // "exclusive" or "shared"
	mu        sync.RWMutex
}

type subscription struct {
	id      string
	handler func(operation *collaboration.DocumentOperation)
}

// NewDocumentService creates a production-grade document service
func NewDocumentService(
	config ServiceConfig,
	documentRepo interfaces.DocumentRepository,
	cache cache.Cache,
) DocumentService {
	return &documentService{
		BaseService:   NewBaseService(config),
		documentRepo:  documentRepo,
		cache:         cache,
		cachePrefix:   "document:",
		cacheDuration: 5 * time.Minute,
		lockTimeout:   15 * time.Minute,
		metricsPrefix: "document_service",
	}
}

// NewDocumentServiceWithLocks creates a document service with distributed locking
func NewDocumentServiceWithLocks(
	config ServiceConfig,
	documentRepo interfaces.DocumentRepository,
	cache cache.Cache,
	lockService DocumentLockService,
) DocumentService {
	return &documentService{
		BaseService:   NewBaseService(config),
		documentRepo:  documentRepo,
		cache:         cache,
		lockService:   lockService,
		cachePrefix:   "document:",
		cacheDuration: 5 * time.Minute,
		lockTimeout:   15 * time.Minute,
		metricsPrefix: "document_service",
	}
}

// Create creates a document with validation and versioning
func (s *documentService) Create(ctx context.Context, document *models.SharedDocument) error {
	ctx, span := s.config.Tracer(ctx, "DocumentService.Create")
	defer span.End()

	startTime := time.Now()
	defer func() {
		s.config.Metrics.RecordHistogram(
			fmt.Sprintf("%s.create.duration", s.metricsPrefix),
			time.Since(startTime).Seconds(),
			nil,
		)
	}()

	// Check rate limit
	if err := s.CheckRateLimit(ctx, "document:create"); err != nil {
		return err
	}

	// Check quota
	if err := s.CheckQuota(ctx, "documents", 1); err != nil {
		return err
	}

	// Validate document
	if err := s.validateDocument(document); err != nil {
		return errors.Wrap(err, "document validation failed")
	}

	// Set defaults
	document.ID = uuid.New()
	document.Version = 1
	document.CreatedAt = time.Now()
	document.UpdatedAt = time.Now()

	if document.Metadata == nil {
		document.Metadata = make(models.JSONMap)
	}
	document.Metadata["version_history"] = []interface{}{
		map[string]interface{}{
			"version":    1,
			"created_at": time.Now(),
			"created_by": document.CreatedBy,
			"size":       len(document.Content),
		},
	}

	// Create with transaction
	err := s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Create document
		if err := s.documentRepo.Create(ctx, document); err != nil {
			return err
		}

		// Create initial snapshot
		if err := s.documentRepo.CreateSnapshot(ctx, document.ID, 1); err != nil {
			return err
		}

		// Quota is already consumed by CheckQuota call above

		return nil
	})

	if err != nil {
		s.config.Metrics.IncrementCounter(fmt.Sprintf("%s.create.error", s.metricsPrefix), 1)
		return err
	}

	// Publish event
	if s.eventPublisher != nil {
		event := &events.DomainEvent{
			ID:        uuid.New(),
			Type:      "document.created",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"document_id":  document.ID,
				"workspace_id": document.WorkspaceID,
				"created_by":   document.CreatedBy,
				"version":      1,
			},
		}
		if err := s.eventPublisher.Publish(ctx, event); err != nil {
			s.config.Logger.Warn("Failed to publish document created event", map[string]interface{}{
				"document_id": document.ID,
				"error":       err.Error(),
			})
		}
	}

	s.config.Metrics.IncrementCounter(fmt.Sprintf("%s.create.success", s.metricsPrefix), 1)
	return nil
}

// Get retrieves a document with caching
func (s *documentService) Get(ctx context.Context, id uuid.UUID) (*models.SharedDocument, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.Get")
	defer span.End()

	// Check cache
	cacheKey := s.cachePrefix + id.String()
	var cached models.SharedDocument
	if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
		s.config.Metrics.IncrementCounter(fmt.Sprintf("%s.cache.hit", s.metricsPrefix), 1)
		return &cached, nil
	}

	s.config.Metrics.IncrementCounter(fmt.Sprintf("%s.cache.miss", s.metricsPrefix), 1)

	// Get from repository
	document, err := s.documentRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cache result
	_ = s.cache.Set(ctx, cacheKey, document, s.cacheDuration)

	return document, nil
}

// Update updates a document with versioning and conflict detection
func (s *documentService) Update(ctx context.Context, document *models.SharedDocument) error {
	ctx, span := s.config.Tracer(ctx, "DocumentService.Update")
	defer span.End()

	// Check rate limit
	if err := s.CheckRateLimit(ctx, "document:update"); err != nil {
		return err
	}

	// Validate document
	if err := s.validateDocument(document); err != nil {
		return errors.Wrap(err, "document validation failed")
	}

	// Check if locked
	lock, err := s.GetLockInfo(ctx, document.ID)
	if err != nil {
		return err
	}
	if lock != nil && lock.LockedBy != document.CreatedBy {
		return fmt.Errorf("document is locked by agent %s", lock.LockedBy)
	}

	// Update with transaction
	err = s.WithTransaction(ctx, func(ctx context.Context, tx Transaction) error {
		// Get current document for conflict detection
		current, err := s.documentRepo.Get(ctx, document.ID)
		if err != nil {
			return err
		}

		// Check version for optimistic locking
		if current.Version != document.Version {
			return ErrConcurrentModification
		}

		// Increment version
		document.Version++
		document.UpdatedAt = time.Now()

		// Update version history in metadata
		if document.Metadata == nil {
			document.Metadata = make(models.JSONMap)
		}

		history, _ := document.Metadata["version_history"].([]interface{})
		history = append(history, map[string]interface{}{
			"version":    document.Version,
			"created_at": time.Now(),
			"created_by": document.CreatedBy,
			"size":       len(document.Content),
			"changes":    s.calculateChanges(current.Content, document.Content),
		})

		// Keep only last 100 versions in metadata
		if len(history) > 100 {
			history = history[len(history)-100:]
		}
		document.Metadata["version_history"] = history

		// Update document
		if err := s.documentRepo.Update(ctx, document); err != nil {
			return err
		}

		// Create version snapshot
		if err := s.documentRepo.CreateSnapshot(ctx, document.ID, document.Version); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		s.config.Metrics.IncrementCounter(fmt.Sprintf("%s.update.error", s.metricsPrefix), 1)
		return err
	}

	// Invalidate cache
	cacheKey := s.cachePrefix + document.ID.String()
	_ = s.cache.Delete(ctx, cacheKey)

	// Publish event
	if s.eventPublisher != nil {
		event := &events.DomainEvent{
			ID:        uuid.New(),
			Type:      "document.updated",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"document_id":  document.ID,
				"workspace_id": document.WorkspaceID,
				"updated_by":   document.CreatedBy,
				"version":      document.Version,
			},
		}
		_ = s.eventPublisher.Publish(ctx, event)
	}

	s.config.Metrics.IncrementCounter(fmt.Sprintf("%s.update.success", s.metricsPrefix), 1)
	return nil
}

// Delete soft deletes a document
func (s *documentService) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := s.config.Tracer(ctx, "DocumentService.Delete")
	defer span.End()

	// Check rate limit
	if err := s.CheckRateLimit(ctx, "document:delete"); err != nil {
		return err
	}

	// Delete from repository
	if err := s.documentRepo.Delete(ctx, id); err != nil {
		return err
	}

	// Invalidate cache
	cacheKey := s.cachePrefix + id.String()
	_ = s.cache.Delete(ctx, cacheKey)

	// Clean up locks
	s.locks.Delete(id)

	// Clean up subscriptions
	s.subscriptions.Delete(id)

	return nil
}

// ApplyOperation applies a CRDT operation to a document
func (s *documentService) ApplyOperation(ctx context.Context, docID uuid.UUID, operation *collaboration.DocumentOperation) error {
	ctx, span := s.config.Tracer(ctx, "DocumentService.ApplyOperation")
	defer span.End()

	// Validate operation
	if operation.DocumentID != docID {
		return fmt.Errorf("operation document ID mismatch")
	}

	// Convert collaboration.DocumentOperation to models.DocumentOperation
	modelOp := &models.DocumentOperation{
		ID:             operation.ID,
		DocumentID:     operation.DocumentID,
		AgentID:        operation.AgentID,
		OperationType:  string(operation.Type),
		OperationData:  models.JSONMap{"value": operation.Value, "path": operation.Path},
		VectorClock:    models.JSONMap{},
		SequenceNumber: operation.Sequence,
		Timestamp:      operation.AppliedAt,
		IsApplied:      false,
	}

	// Copy vector clock
	for k, v := range operation.VectorClock {
		modelOp.VectorClock[k] = v
	}

	// Record operation
	if err := s.documentRepo.RecordOperation(ctx, modelOp); err != nil {
		return err
	}

	// Broadcast to subscribers
	return s.BroadcastChange(ctx, docID, operation)
}

// GetOperations retrieves operations since a given time
func (s *documentService) GetOperations(ctx context.Context, docID uuid.UUID, since time.Time) ([]*collaboration.DocumentOperation, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.GetOperations")
	defer span.End()

	// Get model operations
	modelOps, err := s.documentRepo.GetOperations(ctx, docID, since)
	if err != nil {
		return nil, err
	}

	// Convert to collaboration operations
	result := make([]*collaboration.DocumentOperation, len(modelOps))
	for i, op := range modelOps {
		result[i] = s.convertToCollaborationOp(op)
	}

	return result, nil
}

// GetOperationsBySequence retrieves operations by sequence range
func (s *documentService) GetOperationsBySequence(ctx context.Context, docID uuid.UUID, fromSeq, toSeq int64) ([]*collaboration.DocumentOperation, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.GetOperationsBySequence")
	defer span.End()

	// Get model operations
	modelOps, err := s.documentRepo.GetOperationsBySequence(ctx, docID, fromSeq, toSeq)
	if err != nil {
		return nil, err
	}

	// Convert to collaboration operations
	result := make([]*collaboration.DocumentOperation, len(modelOps))
	for i, op := range modelOps {
		result[i] = s.convertToCollaborationOp(op)
	}

	return result, nil
}

// AcquireLock acquires a lock on a document
func (s *documentService) AcquireLock(ctx context.Context, docID uuid.UUID, agentID string, duration time.Duration) error {
	ctx, span := s.config.Tracer(ctx, "DocumentService.AcquireLock")
	defer span.End()

	// Use distributed lock service if available
	if s.lockService != nil {
		lock, err := s.lockService.LockDocument(ctx, docID, agentID, duration)
		if err != nil {
			return errors.Wrap(err, "failed to acquire distributed lock")
		}

		// Update document metadata to reflect lock
		doc, err := s.documentRepo.Get(ctx, docID)
		if err != nil {
			// Rollback lock
			_ = s.lockService.UnlockDocument(ctx, docID, agentID)
			return err
		}

		doc.LockedBy = &agentID
		doc.LockedAt = &lock.AcquiredAt
		doc.LockExpiresAt = &lock.ExpiresAt

		if err := s.documentRepo.Update(ctx, doc); err != nil {
			// Rollback lock
			_ = s.lockService.UnlockDocument(ctx, docID, agentID)
			return err
		}

		return nil
	}

	// Fallback to local locking (deprecated)
	if duration > s.lockTimeout {
		duration = s.lockTimeout
	}

	// Check if already locked
	if val, exists := s.locks.Load(docID); exists {
		lock := val.(*documentLock)
		lock.mu.RLock()
		defer lock.mu.RUnlock()

		if time.Now().Before(lock.expiresAt) {
			if lock.agentID == agentID {
				// Extend lock
				lock.expiresAt = time.Now().Add(duration)
				return nil
			}
			return fmt.Errorf("document already locked by agent %s", lock.agentID)
		}
	}

	// Create new lock
	lock := &documentLock{
		agentID:   agentID,
		expiresAt: time.Now().Add(duration),
		lockType:  "exclusive",
	}

	s.locks.Store(docID, lock)

	// Update document lock fields
	doc, err := s.documentRepo.Get(ctx, docID)
	if err != nil {
		return err
	}

	now := time.Now()
	doc.LockedBy = &agentID
	doc.LockedAt = &now
	doc.LockExpiresAt = &lock.expiresAt

	if err := s.documentRepo.Update(ctx, doc); err != nil {
		s.locks.Delete(docID)
		return err
	}

	s.config.Metrics.IncrementCounter(fmt.Sprintf("%s.lock.acquired", s.metricsPrefix), 1)
	return nil
}

// ReleaseLock releases a lock on a document
func (s *documentService) ReleaseLock(ctx context.Context, docID uuid.UUID, agentID string) error {
	ctx, span := s.config.Tracer(ctx, "DocumentService.ReleaseLock")
	defer span.End()

	// Use distributed lock service if available
	if s.lockService != nil {
		if err := s.lockService.UnlockDocument(ctx, docID, agentID); err != nil {
			return errors.Wrap(err, "failed to release distributed lock")
		}

		// Update document metadata
		doc, err := s.documentRepo.Get(ctx, docID)
		if err != nil {
			return err
		}

		doc.LockedBy = nil
		doc.LockedAt = nil
		doc.LockExpiresAt = nil

		if err := s.documentRepo.Update(ctx, doc); err != nil {
			return err
		}

		return nil
	}

	// Fallback to local locking (deprecated)
	// Check lock ownership
	if val, exists := s.locks.Load(docID); exists {
		lock := val.(*documentLock)
		lock.mu.RLock()

		if lock.agentID != agentID {
			lock.mu.RUnlock()
			return fmt.Errorf("lock not owned by agent %s", agentID)
		}
		lock.mu.RUnlock()
	}

	// Remove lock
	s.locks.Delete(docID)

	// Update document
	doc, err := s.documentRepo.Get(ctx, docID)
	if err != nil {
		return err
	}

	doc.LockedBy = nil
	doc.LockedAt = nil
	doc.LockExpiresAt = nil

	if err := s.documentRepo.Update(ctx, doc); err != nil {
		return err
	}

	s.config.Metrics.IncrementCounter(fmt.Sprintf("%s.lock.released", s.metricsPrefix), 1)
	return nil
}

// ExtendLock extends a lock on a document
func (s *documentService) ExtendLock(ctx context.Context, docID uuid.UUID, agentID string, duration time.Duration) error {
	ctx, span := s.config.Tracer(ctx, "DocumentService.ExtendLock")
	defer span.End()

	// Use distributed lock service if available
	if s.lockService != nil {
		if err := s.lockService.ExtendLock(ctx, docID, agentID, duration); err != nil {
			return errors.Wrap(err, "failed to extend distributed lock")
		}

		// Update document metadata
		doc, err := s.documentRepo.Get(ctx, docID)
		if err != nil {
			return err
		}

		newExpiry := time.Now().Add(duration)
		doc.LockExpiresAt = &newExpiry

		if err := s.documentRepo.Update(ctx, doc); err != nil {
			return err
		}

		return nil
	}

	if duration > s.lockTimeout {
		duration = s.lockTimeout
	}

	// Check lock ownership
	if val, exists := s.locks.Load(docID); exists {
		lock := val.(*documentLock)
		lock.mu.Lock()
		defer lock.mu.Unlock()

		if lock.agentID != agentID {
			return fmt.Errorf("lock not owned by agent %s", agentID)
		}

		// Extend lock
		lock.expiresAt = time.Now().Add(duration)

		// Update document
		doc, err := s.documentRepo.Get(ctx, docID)
		if err != nil {
			return err
		}

		doc.LockExpiresAt = &lock.expiresAt

		if err := s.documentRepo.Update(ctx, doc); err != nil {
			return err
		}

		return nil
	}

	return fmt.Errorf("no lock found for document %s", docID)
}

// GetLockInfo retrieves lock information for a document
func (s *documentService) GetLockInfo(ctx context.Context, docID uuid.UUID) (*models.DocumentLock, error) {
	_, span := s.config.Tracer(ctx, "DocumentService.GetLockInfo")
	defer span.End()

	if val, exists := s.locks.Load(docID); exists {
		lock := val.(*documentLock)
		lock.mu.RLock()
		defer lock.mu.RUnlock()

		if time.Now().Before(lock.expiresAt) {
			return &models.DocumentLock{
				DocumentID:    docID,
				LockedBy:      lock.agentID,
				LockedAt:      lock.expiresAt.Add(-s.lockTimeout),
				LockExpiresAt: lock.expiresAt,
				LockType:      lock.lockType,
			}, nil
		}
	}

	return nil, nil
}

// DetectConflicts detects conflicts in document operations
func (s *documentService) DetectConflicts(ctx context.Context, docID uuid.UUID) ([]*models.ConflictInfo, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.DetectConflicts")
	defer span.End()

	// Get unresolved conflicts from repository
	conflicts, err := s.documentRepo.GetUnresolvedConflicts(ctx, docID)
	if err != nil {
		return nil, err
	}

	// Convert to ConflictInfo
	result := make([]*models.ConflictInfo, len(conflicts))
	for i, c := range conflicts {
		result[i] = &models.ConflictInfo{
			ID:         c.ID,
			DocumentID: c.ResourceID,
			Type:       c.ConflictType,
			DetectedAt: c.CreatedAt,
			Metadata:   c.Details,
		}
	}

	return result, nil
}

// ResolveConflict resolves a conflict
func (s *documentService) ResolveConflict(ctx context.Context, conflictID uuid.UUID, resolution interface{}) error {
	ctx, span := s.config.Tracer(ctx, "DocumentService.ResolveConflict")
	defer span.End()

	// Convert resolution to map[string]interface{}
	resolutionMap := make(map[string]interface{})
	if m, ok := resolution.(map[string]interface{}); ok {
		resolutionMap = m
	} else {
		resolutionMap["resolution"] = resolution
	}

	return s.documentRepo.ResolveConflict(ctx, conflictID, resolutionMap)
}

// GetConflictHistory retrieves conflict history for a document
func (s *documentService) GetConflictHistory(ctx context.Context, docID uuid.UUID) ([]*models.ConflictResolution, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.GetConflictHistory")
	defer span.End()

	return s.documentRepo.GetConflicts(ctx, docID)
}

// CreateSnapshot creates a snapshot of a document
func (s *documentService) CreateSnapshot(ctx context.Context, docID uuid.UUID) (*models.DocumentSnapshot, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.CreateSnapshot")
	defer span.End()

	doc, err := s.Get(ctx, docID)
	if err != nil {
		return nil, err
	}

	// Create snapshot in repository
	if err := s.documentRepo.CreateSnapshot(ctx, docID, doc.Version); err != nil {
		return nil, err
	}

	// Return the snapshot object
	snapshot := &models.DocumentSnapshot{
		ID:         uuid.New(),
		DocumentID: docID,
		Version:    doc.Version,
		Content:    doc.Content,
		CreatedBy:  doc.CreatedBy,
		CreatedAt:  time.Now(),
	}

	return snapshot, nil
}

// GetSnapshot retrieves a specific snapshot
func (s *documentService) GetSnapshot(ctx context.Context, docID uuid.UUID, version int64) (*models.DocumentSnapshot, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.GetSnapshot")
	defer span.End()

	return s.documentRepo.GetSnapshot(ctx, docID, version)
}

// ListSnapshots lists all snapshots for a document
func (s *documentService) ListSnapshots(ctx context.Context, docID uuid.UUID) ([]*models.DocumentSnapshot, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.ListSnapshots")
	defer span.End()

	return s.documentRepo.ListSnapshots(ctx, docID)
}

// RestoreSnapshot restores a document to a specific snapshot
func (s *documentService) RestoreSnapshot(ctx context.Context, docID uuid.UUID, version int64) error {
	ctx, span := s.config.Tracer(ctx, "DocumentService.RestoreSnapshot")
	defer span.End()

	snapshot, err := s.GetSnapshot(ctx, docID, version)
	if err != nil {
		return err
	}

	doc, err := s.Get(ctx, docID)
	if err != nil {
		return err
	}

	doc.Content = snapshot.Content
	doc.Version++

	return s.Update(ctx, doc)
}

// SearchDocuments searches documents
func (s *documentService) SearchDocuments(ctx context.Context, query string, filters interfaces.DocumentFilters) ([]*models.SharedDocument, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.SearchDocuments")
	defer span.End()

	return s.documentRepo.SearchDocuments(ctx, query, filters)
}

// GetDocumentsByWorkspace retrieves documents by workspace
func (s *documentService) GetDocumentsByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*models.SharedDocument, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.GetDocumentsByWorkspace")
	defer span.End()

	return s.documentRepo.ListByWorkspace(ctx, workspaceID)
}

// GetDocumentsByCreator retrieves documents by creator
func (s *documentService) GetDocumentsByCreator(ctx context.Context, createdBy string) ([]*models.SharedDocument, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.GetDocumentsByCreator")
	defer span.End()

	return s.documentRepo.ListByCreator(ctx, createdBy)
}

// GetDocumentStats retrieves document statistics
func (s *documentService) GetDocumentStats(ctx context.Context, docID uuid.UUID) (*interfaces.DocumentStats, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.GetDocumentStats")
	defer span.End()

	return s.documentRepo.GetDocumentStats(ctx, docID)
}

// GetCollaborationMetrics retrieves collaboration metrics
func (s *documentService) GetCollaborationMetrics(ctx context.Context, docID uuid.UUID, period time.Duration) (*models.CollaborationMetrics, error) {
	ctx, span := s.config.Tracer(ctx, "DocumentService.GetCollaborationMetrics")
	defer span.End()

	return s.documentRepo.GetCollaborationMetrics(ctx, docID, period)
}

// SubscribeToChanges subscribes to document changes
func (s *documentService) SubscribeToChanges(ctx context.Context, docID uuid.UUID, handler func(operation *collaboration.DocumentOperation)) (unsubscribe func()) {
	subID := uuid.New().String()
	sub := &subscription{
		id:      subID,
		handler: handler,
	}

	// Get or create subscription map for document
	subsMap, _ := s.subscriptions.LoadOrStore(docID, &sync.Map{})
	subs := subsMap.(*sync.Map)
	subs.Store(subID, sub)

	// Return unsubscribe function
	return func() {
		if subsMap, ok := s.subscriptions.Load(docID); ok {
			subs := subsMap.(*sync.Map)
			subs.Delete(subID)
		}
	}
}

// BroadcastChange broadcasts a change to subscribers
func (s *documentService) BroadcastChange(ctx context.Context, docID uuid.UUID, operation *collaboration.DocumentOperation) error {
	_, span := s.config.Tracer(ctx, "DocumentService.BroadcastChange")
	defer span.End()

	// Get subscribers for document
	if subsMap, ok := s.subscriptions.Load(docID); ok {
		subs := subsMap.(*sync.Map)

		// Notify each subscriber
		subs.Range(func(key, value interface{}) bool {
			sub := value.(*subscription)
			// Run handler in goroutine to prevent blocking
			go func() {
				defer func() {
					if r := recover(); r != nil {
						s.config.Logger.Error("Subscription handler panic", map[string]interface{}{
							"document_id": docID,
							"sub_id":      sub.id,
							"panic":       r,
						})
					}
				}()
				sub.handler(operation)
			}()
			return true
		})
	}

	return nil
}

// Helper methods

func (s *documentService) validateDocument(document *models.SharedDocument) error {
	if document.Title == "" {
		return fmt.Errorf("document title is required")
	}
	if document.WorkspaceID == uuid.Nil {
		return fmt.Errorf("workspace ID is required")
	}
	if document.CreatedBy == "" {
		return fmt.Errorf("created by is required")
	}
	if len(document.Content) > 10*1024*1024 { // 10MB limit
		return fmt.Errorf("document content exceeds maximum size of 10MB")
	}
	return nil
}

// hashContent removed - implement when content hashing is needed

func (s *documentService) calculateChanges(oldContent, newContent string) map[string]interface{} {
	// Simple change detection - use proper diff algorithm in production
	return map[string]interface{}{
		"additions": max(0, len(newContent)-len(oldContent)),
		"deletions": max(0, len(oldContent)-len(newContent)),
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// convertToCollaborationOp converts a model operation to collaboration operation
func (s *documentService) convertToCollaborationOp(op *models.DocumentOperation) *collaboration.DocumentOperation {
	result := &collaboration.DocumentOperation{
		ID:          op.ID,
		DocumentID:  op.DocumentID,
		Sequence:    op.SequenceNumber,
		Type:        op.OperationType,
		AgentID:     op.AgentID,
		VectorClock: make(map[string]int),
		AppliedAt:   op.Timestamp,
	}

	// Extract path and value from operation data
	if path, ok := op.OperationData["path"].(string); ok {
		result.Path = path
	}
	if value, ok := op.OperationData["value"]; ok {
		result.Value = value
	}

	// Copy vector clock
	for k, v := range op.VectorClock {
		if intVal, ok := v.(int); ok {
			result.VectorClock[k] = intVal
		} else if floatVal, ok := v.(float64); ok {
			result.VectorClock[k] = int(floatVal)
		}
	}

	// Copy dependencies if any
	if deps, ok := op.OperationData["dependencies"].([]interface{}); ok {
		result.Dependencies = make([]uuid.UUID, 0, len(deps))
		for _, dep := range deps {
			if depStr, ok := dep.(string); ok {
				if depID, err := uuid.Parse(depStr); err == nil {
					result.Dependencies = append(result.Dependencies, depID)
				}
			}
		}
	}

	return result
}
