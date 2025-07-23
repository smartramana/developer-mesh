package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/repository/types"
)

// documentRepository implements DocumentRepository with production features
type documentRepository struct {
	writeDB *sqlx.DB
	readDB  *sqlx.DB
	cache   cache.Cache
	logger  observability.Logger
	tracer  observability.StartSpanFunc
}

// NewDocumentRepository creates a production-ready document repository
func NewDocumentRepository(
	writeDB, readDB *sqlx.DB,
	cache cache.Cache,
	logger observability.Logger,
	tracer observability.StartSpanFunc,
) interfaces.DocumentRepository {
	return &documentRepository{
		writeDB: writeDB,
		readDB:  readDB,
		cache:   cache,
		logger:  logger,
		tracer:  tracer,
	}
}

// WithTx returns a repository instance that uses the provided transaction
func (r *documentRepository) WithTx(tx types.Transaction) interfaces.DocumentRepository {
	// For now, just return the same repository instance
	// In a real implementation, we would wrap the transaction
	return r
}

// BeginTx starts a new transaction with options
func (r *documentRepository) BeginTx(ctx context.Context, opts *types.TxOptions) (types.Transaction, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.BeginTx")
	defer span.End()

	var txOpts *sql.TxOptions
	if opts != nil {
		txOpts = &sql.TxOptions{
			Isolation: sql.IsolationLevel(opts.Isolation),
			ReadOnly:  opts.ReadOnly,
		}
	}

	tx, err := r.writeDB.BeginTxx(ctx, txOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to begin transaction")
	}

	return &pgTransaction{tx: tx, logger: r.logger, startTime: time.Now()}, nil
}

// Create creates a new shared document
func (r *documentRepository) Create(ctx context.Context, doc *models.SharedDocument) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.Create")
	defer span.End()

	if doc.ID == uuid.Nil {
		doc.ID = uuid.New()
	}
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now()
	}
	doc.UpdatedAt = time.Now()
	if doc.Version == 0 {
		doc.Version = 1
	}

	query := `
		INSERT INTO shared_documents (
			id, workspace_id, tenant_id, type, title, content, content_type,
			version, created_by, metadata, locked_by, locked_at, lock_expires_at,
			created_at, updated_at
		) VALUES (
			:id, :workspace_id, :tenant_id, :type, :title, :content, :content_type,
			:version, :created_by, :metadata, :locked_by, :locked_at, :lock_expires_at,
			:created_at, :updated_at
		)
	`

	_, err := r.writeDB.NamedExecContext(ctx, query, doc)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			return errors.Wrap(err, "document already exists")
		}
		return errors.Wrap(err, "failed to create document")
	}

	// Invalidate cache for workspace documents
	cacheKey := fmt.Sprintf("documents:workspace:%s", doc.WorkspaceID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
			"key":   cacheKey,
			"error": err.Error(),
		})
	}

	return nil
}

// Get retrieves a document by ID
func (r *documentRepository) Get(ctx context.Context, id uuid.UUID) (*models.SharedDocument, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.Get")
	defer span.End()

	// Check cache first
	cacheKey := fmt.Sprintf("document:%s", id)
	var doc models.SharedDocument
	if err := r.cache.Get(ctx, cacheKey, &doc); err == nil {
		return &doc, nil
	}

	query := `
		SELECT id, workspace_id, tenant_id, type, title, content, content_type,
		       version, created_by, metadata, locked_by, locked_at, lock_expires_at,
		       created_at, updated_at, deleted_at
		FROM shared_documents
		WHERE id = $1 AND deleted_at IS NULL
	`

	err := r.readDB.GetContext(ctx, &doc, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("document not found")
		}
		return nil, errors.Wrap(err, "failed to get document")
	}

	// Cache the result
	if err := r.cache.Set(ctx, cacheKey, &doc, 5*time.Minute); err != nil {
		r.logger.Warn("Failed to cache document", map[string]interface{}{
			"id":    id,
			"error": err.Error(),
		})
	}

	return &doc, nil
}

// GetForUpdate retrieves a document with a lock for update
func (r *documentRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (*models.SharedDocument, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetForUpdate")
	defer span.End()

	query := `
		SELECT id, workspace_id, tenant_id, type, title, content, content_type,
		       version, created_by, metadata, locked_by, locked_at, lock_expires_at,
		       created_at, updated_at, deleted_at
		FROM shared_documents
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`

	var doc models.SharedDocument
	err := r.writeDB.GetContext(ctx, &doc, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("document not found")
		}
		return nil, errors.Wrap(err, "failed to get document for update")
	}

	return &doc, nil
}

// Update updates a document
func (r *documentRepository) Update(ctx context.Context, doc *models.SharedDocument) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.Update")
	defer span.End()

	doc.UpdatedAt = time.Now()
	doc.Version++

	query := `
		UPDATE shared_documents
		SET workspace_id = :workspace_id,
		    type = :type,
		    title = :title,
		    content = :content,
		    content_type = :content_type,
		    version = :version,
		    metadata = :metadata,
		    locked_by = :locked_by,
		    locked_at = :locked_at,
		    lock_expires_at = :lock_expires_at,
		    updated_at = :updated_at
		WHERE id = :id AND deleted_at IS NULL
	`

	result, err := r.writeDB.NamedExecContext(ctx, query, doc)
	if err != nil {
		return errors.Wrap(err, "failed to update document")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("document not found or already deleted")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("document:%s", doc.ID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
			"key":   cacheKey,
			"error": err.Error(),
		})
	}

	return nil
}

// UpdateWithVersion updates a document with optimistic locking
func (r *documentRepository) UpdateWithVersion(ctx context.Context, doc *models.SharedDocument, expectedVersion int64) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.UpdateWithVersion")
	defer span.End()

	doc.UpdatedAt = time.Now()
	doc.Version = expectedVersion + 1

	query := `
		UPDATE shared_documents
		SET workspace_id = $2,
		    type = $3,
		    title = $4,
		    content = $5,
		    content_type = $6,
		    version = $7,
		    metadata = $8,
		    locked_by = $9,
		    locked_at = $10,
		    lock_expires_at = $11,
		    updated_at = $12
		WHERE id = $1 AND version = $13 AND deleted_at IS NULL
	`

	result, err := r.writeDB.ExecContext(ctx, query,
		doc.ID, doc.WorkspaceID, doc.Type, doc.Title, doc.Content,
		doc.ContentType, doc.Version, doc.Metadata, doc.LockedBy,
		doc.LockedAt, doc.LockExpiresAt, doc.UpdatedAt, expectedVersion)
	if err != nil {
		return errors.Wrap(err, "failed to update document")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("version conflict or document not found")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("document:%s", doc.ID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
			"key":   cacheKey,
			"error": err.Error(),
		})
	}

	return nil
}

// Delete deletes a document
func (r *documentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.Delete")
	defer span.End()

	query := `DELETE FROM shared_documents WHERE id = $1`

	result, err := r.writeDB.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, "failed to delete document")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("document not found")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("document:%s", id)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
			"key":   cacheKey,
			"error": err.Error(),
		})
	}

	return nil
}

// GetByWorkspace retrieves all documents in a workspace
func (r *documentRepository) GetByWorkspace(ctx context.Context, workspaceID uuid.UUID, filters types.DocumentFilters) ([]*models.SharedDocument, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetByWorkspace")
	defer span.End()

	// Note: This method uses types.DocumentFilters which is different from interfaces.DocumentFilters
	// Convert or handle appropriately
	query := `
		SELECT id, workspace_id, tenant_id, type, title, content, content_type,
		       version, created_by, metadata, locked_by, locked_at, lock_expires_at,
		       created_at, updated_at, deleted_at
		FROM shared_documents
		WHERE workspace_id = $1 AND deleted_at IS NULL
		ORDER BY updated_at DESC
	`

	var documents []*models.SharedDocument
	err := r.readDB.SelectContext(ctx, &documents, query, workspaceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get documents by workspace")
	}

	return documents, nil
}

// List retrieves documents based on filters
func (r *documentRepository) List(ctx context.Context, tenantID uuid.UUID, filters interfaces.DocumentFilters) ([]*models.SharedDocument, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.List")
	defer span.End()

	// Build dynamic query
	query := `
		SELECT id, workspace_id, tenant_id, type, title, content, content_type,
		       version, created_by, metadata, locked_by, locked_at, lock_expires_at,
		       created_at, updated_at, deleted_at
		FROM shared_documents
		WHERE tenant_id = $1 AND deleted_at IS NULL
	`

	args := []interface{}{tenantID}
	argCount := 1

	// Apply filters
	if filters.WorkspaceID != nil {
		argCount++
		query += fmt.Sprintf(" AND workspace_id = $%d", argCount)
		args = append(args, *filters.WorkspaceID)
	}

	if len(filters.Type) > 0 {
		argCount++
		query += fmt.Sprintf(" AND type = ANY($%d)", argCount)
		args = append(args, pq.Array(filters.Type))
	}

	if filters.CreatedBy != nil {
		argCount++
		query += fmt.Sprintf(" AND created_by = $%d", argCount)
		args = append(args, *filters.CreatedBy)
	}

	if filters.CreatedAfter != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, *filters.CreatedAfter)
	}

	if filters.CreatedBefore != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, *filters.CreatedBefore)
	}

	if filters.UpdatedAfter != nil {
		argCount++
		query += fmt.Sprintf(" AND updated_at >= $%d", argCount)
		args = append(args, *filters.UpdatedAfter)
	}

	if len(filters.ContentType) > 0 {
		argCount++
		query += fmt.Sprintf(" AND content_type = ANY($%d)", argCount)
		args = append(args, pq.Array(filters.ContentType))
	}

	// Add sorting
	if filters.SortBy != "" {
		orderDir := "ASC"
		if filters.SortOrder == types.SortDesc {
			orderDir = "DESC"
		}
		query += fmt.Sprintf(" ORDER BY %s %s", filters.SortBy, orderDir)
	} else {
		query += " ORDER BY updated_at DESC"
	}

	// Add pagination
	if filters.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filters.Limit)
	}

	if filters.Offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filters.Offset)
	}

	var documents []*models.SharedDocument
	err := r.readDB.SelectContext(ctx, &documents, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list documents")
	}

	return documents, nil
}

// ListByWorkspace retrieves documents in a workspace
func (r *documentRepository) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*models.SharedDocument, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.ListByWorkspace")
	defer span.End()

	// Check cache first
	cacheKey := fmt.Sprintf("documents:workspace:%s", workspaceID)
	var documents []*models.SharedDocument
	if err := r.cache.Get(ctx, cacheKey, &documents); err == nil {
		return documents, nil
	}

	query := `
		SELECT id, workspace_id, tenant_id, type, title, content, content_type,
		       version, created_by, metadata, locked_by, locked_at, lock_expires_at,
		       created_at, updated_at, deleted_at
		FROM shared_documents
		WHERE workspace_id = $1 AND deleted_at IS NULL
		ORDER BY updated_at DESC
	`

	err := r.readDB.SelectContext(ctx, &documents, query, workspaceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list documents by workspace")
	}

	// Cache the results
	if err := r.cache.Set(ctx, cacheKey, documents, 5*time.Minute); err != nil {
		r.logger.Warn("Failed to cache workspace documents", map[string]interface{}{
			"workspace_id": workspaceID,
			"error":        err.Error(),
		})
	}

	return documents, nil
}

// ListByCreator retrieves documents created by an agent
func (r *documentRepository) ListByCreator(ctx context.Context, createdBy string) ([]*models.SharedDocument, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.ListByCreator")
	defer span.End()

	query := `
		SELECT id, workspace_id, tenant_id, type, title, content, content_type,
		       version, created_by, metadata, locked_by, locked_at, lock_expires_at,
		       created_at, updated_at, deleted_at
		FROM shared_documents
		WHERE created_by = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	var documents []*models.SharedDocument
	err := r.readDB.SelectContext(ctx, &documents, query, createdBy)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list documents by creator")
	}

	return documents, nil
}

// SearchDocuments searches documents by query
func (r *documentRepository) SearchDocuments(ctx context.Context, query string, filters interfaces.DocumentFilters) ([]*models.SharedDocument, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.SearchDocuments")
	defer span.End()

	// Use PostgreSQL full-text search
	searchQuery := `
		SELECT id, workspace_id, tenant_id, type, title, content, content_type,
		       version, created_by, metadata, locked_by, locked_at, lock_expires_at,
		       created_at, updated_at, deleted_at,
		       ts_rank(to_tsvector('english', title || ' ' || content), plainto_tsquery('english', $1)) AS rank
		FROM shared_documents
		WHERE deleted_at IS NULL
		  AND to_tsvector('english', title || ' ' || content) @@ plainto_tsquery('english', $1)
	`

	args := []interface{}{query}
	argCount := 1

	// Apply filters
	if filters.WorkspaceID != nil {
		argCount++
		searchQuery += fmt.Sprintf(" AND workspace_id = $%d", argCount)
		args = append(args, *filters.WorkspaceID)
	}

	if len(filters.Type) > 0 {
		argCount++
		searchQuery += fmt.Sprintf(" AND type = ANY($%d)", argCount)
		args = append(args, pq.Array(filters.Type))
	}

	searchQuery += " ORDER BY rank DESC"

	if filters.Limit > 0 {
		argCount++
		searchQuery += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filters.Limit)
	}

	var documents []*models.SharedDocument
	err := r.readDB.SelectContext(ctx, &documents, searchQuery, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to search documents")
	}

	return documents, nil
}

// SoftDelete soft deletes a document
func (r *documentRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.SoftDelete")
	defer span.End()

	now := time.Now()
	query := `
		UPDATE shared_documents
		SET deleted_at = $2, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.writeDB.ExecContext(ctx, query, id, now, now)
	if err != nil {
		return errors.Wrap(err, "failed to soft delete document")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("document not found or already deleted")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("document:%s", id)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
			"key":   cacheKey,
			"error": err.Error(),
		})
	}

	return nil
}

// UpdateWithLock updates a document with lock verification
func (r *documentRepository) UpdateWithLock(ctx context.Context, document *models.SharedDocument, lockOwner string) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.UpdateWithLock")
	defer span.End()

	document.UpdatedAt = time.Now()
	document.Version++

	query := `
		UPDATE shared_documents
		SET workspace_id = :workspace_id,
		    type = :type,
		    title = :title,
		    content = :content,
		    content_type = :content_type,
		    version = :version,
		    metadata = :metadata,
		    updated_at = :updated_at
		WHERE id = :id 
		  AND deleted_at IS NULL
		  AND locked_by = :lock_owner
		  AND lock_expires_at > NOW()
	`

	// Create a map with document fields and lock owner
	params := map[string]interface{}{
		"id":           document.ID,
		"workspace_id": document.WorkspaceID,
		"type":         document.Type,
		"title":        document.Title,
		"content":      document.Content,
		"content_type": document.ContentType,
		"version":      document.Version,
		"metadata":     document.Metadata,
		"updated_at":   document.UpdatedAt,
		"lock_owner":   lockOwner,
	}

	result, err := r.writeDB.NamedExecContext(ctx, query, params)
	if err != nil {
		return errors.Wrap(err, "failed to update document with lock")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("document not found, not locked by owner, or lock expired")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("document:%s", document.ID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
			"key":   cacheKey,
			"error": err.Error(),
		})
	}

	return nil
}

// AcquireLock acquires an exclusive lock on a document
func (r *documentRepository) AcquireLock(ctx context.Context, docID uuid.UUID, agentID string, duration time.Duration) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.AcquireLock")
	defer span.End()

	now := time.Now()
	expiresAt := now.Add(duration)

	// Try to acquire lock only if not locked or lock expired
	query := `
		UPDATE shared_documents
		SET locked_by = $2,
		    locked_at = $3,
		    lock_expires_at = $4,
		    updated_at = $5
		WHERE id = $1
		  AND deleted_at IS NULL
		  AND (locked_by IS NULL OR lock_expires_at < NOW())
	`

	result, err := r.writeDB.ExecContext(ctx, query, docID, agentID, now, expiresAt, now)
	if err != nil {
		return errors.Wrap(err, "failed to acquire lock")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("document not found or already locked")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("document:%s", docID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
			"key":   cacheKey,
			"error": err.Error(),
		})
	}

	return nil
}

// ReleaseLock releases a lock on a document
func (r *documentRepository) ReleaseLock(ctx context.Context, docID uuid.UUID, agentID string) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.ReleaseLock")
	defer span.End()

	query := `
		UPDATE shared_documents
		SET locked_by = NULL,
		    locked_at = NULL,
		    lock_expires_at = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND locked_by = $2 AND deleted_at IS NULL
	`

	result, err := r.writeDB.ExecContext(ctx, query, docID, agentID)
	if err != nil {
		return errors.Wrap(err, "failed to release lock")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("document not found or not locked by agent")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("document:%s", docID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
			"key":   cacheKey,
			"error": err.Error(),
		})
	}

	return nil
}

// ExtendLock extends an existing lock on a document
func (r *documentRepository) ExtendLock(ctx context.Context, docID uuid.UUID, agentID string, duration time.Duration) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.ExtendLock")
	defer span.End()

	newExpiresAt := time.Now().Add(duration)

	query := `
		UPDATE shared_documents
		SET lock_expires_at = $3,
		    updated_at = NOW()
		WHERE id = $1 
		  AND locked_by = $2 
		  AND lock_expires_at > NOW()
		  AND deleted_at IS NULL
	`

	result, err := r.writeDB.ExecContext(ctx, query, docID, agentID, newExpiresAt)
	if err != nil {
		return errors.Wrap(err, "failed to extend lock")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("document not found, not locked by agent, or lock expired")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("document:%s", docID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
			"key":   cacheKey,
			"error": err.Error(),
		})
	}

	return nil
}

// GetLockInfo retrieves lock information for a document
func (r *documentRepository) GetLockInfo(ctx context.Context, docID uuid.UUID) (*models.DocumentLock, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetLockInfo")
	defer span.End()

	query := `
		SELECT locked_by, locked_at, lock_expires_at
		FROM shared_documents
		WHERE id = $1 AND deleted_at IS NULL
	`

	var lockedBy sql.NullString
	var lockedAt, lockExpiresAt sql.NullTime

	err := r.readDB.QueryRowContext(ctx, query, docID).Scan(&lockedBy, &lockedAt, &lockExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("document not found")
		}
		return nil, errors.Wrap(err, "failed to get lock info")
	}

	if !lockedBy.Valid || !lockedAt.Valid || !lockExpiresAt.Valid {
		return nil, nil // No lock
	}

	return &models.DocumentLock{
		DocumentID:    docID,
		LockedBy:      lockedBy.String,
		LockedAt:      lockedAt.Time,
		LockExpiresAt: lockExpiresAt.Time,
		LockType:      "exclusive",
	}, nil
}

// ForceReleaseLock forces the release of a document lock
func (r *documentRepository) ForceReleaseLock(ctx context.Context, documentID uuid.UUID) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.ForceReleaseLock")
	defer span.End()

	query := `
		UPDATE shared_documents
		SET locked_by = NULL,
		    locked_at = NULL,
		    lock_expires_at = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.writeDB.ExecContext(ctx, query, documentID)
	if err != nil {
		return errors.Wrap(err, "failed to force release lock")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("document not found")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("document:%s", documentID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
			"key":   cacheKey,
			"error": err.Error(),
		})
	}

	return nil
}

// RecordOperation records a document operation
func (r *documentRepository) RecordOperation(ctx context.Context, operation *models.DocumentOperation) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.RecordOperation")
	defer span.End()

	if operation.ID == uuid.Nil {
		operation.ID = uuid.New()
	}
	if operation.Timestamp.IsZero() {
		operation.Timestamp = time.Now()
	}

	query := `
		INSERT INTO document_operations (
			id, document_id, tenant_id, agent_id, operation_type, operation_data,
			vector_clock, sequence_number, timestamp, parent_operation_id, is_applied
		) VALUES (
			:id, :document_id, :tenant_id, :agent_id, :operation_type, :operation_data,
			:vector_clock, :sequence_number, :timestamp, :parent_operation_id, :is_applied
		)
	`

	_, err := r.writeDB.NamedExecContext(ctx, query, operation)
	if err != nil {
		return errors.Wrap(err, "failed to record operation")
	}

	return nil
}

// GetOperations retrieves operations for a document
func (r *documentRepository) GetOperations(ctx context.Context, docID uuid.UUID, since time.Time) ([]*models.DocumentOperation, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetOperations")
	defer span.End()

	query := `
		SELECT id, document_id, tenant_id, agent_id, operation_type, operation_data,
		       vector_clock, sequence_number, timestamp, parent_operation_id, is_applied
		FROM document_operations
		WHERE document_id = $1 AND timestamp >= $2
		ORDER BY sequence_number ASC
	`

	var operations []*models.DocumentOperation
	err := r.readDB.SelectContext(ctx, &operations, query, docID, since)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get operations")
	}

	return operations, nil
}

// GetOperationsBySequence retrieves operations by sequence number range
func (r *documentRepository) GetOperationsBySequence(ctx context.Context, docID uuid.UUID, fromSeq, toSeq int64) ([]*models.DocumentOperation, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetOperationsBySequence")
	defer span.End()

	query := `
		SELECT id, document_id, tenant_id, agent_id, operation_type, operation_data,
		       vector_clock, sequence_number, timestamp, parent_operation_id, is_applied
		FROM document_operations
		WHERE document_id = $1 AND sequence_number >= $2 AND sequence_number <= $3
		ORDER BY sequence_number ASC
	`

	var operations []*models.DocumentOperation
	err := r.readDB.SelectContext(ctx, &operations, query, docID, fromSeq, toSeq)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get operations by sequence")
	}

	return operations, nil
}

// GetPendingOperations retrieves pending operations for a document
func (r *documentRepository) GetPendingOperations(ctx context.Context, documentID uuid.UUID) ([]*models.DocumentOperation, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetPendingOperations")
	defer span.End()

	query := `
		SELECT id, document_id, tenant_id, agent_id, operation_type, operation_data,
		       vector_clock, sequence_number, timestamp, parent_operation_id, is_applied
		FROM document_operations
		WHERE document_id = $1 AND is_applied = false
		ORDER BY sequence_number ASC
	`

	var operations []*models.DocumentOperation
	err := r.readDB.SelectContext(ctx, &operations, query, documentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pending operations")
	}

	return operations, nil
}

// MarkOperationApplied marks an operation as applied
func (r *documentRepository) MarkOperationApplied(ctx context.Context, operationID uuid.UUID) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.MarkOperationApplied")
	defer span.End()

	query := `
		UPDATE document_operations
		SET is_applied = true
		WHERE id = $1
	`

	result, err := r.writeDB.ExecContext(ctx, query, operationID)
	if err != nil {
		return errors.Wrap(err, "failed to mark operation as applied")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("operation not found")
	}

	return nil
}

// CreateSnapshot creates a snapshot of a document
func (r *documentRepository) CreateSnapshot(ctx context.Context, documentID uuid.UUID, version int64) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.CreateSnapshot")
	defer span.End()

	// Get current document state
	doc, err := r.Get(ctx, documentID)
	if err != nil {
		return errors.Wrap(err, "failed to get document for snapshot")
	}

	// Get latest vector clock from operations
	var vectorClock models.JSONMap
	clockQuery := `
		SELECT vector_clock
		FROM document_operations
		WHERE document_id = $1 AND is_applied = true
		ORDER BY sequence_number DESC
		LIMIT 1
	`
	err = r.readDB.GetContext(ctx, &vectorClock, clockQuery, documentID)
	if err != nil && err != sql.ErrNoRows {
		return errors.Wrap(err, "failed to get vector clock")
	}

	snapshotID := uuid.New()
	query := `
		INSERT INTO document_snapshots (
			id, document_id, version, content, vector_clock, created_at, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
	`

	_, err = r.writeDB.ExecContext(ctx, query,
		snapshotID, documentID, version, doc.Content, vectorClock, time.Now(), doc.CreatedBy)
	if err != nil {
		return errors.Wrap(err, "failed to create snapshot")
	}

	return nil
}

// GetLatestSnapshot retrieves the latest snapshot of a document
func (r *documentRepository) GetLatestSnapshot(ctx context.Context, docID uuid.UUID) (*models.DocumentSnapshot, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetLatestSnapshot")
	defer span.End()

	query := `
		SELECT id, document_id, version, content, vector_clock, created_at, created_by
		FROM document_snapshots
		WHERE document_id = $1
		ORDER BY version DESC
		LIMIT 1
	`

	var snapshot models.DocumentSnapshot
	err := r.readDB.GetContext(ctx, &snapshot, query, docID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("no snapshots found")
		}
		return nil, errors.Wrap(err, "failed to get latest snapshot")
	}

	return &snapshot, nil
}

// GetSnapshotBeforeTime retrieves the snapshot before a specific time
func (r *documentRepository) GetSnapshotBeforeTime(ctx context.Context, docID uuid.UUID, before time.Time) (*models.DocumentSnapshot, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetSnapshotBeforeTime")
	defer span.End()

	query := `
		SELECT id, document_id, version, content, vector_clock, created_at, created_by
		FROM document_snapshots
		WHERE document_id = $1 AND created_at < $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	var snapshot models.DocumentSnapshot
	err := r.readDB.GetContext(ctx, &snapshot, query, docID, before)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("no snapshots found before specified time")
		}
		return nil, errors.Wrap(err, "failed to get snapshot before time")
	}

	return &snapshot, nil
}

// GetSnapshot retrieves a specific snapshot version
func (r *documentRepository) GetSnapshot(ctx context.Context, documentID uuid.UUID, version int64) (*models.DocumentSnapshot, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetSnapshot")
	defer span.End()

	query := `
		SELECT id, document_id, version, content, vector_clock, created_at, created_by
		FROM document_snapshots
		WHERE document_id = $1 AND version = $2
	`

	var snapshot models.DocumentSnapshot
	err := r.readDB.GetContext(ctx, &snapshot, query, documentID, version)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("snapshot not found")
		}
		return nil, errors.Wrap(err, "failed to get snapshot")
	}

	return &snapshot, nil
}

// ListSnapshots lists all snapshots for a document
func (r *documentRepository) ListSnapshots(ctx context.Context, documentID uuid.UUID) ([]*models.DocumentSnapshot, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.ListSnapshots")
	defer span.End()

	query := `
		SELECT id, document_id, version, content, vector_clock, created_at, created_by
		FROM document_snapshots
		WHERE document_id = $1
		ORDER BY version DESC
	`

	var snapshots []*models.DocumentSnapshot
	err := r.readDB.SelectContext(ctx, &snapshots, query, documentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list snapshots")
	}

	return snapshots, nil
}

// RecordConflict records a conflict resolution
func (r *documentRepository) RecordConflict(ctx context.Context, conflict *models.ConflictResolution) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.RecordConflict")
	defer span.End()

	if conflict.ID == uuid.Nil {
		conflict.ID = uuid.New()
	}
	if conflict.CreatedAt.IsZero() {
		conflict.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO conflict_resolutions (
			id, tenant_id, resource_type, resource_id, conflict_type, description,
			resolution_strategy, details, resolved_by, resolved_at, created_at
		) VALUES (
			:id, :tenant_id, :resource_type, :resource_id, :conflict_type, :description,
			:resolution_strategy, :details, :resolved_by, :resolved_at, :created_at
		)
	`

	_, err := r.writeDB.NamedExecContext(ctx, query, conflict)
	if err != nil {
		return errors.Wrap(err, "failed to record conflict")
	}

	return nil
}

// GetConflicts retrieves all conflicts for a document
func (r *documentRepository) GetConflicts(ctx context.Context, docID uuid.UUID) ([]*models.ConflictResolution, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetConflicts")
	defer span.End()

	query := `
		SELECT id, tenant_id, resource_type, resource_id, conflict_type, description,
		       resolution_strategy, details, resolved_by, resolved_at, created_at
		FROM conflict_resolutions
		WHERE resource_id = $1 AND resource_type = 'document'
		ORDER BY created_at DESC
	`

	var conflicts []*models.ConflictResolution
	err := r.readDB.SelectContext(ctx, &conflicts, query, docID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get conflicts")
	}

	return conflicts, nil
}

// GetUnresolvedConflicts retrieves unresolved conflicts for a document
func (r *documentRepository) GetUnresolvedConflicts(ctx context.Context, documentID uuid.UUID) ([]*models.ConflictResolution, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetUnresolvedConflicts")
	defer span.End()

	query := `
		SELECT id, tenant_id, resource_type, resource_id, conflict_type, description,
		       resolution_strategy, details, resolved_by, resolved_at, created_at
		FROM conflict_resolutions
		WHERE resource_id = $1 AND resource_type = 'document' AND resolved_at IS NULL
		ORDER BY created_at ASC
	`

	var conflicts []*models.ConflictResolution
	err := r.readDB.SelectContext(ctx, &conflicts, query, documentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get unresolved conflicts")
	}

	return conflicts, nil
}

// ResolveConflict marks a conflict as resolved
func (r *documentRepository) ResolveConflict(ctx context.Context, conflictID uuid.UUID, resolution map[string]interface{}) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.ResolveConflict")
	defer span.End()

	// Get the agent ID from the resolution map, if provided
	var resolvedBy *string
	if agentID, ok := resolution["resolved_by"].(string); ok {
		resolvedBy = &agentID
		delete(resolution, "resolved_by") // Remove from details
	}

	now := time.Now()
	query := `
		UPDATE conflict_resolutions
		SET details = details || $2,
		    resolved_by = $3,
		    resolved_at = $4
		WHERE id = $1 AND resolved_at IS NULL
	`

	result, err := r.writeDB.ExecContext(ctx, query, conflictID, resolution, resolvedBy, now)
	if err != nil {
		return errors.Wrap(err, "failed to resolve conflict")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("conflict not found or already resolved")
	}

	return nil
}

// GetCollaborators retrieves current collaborators of a document
func (r *documentRepository) GetCollaborators(ctx context.Context, docID uuid.UUID) ([]string, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetCollaborators")
	defer span.End()

	// Get unique agent IDs who have operations on this document in the last 24 hours
	query := `
		SELECT DISTINCT agent_id
		FROM document_operations
		WHERE document_id = $1 AND timestamp > NOW() - INTERVAL '24 hours'
		ORDER BY agent_id
	`

	var collaborators []string
	err := r.readDB.SelectContext(ctx, &collaborators, query, docID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collaborators")
	}

	return collaborators, nil
}

// UpdateCollaborators updates the list of document collaborators
func (r *documentRepository) UpdateCollaborators(ctx context.Context, docID uuid.UUID, collaborators []string) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.UpdateCollaborators")
	defer span.End()

	// This is typically managed through the document_operations table
	// But we can store a denormalized list in the documents table for quick access
	// For now, we'll update a collaborators array in the metadata field

	query := `
		UPDATE shared_documents
		SET metadata = jsonb_set(
			COALESCE(metadata, '{}')::jsonb,
			'{collaborators}',
			$2::jsonb
		),
		updated_at = NOW()
		WHERE id = $1
	`

	collaboratorsJSON, err := json.Marshal(collaborators)
	if err != nil {
		return errors.Wrap(err, "failed to marshal collaborators")
	}

	_, err = r.writeDB.ExecContext(ctx, query, docID, string(collaboratorsJSON))
	if err != nil {
		return errors.Wrap(err, "failed to update collaborators")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("document:%s", docID)
	if err := r.cache.Delete(ctx, cacheKey); err != nil {
		r.logger.Warn("Failed to invalidate cache", map[string]interface{}{
			"key":   cacheKey,
			"error": err.Error(),
		})
	}

	return nil
}

// GetDocumentStats retrieves document statistics
func (r *documentRepository) GetDocumentStats(ctx context.Context, documentID uuid.UUID) (*interfaces.DocumentStats, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetDocumentStats")
	defer span.End()

	var stats interfaces.DocumentStats

	// Get operation counts
	opQuery := `
		SELECT 
			COUNT(*) as total_operations,
			COUNT(DISTINCT agent_id) as unique_collaborators,
			MAX(timestamp) as last_modified_at
		FROM document_operations
		WHERE document_id = $1
	`

	err := r.readDB.QueryRowContext(ctx, opQuery, documentID).Scan(
		&stats.TotalOperations,
		&stats.UniqueCollaborators,
		&stats.LastModifiedAt,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to get operation stats")
	}

	// Get conflict count
	conflictQuery := `
		SELECT COUNT(*)
		FROM conflict_resolutions
		WHERE resource_id = $1 AND resource_type = 'document'
	`
	err = r.readDB.QueryRowContext(ctx, conflictQuery, documentID).Scan(&stats.ConflictCount)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to get conflict count")
	}

	// Get document size and version count
	docQuery := `
		SELECT 
			LENGTH(content) as size_bytes,
			version as version_count
		FROM shared_documents
		WHERE id = $1
	`
	err = r.readDB.QueryRowContext(ctx, docQuery, documentID).Scan(
		&stats.SizeBytes,
		&stats.VersionCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("document not found")
		}
		return nil, errors.Wrap(err, "failed to get document stats")
	}

	return &stats, nil
}

// GetCollaborationMetrics retrieves collaboration metrics for a document
func (r *documentRepository) GetCollaborationMetrics(ctx context.Context, docID uuid.UUID, period time.Duration) (*models.CollaborationMetrics, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.GetCollaborationMetrics")
	defer span.End()

	since := time.Now().Add(-period)
	metrics := &models.CollaborationMetrics{
		DocumentID:       docID,
		Period:           period,
		OperationsByType: make(map[string]int64),
	}

	// Get unique collaborators and total operations
	baseQuery := `
		SELECT 
			COUNT(DISTINCT agent_id) as unique_collaborators,
			COUNT(*) as total_operations
		FROM document_operations
		WHERE document_id = $1 AND timestamp >= $2
	`
	err := r.readDB.QueryRowContext(ctx, baseQuery, docID, since).Scan(
		&metrics.UniqueCollaborators,
		&metrics.TotalOperations,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to get base metrics")
	}

	// Get operations by type
	typeQuery := `
		SELECT operation_type, COUNT(*) as count
		FROM document_operations
		WHERE document_id = $1 AND timestamp >= $2
		GROUP BY operation_type
	`
	rows, err := r.readDB.QueryContext(ctx, typeQuery, docID, since)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get operations by type")
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var opType string
		var count int64
		if err := rows.Scan(&opType, &count); err != nil {
			return nil, errors.Wrap(err, "failed to scan operation type")
		}
		metrics.OperationsByType[opType] = count
	}

	// Get conflict count
	conflictQuery := `
		SELECT COUNT(*)
		FROM conflict_resolutions
		WHERE resource_id = $1 AND resource_type = 'document' AND created_at >= $2
	`
	err = r.readDB.QueryRowContext(ctx, conflictQuery, docID, since).Scan(&metrics.ConflictCount)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to get conflict count")
	}

	// Calculate average response time (simplified - time between operations)
	respQuery := `
		SELECT AVG(time_diff) as avg_response_time
		FROM (
			SELECT EXTRACT(EPOCH FROM (timestamp - LAG(timestamp) OVER (ORDER BY timestamp))) as time_diff
			FROM document_operations
			WHERE document_id = $1 AND timestamp >= $2
		) t
		WHERE time_diff IS NOT NULL
	`
	var avgSeconds sql.NullFloat64
	err = r.readDB.QueryRowContext(ctx, respQuery, docID, since).Scan(&avgSeconds)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to get average response time")
	}
	if avgSeconds.Valid {
		metrics.AverageResponseTime = time.Duration(avgSeconds.Float64 * float64(time.Second))
	}

	// Get peak concurrency (max collaborators in any hour)
	concurrencyQuery := `
		SELECT MAX(concurrent_users) as peak_concurrency
		FROM (
			SELECT COUNT(DISTINCT agent_id) as concurrent_users
			FROM document_operations
			WHERE document_id = $1 AND timestamp >= $2
			GROUP BY DATE_TRUNC('hour', timestamp)
		) t
	`
	err = r.readDB.QueryRowContext(ctx, concurrencyQuery, docID, since).Scan(&metrics.PeakConcurrency)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "failed to get peak concurrency")
	}

	return metrics, nil
}

// CleanupOldOperations removes old operations while preserving snapshots
func (r *documentRepository) CleanupOldOperations(ctx context.Context, before time.Time) (int64, error) {
	ctx, span := r.tracer(ctx, "DocumentRepository.CleanupOldOperations")
	defer span.End()

	// Delete operations that are:
	// 1. Older than the specified time
	// 2. Already applied
	// 3. Have a snapshot created after them
	query := `
		DELETE FROM document_operations
		WHERE timestamp < $1 
		  AND is_applied = true
		  AND EXISTS (
			  SELECT 1 FROM document_snapshots
			  WHERE document_snapshots.document_id = document_operations.document_id
			    AND document_snapshots.created_at > document_operations.timestamp
		  )
	`

	result, err := r.writeDB.ExecContext(ctx, query, before)
	if err != nil {
		return 0, errors.Wrap(err, "failed to cleanup old operations")
	}

	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected")
	}

	r.logger.Info("Cleaned up old operations", map[string]interface{}{
		"rows_deleted": rowsDeleted,
		"before":       before,
	})

	return rowsDeleted, nil
}

// VacuumSnapshots consolidates snapshots for storage efficiency
func (r *documentRepository) VacuumSnapshots(ctx context.Context, docID uuid.UUID) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.VacuumSnapshots")
	defer span.End()

	// Keep snapshots using exponential backoff strategy:
	// - All snapshots from last 24 hours
	// - One per day for last week
	// - One per week for last month
	// - One per month for older

	// First, mark snapshots to keep
	keepQuery := `
		WITH snapshots_to_keep AS (
			-- Keep all from last 24 hours
			SELECT id FROM document_snapshots
			WHERE document_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
			UNION
			-- Keep one per day for last week
			SELECT DISTINCT ON (DATE(created_at)) id
			FROM document_snapshots
			WHERE document_id = $1 
			  AND created_at > NOW() - INTERVAL '7 days'
			  AND created_at <= NOW() - INTERVAL '24 hours'
			ORDER BY DATE(created_at), version DESC
			UNION
			-- Keep one per week for last month
			SELECT DISTINCT ON (DATE_TRUNC('week', created_at)) id
			FROM document_snapshots
			WHERE document_id = $1
			  AND created_at > NOW() - INTERVAL '30 days'
			  AND created_at <= NOW() - INTERVAL '7 days'
			ORDER BY DATE_TRUNC('week', created_at), version DESC
			UNION
			-- Keep one per month for older
			SELECT DISTINCT ON (DATE_TRUNC('month', created_at)) id
			FROM document_snapshots
			WHERE document_id = $1
			  AND created_at <= NOW() - INTERVAL '30 days'
			ORDER BY DATE_TRUNC('month', created_at), version DESC
			UNION
			-- Always keep the latest snapshot
			SELECT id FROM document_snapshots
			WHERE document_id = $1
			ORDER BY version DESC
			LIMIT 1
		)
		DELETE FROM document_snapshots
		WHERE document_id = $1
		  AND id NOT IN (SELECT id FROM snapshots_to_keep)
	`

	result, err := r.writeDB.ExecContext(ctx, keepQuery, docID)
	if err != nil {
		return errors.Wrap(err, "failed to vacuum snapshots")
	}

	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	r.logger.Info("Vacuumed snapshots", map[string]interface{}{
		"document_id":  docID,
		"rows_deleted": rowsDeleted,
	})

	return nil
}

// CompactOperations compacts operations before a sequence number
func (r *documentRepository) CompactOperations(ctx context.Context, documentID uuid.UUID, beforeSeq int64) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.CompactOperations")
	defer span.End()

	// Start a transaction for atomicity
	tx, err := r.BeginTx(ctx, &types.TxOptions{
		Isolation: types.IsolationSerializable,
	})
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	// Create a snapshot at the specified sequence number if one doesn't exist
	snapshotQuery := `
		INSERT INTO document_snapshots (id, document_id, version, content, vector_clock, created_at, created_by)
		SELECT 
			$1, d.id, $2, d.content, 
			(SELECT vector_clock FROM document_operations WHERE document_id = $3 AND sequence_number = $2),
			NOW(), d.created_by
		FROM shared_documents d
		WHERE d.id = $3
		  AND NOT EXISTS (
			  SELECT 1 FROM document_snapshots
			  WHERE document_id = $3 AND version = $2
		  )
	`

	snapshotID := uuid.New()
	_, err = r.writeDB.ExecContext(ctx, snapshotQuery, snapshotID, beforeSeq, documentID)
	if err != nil {
		return errors.Wrap(err, "failed to create compaction snapshot")
	}

	// Delete operations before the sequence number
	deleteQuery := `
		DELETE FROM document_operations
		WHERE document_id = $1 AND sequence_number < $2 AND is_applied = true
	`

	result, err := r.writeDB.ExecContext(ctx, deleteQuery, documentID, beforeSeq)
	if err != nil {
		return errors.Wrap(err, "failed to delete compacted operations")
	}

	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	r.logger.Info("Compacted operations", map[string]interface{}{
		"document_id":  documentID,
		"before_seq":   beforeSeq,
		"rows_deleted": rowsDeleted,
	})

	return nil
}

// PurgeOldSnapshots removes old snapshots keeping the last N
func (r *documentRepository) PurgeOldSnapshots(ctx context.Context, documentID uuid.UUID, keepLast int) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.PurgeOldSnapshots")
	defer span.End()

	if keepLast < 1 {
		return errors.New("keepLast must be at least 1")
	}

	// Delete all but the last N snapshots
	query := `
		WITH snapshots_to_keep AS (
			SELECT id
			FROM document_snapshots
			WHERE document_id = $1
			ORDER BY version DESC
			LIMIT $2
		)
		DELETE FROM document_snapshots
		WHERE document_id = $1
		  AND id NOT IN (SELECT id FROM snapshots_to_keep)
	`

	result, err := r.writeDB.ExecContext(ctx, query, documentID, keepLast)
	if err != nil {
		return errors.Wrap(err, "failed to purge old snapshots")
	}

	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	r.logger.Info("Purged old snapshots", map[string]interface{}{
		"document_id":  documentID,
		"keep_last":    keepLast,
		"rows_deleted": rowsDeleted,
	})

	return nil
}

// ValidateDocumentIntegrity validates document data integrity
func (r *documentRepository) ValidateDocumentIntegrity(ctx context.Context, documentID uuid.UUID) error {
	ctx, span := r.tracer(ctx, "DocumentRepository.ValidateDocumentIntegrity")
	defer span.End()

	// Check 1: Document exists and is not deleted
	var docExists bool
	docQuery := `SELECT EXISTS(SELECT 1 FROM shared_documents WHERE id = $1 AND deleted_at IS NULL)`
	err := r.readDB.QueryRowContext(ctx, docQuery, documentID).Scan(&docExists)
	if err != nil {
		return errors.Wrap(err, "failed to check document existence")
	}
	if !docExists {
		return errors.New("document not found or deleted")
	}

	// Check 2: Verify operation sequence numbers are continuous
	seqQuery := `
		WITH seq_check AS (
			SELECT 
				sequence_number,
				LAG(sequence_number) OVER (ORDER BY sequence_number) as prev_seq
			FROM document_operations
			WHERE document_id = $1
		)
		SELECT COUNT(*)
		FROM seq_check
		WHERE sequence_number - prev_seq > 1
	`

	var gaps int
	err = r.readDB.QueryRowContext(ctx, seqQuery, documentID).Scan(&gaps)
	if err != nil {
		return errors.Wrap(err, "failed to check sequence gaps")
	}
	if gaps > 0 {
		return errors.New("operation sequence has gaps")
	}

	// Check 3: Verify all applied operations have valid vector clocks
	clockQuery := `
		SELECT COUNT(*)
		FROM document_operations
		WHERE document_id = $1 
		  AND is_applied = true
		  AND (vector_clock IS NULL OR vector_clock = '{}'::jsonb)
	`

	var invalidClocks int
	err = r.readDB.QueryRowContext(ctx, clockQuery, documentID).Scan(&invalidClocks)
	if err != nil {
		return errors.Wrap(err, "failed to check vector clocks")
	}
	if invalidClocks > 0 {
		return errors.New("found applied operations with invalid vector clocks")
	}

	// Check 4: Verify snapshot consistency
	snapshotQuery := `
		SELECT COUNT(*)
		FROM document_snapshots s1
		JOIN document_snapshots s2 ON s1.document_id = s2.document_id
		WHERE s1.document_id = $1
		  AND s1.version > s2.version
		  AND s1.created_at < s2.created_at
	`

	var inconsistentSnapshots int
	err = r.readDB.QueryRowContext(ctx, snapshotQuery, documentID).Scan(&inconsistentSnapshots)
	if err != nil {
		return errors.Wrap(err, "failed to check snapshot consistency")
	}
	if inconsistentSnapshots > 0 {
		return errors.New("snapshot versions are inconsistent with creation times")
	}

	// Check 5: Verify lock consistency
	var lockExpired bool
	lockQuery := `
		SELECT locked_by IS NOT NULL AND lock_expires_at < NOW()
		FROM shared_documents
		WHERE id = $1
	`
	err = r.readDB.QueryRowContext(ctx, lockQuery, documentID).Scan(&lockExpired)
	if err != nil {
		return errors.Wrap(err, "failed to check lock status")
	}
	if lockExpired {
		return errors.New("document has expired lock")
	}

	r.logger.Info("Document integrity validated", map[string]interface{}{
		"document_id": documentID,
	})

	return nil
}
