package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
	"github.com/S-Corkum/devops-mcp/pkg/repository/types"
)

// workspaceRepository implements WorkspaceRepository with production features
type workspaceRepository struct {
	writeDB *sqlx.DB
	readDB  *sqlx.DB
	cache   cache.Cache
	logger  observability.Logger
	tracer  observability.StartSpanFunc
}

// NewWorkspaceRepository creates a production-ready workspace repository
func NewWorkspaceRepository(
	writeDB, readDB *sqlx.DB,
	cache cache.Cache,
	logger observability.Logger,
	tracer observability.StartSpanFunc,
) interfaces.WorkspaceRepository {
	return &workspaceRepository{
		writeDB: writeDB,
		readDB:  readDB,
		cache:   cache,
		logger:  logger,
		tracer:  tracer,
	}
}

// WithTx returns a repository instance that uses the provided transaction
func (r *workspaceRepository) WithTx(tx types.Transaction) interfaces.WorkspaceRepository {
	// For now, just return the same repository instance
	// In a real implementation, we would wrap the transaction
	return r
}

// BeginTx starts a new transaction with options
func (r *workspaceRepository) BeginTx(ctx context.Context, opts *types.TxOptions) (types.Transaction, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.BeginTx")
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

	// Wrap the sqlx.Tx to implement the Transaction interface
	return &transactionWrapper{tx: tx}, nil
}

// transactionWrapper wraps sqlx.Tx to implement types.Transaction
type transactionWrapper struct {
	tx *sqlx.Tx
}

func (t *transactionWrapper) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (t *transactionWrapper) Savepoint(ctx context.Context, name string) error {
	_, err := t.tx.ExecContext(ctx, "SAVEPOINT "+name)
	return err
}

func (t *transactionWrapper) RollbackToSavepoint(ctx context.Context, name string) error {
	_, err := t.tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+name)
	return err
}

func (t *transactionWrapper) Commit() error {
	return t.tx.Commit()
}

func (t *transactionWrapper) Rollback() error {
	return t.tx.Rollback()
}

// Create creates a new workspace
func (r *workspaceRepository) Create(ctx context.Context, workspace *models.Workspace) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.Create")
	defer span.End()

	query := `
		INSERT INTO workspaces (
			id, tenant_id, name, description, type, owner_id,
			is_active, is_public, settings, tags, metadata,
			created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15
		)
	`

	_, err := r.writeDB.ExecContext(ctx, query,
		workspace.ID,
		workspace.TenantID,
		workspace.Name,
		workspace.Description,
		workspace.Type,
		workspace.OwnerID,
		workspace.IsActive,
		workspace.IsPublic,
		workspace.Settings,
		pq.Array(workspace.Tags),
		workspace.Metadata,
		workspace.CreatedAt,
		workspace.UpdatedAt,
		workspace.OwnerID, // Use OwnerID for created_by
		workspace.OwnerID, // Use OwnerID for updated_by
	)

	if err != nil {
		r.logger.Error("Failed to create workspace", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": workspace.ID,
			"tenant_id":    workspace.TenantID,
		})
		return errors.Wrap(err, "failed to create workspace")
	}

	// Invalidate cache for tenant's workspace list
	cacheKey := fmt.Sprintf("workspaces:tenant:%s", workspace.TenantID)
	_ = r.cache.Delete(ctx, cacheKey)

	return nil
}

// Get retrieves a workspace by ID
func (r *workspaceRepository) Get(ctx context.Context, id uuid.UUID) (*models.Workspace, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.Get")
	defer span.End()

	// Check cache first
	cacheKey := fmt.Sprintf("workspace:%s", id)
	var cached models.Workspace
	if err := r.cache.Get(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	}

	query := `
		SELECT
			id, tenant_id, name, description, type, owner_id,
			is_active, is_public, settings, tags, metadata,
			created_at, updated_at, deleted_at, created_by, updated_by
		FROM workspaces
		WHERE id = $1
	`

	var workspace models.Workspace
	err := r.readDB.GetContext(ctx, &workspace, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("workspace not found")
		}
		return nil, errors.Wrap(err, "failed to get workspace")
	}

	// Cache for 5 minutes
	_ = r.cache.Set(ctx, cacheKey, &workspace, 5*time.Minute)

	return &workspace, nil
}

// Update updates a workspace
func (r *workspaceRepository) Update(ctx context.Context, workspace *models.Workspace) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.Update")
	defer span.End()

	query := `
		UPDATE workspaces SET
			name = $2,
			description = $3,
			type = $4,
			owner_id = $5,
			is_active = $6,
			is_public = $7,
			settings = $8,
			tags = $9,
			metadata = $10,
			updated_at = $11,
			updated_by = $12
		WHERE id = $1
	`

	_, err := r.writeDB.ExecContext(ctx, query,
		workspace.ID,
		workspace.Name,
		workspace.Description,
		workspace.Type,
		workspace.OwnerID,
		workspace.IsActive,
		workspace.IsPublic,
		workspace.Settings,
		pq.Array(workspace.Tags),
		workspace.Metadata,
		workspace.UpdatedAt,
		workspace.OwnerID, // Use OwnerID for updated_by
	)

	if err != nil {
		r.logger.Error("Failed to update workspace", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": workspace.ID,
		})
		return errors.Wrap(err, "failed to update workspace")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("workspace:%s", workspace.ID)
	_ = r.cache.Delete(ctx, cacheKey)

	return nil
}

// Delete deletes a workspace
func (r *workspaceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.Delete")
	defer span.End()

	query := `DELETE FROM workspaces WHERE id = $1`

	_, err := r.writeDB.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete workspace", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": id,
		})
		return errors.Wrap(err, "failed to delete workspace")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("workspace:%s", id)
	_ = r.cache.Delete(ctx, cacheKey)

	return nil
}

// GetByTenant retrieves workspaces for a specific tenant
func (r *workspaceRepository) GetByTenant(ctx context.Context, tenantID uuid.UUID) ([]*models.Workspace, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.GetByTenant")
	defer span.End()

	// Check cache first
	cacheKey := fmt.Sprintf("workspaces:tenant:%s", tenantID)
	var cached []*models.Workspace
	if err := r.cache.Get(ctx, cacheKey, &cached); err == nil {
		return cached, nil
	}

	query := `
		SELECT
			id, tenant_id, name, description, type, owner_id,
			is_active, is_public, settings, tags, metadata,
			created_at, updated_at, deleted_at, created_by, updated_by
		FROM workspaces
		WHERE tenant_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	var workspaces []*models.Workspace
	err := r.readDB.SelectContext(ctx, &workspaces, query, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get workspaces by tenant")
	}

	// Cache for 1 minute
	_ = r.cache.Set(ctx, cacheKey, workspaces, 1*time.Minute)

	return workspaces, nil
}

// GetByAgent retrieves workspaces for a specific agent
func (r *workspaceRepository) GetByAgent(ctx context.Context, agentID string) ([]*models.Workspace, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.GetByAgent")
	defer span.End()

	query := `
		SELECT DISTINCT w.*
		FROM workspaces w
		LEFT JOIN workspace_members wm ON w.id = wm.workspace_id
		WHERE (w.owner_id = $1 OR wm.agent_id = $1)
			AND w.deleted_at IS NULL
		ORDER BY w.created_at DESC
	`

	var workspaces []*models.Workspace
	err := r.readDB.SelectContext(ctx, &workspaces, query, agentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get workspaces by agent")
	}

	return workspaces, nil
}

// ListByTenant is an alias for GetByTenant for compatibility
func (r *workspaceRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]*models.Workspace, error) {
	return r.GetByTenant(ctx, tenantID)
}

// ListByAgent is an alias for GetByAgent for compatibility
func (r *workspaceRepository) ListByAgent(ctx context.Context, agentID string) ([]*models.Workspace, error) {
	return r.GetByAgent(ctx, agentID)
}

// List retrieves workspaces based on filters
func (r *workspaceRepository) List(ctx context.Context, tenantID uuid.UUID, filters interfaces.WorkspaceFilters) ([]*models.Workspace, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.List")
	defer span.End()

	query := `
		SELECT
			id, tenant_id, name, description, type, owner_id,
			is_active, is_public, settings, tags, metadata,
			created_at, updated_at, deleted_at, created_by, updated_by
		FROM workspaces
		WHERE tenant_id = $1 AND deleted_at IS NULL
	`

	args := []interface{}{tenantID}
	argCount := 1

	// Apply filters
	if filters.IsActive != nil {
		argCount++
		query += fmt.Sprintf(" AND is_active = $%d", argCount)
		args = append(args, *filters.IsActive)
	}

	if filters.OwnerID != nil {
		argCount++
		query += fmt.Sprintf(" AND owner_id = $%d", argCount)
		args = append(args, *filters.OwnerID)
	}

	if filters.CreatedAfter != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at > $%d", argCount)
		args = append(args, *filters.CreatedAfter)
	}

	if filters.CreatedBefore != nil {
		argCount++
		query += fmt.Sprintf(" AND created_at < $%d", argCount)
		args = append(args, *filters.CreatedBefore)
	}

	// Add sorting
	if filters.SortBy != "" {
		order := "ASC"
		if filters.SortOrder == "desc" {
			order = "DESC"
		}
		query += fmt.Sprintf(" ORDER BY %s %s", filters.SortBy, order)
	} else {
		query += " ORDER BY created_at DESC"
	}

	// Add pagination
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filters.Limit)
	}
	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filters.Offset)
	}

	var workspaces []*models.Workspace
	err := r.readDB.SelectContext(ctx, &workspaces, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list workspaces")
	}

	return workspaces, nil
}

// ListByMember retrieves workspaces for a specific member
func (r *workspaceRepository) ListByMember(ctx context.Context, agentID string) ([]*models.Workspace, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.ListByMember")
	defer span.End()

	query := `
		SELECT w.*
		FROM workspaces w
		INNER JOIN workspace_members wm ON w.id = wm.workspace_id
		WHERE wm.agent_id = $1 AND w.deleted_at IS NULL
		ORDER BY w.created_at DESC
	`

	var workspaces []*models.Workspace
	err := r.readDB.SelectContext(ctx, &workspaces, query, agentID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list workspaces by member")
	}

	return workspaces, nil
}

// ListByOwner retrieves workspaces owned by a specific agent
func (r *workspaceRepository) ListByOwner(ctx context.Context, ownerID string) ([]*models.Workspace, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.ListByOwner")
	defer span.End()

	query := `
		SELECT
			id, tenant_id, name, description, type, owner_id,
			is_active, is_public, settings, tags, metadata,
			created_at, updated_at, deleted_at, created_by, updated_by
		FROM workspaces
		WHERE owner_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	var workspaces []*models.Workspace
	err := r.readDB.SelectContext(ctx, &workspaces, query, ownerID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list workspaces by owner")
	}

	return workspaces, nil
}

// AddMember adds a member to a workspace
func (r *workspaceRepository) AddMember(ctx context.Context, member *models.WorkspaceMember) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.AddMember")
	defer span.End()

	query := `
		INSERT INTO workspace_members (
			workspace_id, agent_id, role, permissions,
			joined_at, last_active_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (workspace_id, agent_id) DO UPDATE SET
			role = EXCLUDED.role,
			permissions = EXCLUDED.permissions,
			last_active_at = EXCLUDED.last_active_at
	`

	_, err := r.writeDB.ExecContext(ctx, query,
		member.WorkspaceID,
		member.AgentID,
		member.Role,
		member.Permissions,
		member.JoinedAt,
		member.LastSeenAt,
	)

	if err != nil {
		r.logger.Error("Failed to add member", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": member.WorkspaceID,
			"agent_id":     member.AgentID,
		})
		return errors.Wrap(err, "failed to add member")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("workspace:members:%s", member.WorkspaceID)
	_ = r.cache.Delete(ctx, cacheKey)

	return nil
}

// RemoveMember removes a member from a workspace
func (r *workspaceRepository) RemoveMember(ctx context.Context, workspaceID uuid.UUID, agentID string) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.RemoveMember")
	defer span.End()

	query := `DELETE FROM workspace_members WHERE workspace_id = $1 AND agent_id = $2`

	_, err := r.writeDB.ExecContext(ctx, query, workspaceID, agentID)
	if err != nil {
		r.logger.Error("Failed to remove member", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": workspaceID,
			"agent_id":     agentID,
		})
		return errors.Wrap(err, "failed to remove member")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("workspace:members:%s", workspaceID)
	_ = r.cache.Delete(ctx, cacheKey)

	return nil
}

// UpdateMemberRole updates a member's role in a workspace
func (r *workspaceRepository) UpdateMemberRole(ctx context.Context, workspaceID uuid.UUID, agentID string, role string) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.UpdateMemberRole")
	defer span.End()

	query := `
		UPDATE workspace_members
		SET role = $3, last_seen_at = NOW()
		WHERE workspace_id = $1 AND agent_id = $2
	`

	_, err := r.writeDB.ExecContext(ctx, query, workspaceID, agentID, role)
	if err != nil {
		r.logger.Error("Failed to update member role", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": workspaceID,
			"agent_id":     agentID,
			"role":         role,
		})
		return errors.Wrap(err, "failed to update member role")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("workspace:members:%s", workspaceID)
	_ = r.cache.Delete(ctx, cacheKey)

	return nil
}

// GetMembers retrieves all members of a workspace
func (r *workspaceRepository) GetMembers(ctx context.Context, workspaceID uuid.UUID) ([]*models.WorkspaceMember, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.GetMembers")
	defer span.End()

	// Check cache first
	cacheKey := fmt.Sprintf("workspace:members:%s", workspaceID)
	var cached []*models.WorkspaceMember
	if err := r.cache.Get(ctx, cacheKey, &cached); err == nil {
		return cached, nil
	}

	query := `
		SELECT
			workspace_id, agent_id, tenant_id, role,
			permissions, joined_at, last_seen_at
		FROM workspace_members
		WHERE workspace_id = $1
		ORDER BY joined_at ASC
	`

	var members []*models.WorkspaceMember
	err := r.readDB.SelectContext(ctx, &members, query, workspaceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get workspace members")
	}

	// Cache for 1 minute
	_ = r.cache.Set(ctx, cacheKey, members, 1*time.Minute)

	return members, nil
}

// ListMembers retrieves all members of a workspace
func (r *workspaceRepository) ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]*models.WorkspaceMember, error) {
	// Delegate to GetMembers as they serve the same purpose
	return r.GetMembers(ctx, workspaceID)
}

// GetMember retrieves a specific member of a workspace
func (r *workspaceRepository) GetMember(ctx context.Context, workspaceID uuid.UUID, agentID string) (*models.WorkspaceMember, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.GetMember")
	defer span.End()

	query := `
		SELECT
			workspace_id, agent_id, tenant_id, role,
			permissions, joined_at, last_seen_at
		FROM workspace_members
		WHERE workspace_id = $1 AND agent_id = $2
	`

	var member models.WorkspaceMember
	err := r.readDB.GetContext(ctx, &member, query, workspaceID, agentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("member not found")
		}
		return nil, errors.Wrap(err, "failed to get member")
	}

	return &member, nil
}

// UpdateMemberActivity updates the last activity time for a member
func (r *workspaceRepository) UpdateMemberActivity(ctx context.Context, workspaceID uuid.UUID, agentID string) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.UpdateMemberActivity")
	defer span.End()

	query := `
		UPDATE workspace_members
		SET last_seen_at = NOW()
		WHERE workspace_id = $1 AND agent_id = $2
	`

	_, err := r.writeDB.ExecContext(ctx, query, workspaceID, agentID)
	if err != nil {
		r.logger.Error("Failed to update member activity", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": workspaceID,
			"agent_id":     agentID,
		})
		return errors.Wrap(err, "failed to update member activity")
	}

	return nil
}

// UpdateState updates workspace state
func (r *workspaceRepository) UpdateState(ctx context.Context, workspaceID uuid.UUID, state map[string]interface{}, version int64) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.UpdateState")
	defer span.End()

	query := `
		UPDATE workspaces
		SET state = $2, state_version = $3, updated_at = NOW()
		WHERE id = $1 AND state_version = $3 - 1
	`

	result, err := r.writeDB.ExecContext(ctx, query, workspaceID, state, version)
	if err != nil {
		r.logger.Error("Failed to update workspace state", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": workspaceID,
		})
		return errors.Wrap(err, "failed to update workspace state")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to check rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("concurrent update detected")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("workspace:%s", workspaceID)
	_ = r.cache.Delete(ctx, cacheKey)

	return nil
}

// LockWorkspace locks a workspace for exclusive access
func (r *workspaceRepository) LockWorkspace(ctx context.Context, workspaceID uuid.UUID, agentID string, duration time.Duration) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.LockWorkspace")
	defer span.End()

	lockExpiresAt := time.Now().Add(duration)

	query := `
		UPDATE workspaces
		SET 
			locked_by = $2,
			locked_at = NOW(),
			lock_expires_at = $3
		WHERE id = $1 AND (locked_by IS NULL OR lock_expires_at < NOW())
	`

	result, err := r.writeDB.ExecContext(ctx, query, workspaceID, agentID, lockExpiresAt)
	if err != nil {
		r.logger.Error("Failed to lock workspace", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": workspaceID,
			"agent_id":     agentID,
		})
		return errors.Wrap(err, "failed to lock workspace")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to check rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("workspace is already locked")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("workspace:%s", workspaceID)
	_ = r.cache.Delete(ctx, cacheKey)

	return nil
}

// UnlockWorkspace unlocks a workspace
func (r *workspaceRepository) UnlockWorkspace(ctx context.Context, workspaceID uuid.UUID, agentID string) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.UnlockWorkspace")
	defer span.End()

	query := `
		UPDATE workspaces
		SET 
			locked_by = NULL,
			locked_at = NULL,
			lock_expires_at = NULL
		WHERE id = $1 AND locked_by = $2
	`

	result, err := r.writeDB.ExecContext(ctx, query, workspaceID, agentID)
	if err != nil {
		r.logger.Error("Failed to unlock workspace", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": workspaceID,
			"agent_id":     agentID,
		})
		return errors.Wrap(err, "failed to unlock workspace")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to check rows affected")
	}

	if rowsAffected == 0 {
		return errors.New("workspace not locked by this agent")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("workspace:%s", workspaceID)
	_ = r.cache.Delete(ctx, cacheKey)

	return nil
}

// SoftDelete soft deletes a workspace
func (r *workspaceRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.SoftDelete")
	defer span.End()

	query := `
		UPDATE workspaces
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	_, err := r.writeDB.ExecContext(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to soft delete workspace", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": id,
		})
		return errors.Wrap(err, "failed to soft delete workspace")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("workspace:%s", id)
	_ = r.cache.Delete(ctx, cacheKey)

	return nil
}

// GetActiveWorkspaces retrieves all active workspaces
func (r *workspaceRepository) GetActiveWorkspaces(ctx context.Context, since time.Time) ([]*models.Workspace, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.GetActiveWorkspaces")
	defer span.End()

	query := `
		SELECT
			id, tenant_id, name, description, type, owner_id,
			is_active, is_public, settings, tags, metadata,
			created_at, updated_at, deleted_at, created_by, updated_by
		FROM workspaces
		WHERE is_active = true 
			AND deleted_at IS NULL
			AND last_activity_at > $1
		ORDER BY last_activity_at DESC
	`

	var workspaces []*models.Workspace
	err := r.readDB.SelectContext(ctx, &workspaces, query, since)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get active workspaces")
	}

	return workspaces, nil
}

// GetWorkspaceStats retrieves workspace statistics
func (r *workspaceRepository) GetWorkspaceStats(ctx context.Context, workspaceID uuid.UUID) (*interfaces.WorkspaceStats, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.GetWorkspaceStats")
	defer span.End()

	stats := &interfaces.WorkspaceStats{}

	// Get member count
	memberQuery := `
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN last_seen_at > NOW() - INTERVAL '7 days' THEN 1 END) as active
		FROM workspace_members
		WHERE workspace_id = $1
	`
	var memberStats struct {
		Total  int64 `db:"total"`
		Active int64 `db:"active"`
	}
	err := r.readDB.GetContext(ctx, &memberStats, memberQuery, workspaceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get member stats")
	}
	stats.TotalMembers = memberStats.Total
	stats.ActiveMembers = memberStats.Active

	// Get document count
	docQuery := `SELECT COUNT(*) FROM documents WHERE workspace_id = $1`
	err = r.readDB.GetContext(ctx, &stats.TotalDocuments, docQuery, workspaceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get document count")
	}

	// Get operation count
	opQuery := `SELECT COUNT(*) FROM document_operations WHERE workspace_id = $1`
	err = r.readDB.GetContext(ctx, &stats.TotalOperations, opQuery, workspaceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get operation count")
	}

	// Get last activity
	activityQuery := `SELECT MAX(last_activity_at) FROM workspaces WHERE id = $1`
	err = r.readDB.GetContext(ctx, &stats.LastActivityAt, activityQuery, workspaceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get last activity")
	}

	// Storage is calculated elsewhere or set to 0
	stats.StorageUsedBytes = 0

	return stats, nil
}

// RecordActivity records an activity in a workspace
func (r *workspaceRepository) RecordActivity(ctx context.Context, workspaceID uuid.UUID, activity *models.WorkspaceActivity) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.RecordActivity")
	defer span.End()

	// Set defaults
	if activity.ID == uuid.Nil {
		activity.ID = uuid.New()
	}
	if activity.Timestamp.IsZero() {
		activity.Timestamp = time.Now()
	}
	activity.WorkspaceID = workspaceID

	query := `
		INSERT INTO workspace_activities (
			id, workspace_id, agent_id, activity_type,
			description, details, timestamp
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.writeDB.ExecContext(ctx, query,
		activity.ID,
		activity.WorkspaceID,
		activity.AgentID,
		activity.ActivityType,
		activity.Description,
		activity.Details,
		activity.Timestamp,
	)

	if err != nil {
		r.logger.Error("Failed to record activity", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": workspaceID,
			"activity_id":  activity.ID,
		})
		return errors.Wrap(err, "failed to record activity")
	}

	// Update last activity timestamp
	updateQuery := `UPDATE workspaces SET last_activity_at = NOW() WHERE id = $1`
	_, _ = r.writeDB.ExecContext(ctx, updateQuery, workspaceID)

	return nil
}

// PurgeInactiveWorkspaces removes inactive workspaces
func (r *workspaceRepository) PurgeInactiveWorkspaces(ctx context.Context, inactiveSince time.Duration) (int64, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.PurgeInactiveWorkspaces")
	defer span.End()

	cutoffTime := time.Now().Add(-inactiveSince)

	query := `
		DELETE FROM workspaces
		WHERE is_active = false
			AND last_activity_at < $1
			AND deleted_at IS NOT NULL
	`

	result, err := r.writeDB.ExecContext(ctx, query, cutoffTime)
	if err != nil {
		r.logger.Error("Failed to purge inactive workspaces", map[string]interface{}{
			"error":       err.Error(),
			"cutoff_time": cutoffTime,
		})
		return 0, errors.Wrap(err, "failed to purge inactive workspaces")
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rows affected")
	}

	return count, nil
}

// ValidateWorkspaceIntegrity validates workspace data integrity
func (r *workspaceRepository) ValidateWorkspaceIntegrity(ctx context.Context, workspaceID uuid.UUID) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.ValidateWorkspaceIntegrity")
	defer span.End()

	// Check workspace exists
	var count int
	query := `SELECT COUNT(*) FROM workspaces WHERE id = $1`
	err := r.readDB.GetContext(ctx, &count, query, workspaceID)
	if err != nil {
		return errors.Wrap(err, "failed to check workspace existence")
	}
	if count == 0 {
		return errors.New("workspace not found")
	}

	// Check for orphaned members
	orphanQuery := `
		SELECT COUNT(*) FROM workspace_members wm
		WHERE wm.workspace_id = $1
			AND NOT EXISTS (
				SELECT 1 FROM agents a WHERE a.id::text = wm.agent_id
			)
	`
	var orphanCount int
	err = r.readDB.GetContext(ctx, &orphanCount, orphanQuery, workspaceID)
	if err != nil {
		return errors.Wrap(err, "failed to check orphaned members")
	}
	if orphanCount > 0 {
		return fmt.Errorf("found %d orphaned members", orphanCount)
	}

	// Check for workspace without owner
	ownerQuery := `
		SELECT COUNT(*) FROM workspaces w
		WHERE w.id = $1
			AND NOT EXISTS (
				SELECT 1 FROM workspace_members wm 
				WHERE wm.workspace_id = w.id AND wm.role = 'owner'
			)
	`
	var noOwnerCount int
	err = r.readDB.GetContext(ctx, &noOwnerCount, ownerQuery, workspaceID)
	if err != nil {
		return errors.Wrap(err, "failed to check workspace owner")
	}
	if noOwnerCount > 0 {
		return errors.New("workspace has no owner")
	}

	return nil
}

// SearchWorkspaces searches workspaces by query
func (r *workspaceRepository) SearchWorkspaces(ctx context.Context, query string, filters interfaces.WorkspaceFilters) ([]*models.Workspace, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.SearchWorkspaces")
	defer span.End()

	sqlQuery := `
		SELECT
			id, tenant_id, name, description, type, owner_id,
			is_active, is_public, settings, tags, metadata,
			created_at, updated_at, deleted_at, created_by, updated_by
		FROM workspaces
		WHERE deleted_at IS NULL
			AND (name ILIKE $1 OR description ILIKE $1)
	`

	args := []interface{}{"%%" + query + "%%"}
	argCount := 1

	// Apply filters
	if filters.IsActive != nil {
		argCount++
		sqlQuery += fmt.Sprintf(" AND is_active = $%d", argCount)
		args = append(args, *filters.IsActive)
	}

	if len(filters.Type) > 0 {
		argCount++
		sqlQuery += fmt.Sprintf(" AND type = ANY($%d)", argCount)
		args = append(args, pq.Array(filters.Type))
	}

	if len(filters.Visibility) > 0 {
		argCount++
		sqlQuery += fmt.Sprintf(" AND visibility = ANY($%d)", argCount)
		args = append(args, pq.Array(filters.Visibility))
	}

	// Add sorting
	if filters.SortBy != "" {
		order := "ASC"
		if filters.SortOrder == "desc" {
			order = "DESC"
		}
		sqlQuery += fmt.Sprintf(" ORDER BY %s %s", filters.SortBy, order)
	} else {
		sqlQuery += " ORDER BY created_at DESC"
	}

	// Add pagination
	if filters.Limit > 0 {
		sqlQuery += fmt.Sprintf(" LIMIT %d", filters.Limit)
	}
	if filters.Offset > 0 {
		sqlQuery += fmt.Sprintf(" OFFSET %d", filters.Offset)
	}

	var workspaces []*models.Workspace
	err := r.readDB.SelectContext(ctx, &workspaces, sqlQuery, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to search workspaces")
	}

	return workspaces, nil
}

// GetRecentActivity retrieves recent activity for a workspace
func (r *workspaceRepository) GetRecentActivity(ctx context.Context, workspaceID uuid.UUID, limit int) ([]*models.WorkspaceActivity, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.GetRecentActivity")
	defer span.End()

	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT
			id, workspace_id, agent_id, activity_type,
			description, details, timestamp
		FROM workspace_activities
		WHERE workspace_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	var activities []*models.WorkspaceActivity
	err := r.readDB.SelectContext(ctx, &activities, query, workspaceID, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get recent activity")
	}

	return activities, nil
}

// GetState retrieves the current state of a workspace
func (r *workspaceRepository) GetState(ctx context.Context, workspaceID uuid.UUID) (map[string]interface{}, int64, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.GetState")
	defer span.End()

	query := `
		SELECT state, state_version
		FROM workspaces
		WHERE id = $1
	`

	var result struct {
		State        map[string]interface{} `db:"state"`
		StateVersion int64                  `db:"state_version"`
	}

	err := r.readDB.GetContext(ctx, &result, query, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, errors.New("workspace not found")
		}
		return nil, 0, errors.Wrap(err, "failed to get workspace state")
	}

	return result.State, result.StateVersion, nil
}

// ListPublic retrieves all public workspaces
func (r *workspaceRepository) ListPublic(ctx context.Context) ([]*models.Workspace, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.ListPublic")
	defer span.End()

	query := `
		SELECT
			id, tenant_id, name, description, type, owner_id,
			is_active, is_public, settings, tags, metadata,
			created_at, updated_at, deleted_at, created_by, updated_by
		FROM workspaces
		WHERE is_public = true 
			AND is_active = true
			AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	var workspaces []*models.Workspace
	err := r.readDB.SelectContext(ctx, &workspaces, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list public workspaces")
	}

	return workspaces, nil
}

// Search is an alias for SearchWorkspaces for compatibility
func (r *workspaceRepository) Search(ctx context.Context, query string, filters interfaces.WorkspaceFilters) ([]*models.Workspace, error) {
	return r.SearchWorkspaces(ctx, query, filters)
}

// UpdateMemberPermissions updates a member's permissions in a workspace
func (r *workspaceRepository) UpdateMemberPermissions(ctx context.Context, workspaceID uuid.UUID, agentID string, permissions []string) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.UpdateMemberPermissions")
	defer span.End()

	query := `
		UPDATE workspace_members
		SET permissions = $3, last_seen_at = NOW()
		WHERE workspace_id = $1 AND agent_id = $2
	`

	permMap := make(map[string]interface{})
	for _, perm := range permissions {
		permMap[perm] = true
	}

	_, err := r.writeDB.ExecContext(ctx, query, workspaceID, agentID, permMap)
	if err != nil {
		r.logger.Error("Failed to update member permissions", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": workspaceID,
			"agent_id":     agentID,
		})
		return errors.Wrap(err, "failed to update member permissions")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("workspace:members:%s", workspaceID)
	_ = r.cache.Delete(ctx, cacheKey)

	return nil
}

// GetMemberActivity retrieves member activity for a workspace
func (r *workspaceRepository) GetMemberActivity(ctx context.Context, workspaceID uuid.UUID) ([]*models.MemberActivity, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.GetMemberActivity")
	defer span.End()

	query := `
		SELECT 
			wm.agent_id,
			wm.role,
			wm.joined_at,
			wm.last_seen_at,
			COUNT(wa.id) as activity_count,
			MAX(wa.timestamp) as last_activity
		FROM workspace_members wm
		LEFT JOIN workspace_activities wa ON wa.workspace_id = wm.workspace_id AND wa.agent_id = wm.agent_id
		WHERE wm.workspace_id = $1
		GROUP BY wm.agent_id, wm.role, wm.joined_at, wm.last_seen_at
		ORDER BY last_activity DESC NULLS LAST
	`

	rows, err := r.readDB.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get member activity")
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Error("Failed to close rows", map[string]interface{}{"error": err.Error()})
		}
	}()

	var activities []*models.MemberActivity
	for rows.Next() {
		var activity models.MemberActivity
		var lastSeenAt, lastActivity sql.NullTime

		var role string
		var joinedAt time.Time

		err := rows.Scan(
			&activity.AgentID,
			&role,
			&joinedAt,
			&lastSeenAt,
			&activity.ActivityCount,
			&lastActivity,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan member activity")
		}

		if lastActivity.Valid {
			activity.LastActivityAt = lastActivity.Time
		}

		activity.WorkspaceID = workspaceID

		activityCopy := activity
		activities = append(activities, &activityCopy)
	}

	return activities, nil
}

// MergeState merges remote state with local state
func (r *workspaceRepository) MergeState(ctx context.Context, workspaceID uuid.UUID, remoteState *models.WorkspaceState) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.MergeState")
	defer span.End()

	// Get current state
	currentState, currentVersion, err := r.GetState(ctx, workspaceID)
	if err != nil {
		return err
	}

	// Merge states (simple last-write-wins for now)
	mergedState := make(map[string]interface{})
	for k, v := range currentState {
		mergedState[k] = v
	}
	if remoteState.Data != nil {
		for k, v := range remoteState.Data {
			mergedState[k] = v
		}
	}

	// Update with new version
	return r.UpdateState(ctx, workspaceID, mergedState, currentVersion+1)
}

// GetStateHistory retrieves state history for a workspace
func (r *workspaceRepository) GetStateHistory(ctx context.Context, workspaceID uuid.UUID, limit int) ([]*models.StateSnapshot, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.GetStateHistory")
	defer span.End()

	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT
			id, workspace_id, state, state_version,
			created_at, created_by
		FROM workspace_state_history
		WHERE workspace_id = $1
		ORDER BY state_version DESC
		LIMIT $2
	`

	var snapshots []*models.StateSnapshot
	err := r.readDB.SelectContext(ctx, &snapshots, query, workspaceID, limit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get state history")
	}

	return snapshots, nil
}

// RestoreState restores workspace state from a snapshot
func (r *workspaceRepository) RestoreState(ctx context.Context, workspaceID uuid.UUID, snapshotID uuid.UUID) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.RestoreState")
	defer span.End()

	// Get snapshot
	var snapshot models.StateSnapshot
	query := `
		SELECT state, state_version
		FROM workspace_state_history
		WHERE id = $1 AND workspace_id = $2
	`
	err := r.readDB.GetContext(ctx, &snapshot, query, snapshotID, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("snapshot not found")
		}
		return errors.Wrap(err, "failed to get snapshot")
	}

	// Get current version
	_, currentVersion, err := r.GetState(ctx, workspaceID)
	if err != nil {
		return err
	}

	// Restore state with new version
	return r.UpdateState(ctx, workspaceID, snapshot.State, currentVersion+1)
}

// GetStats retrieves workspace statistics (alias for GetWorkspaceStats)
func (r *workspaceRepository) GetStats(ctx context.Context, workspaceID uuid.UUID) (*models.WorkspaceStats, error) {
	stats, err := r.GetWorkspaceStats(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	// Convert from interfaces.WorkspaceStats to models.WorkspaceStats
	return &models.WorkspaceStats{
		TotalMembers:     stats.TotalMembers,
		ActiveMembers:    stats.ActiveMembers,
		TotalDocuments:   stats.TotalDocuments,
		TotalOperations:  stats.TotalOperations,
		StorageUsedBytes: stats.StorageUsedBytes,
		LastActivityAt:   stats.LastActivityAt,
	}, nil
}

// GetCollaborationMetrics retrieves collaboration metrics for a workspace
func (r *workspaceRepository) GetCollaborationMetrics(ctx context.Context, workspaceID uuid.UUID, period time.Duration) (*models.CollaborationMetrics, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.GetCollaborationMetrics")
	defer span.End()

	since := time.Now().Add(-period)

	metrics := &models.CollaborationMetrics{
		DocumentID: workspaceID, // Using workspace ID as document ID for workspace-wide metrics
		Period:     period,
	}

	// Get active users
	userQuery := `
		SELECT COUNT(DISTINCT agent_id)
		FROM workspace_activities
		WHERE workspace_id = $1 AND timestamp > $2
	`
	var activeUsers int
	err := r.readDB.GetContext(ctx, &activeUsers, userQuery, workspaceID, since)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get active users")
	}

	// Get total edits
	editQuery := `
		SELECT COUNT(*)
		FROM document_operations
		WHERE workspace_id = $1 AND timestamp > $2
	`
	err = r.readDB.GetContext(ctx, &metrics.TotalOperations, editQuery, workspaceID, since)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get total edits")
	}

	// Get messages sent (from activities)
	msgQuery := `
		SELECT COUNT(*)
		FROM workspace_activities
		WHERE workspace_id = $1 
			AND activity_type = 'message'
			AND timestamp > $2
	`
	var messagesSent int64
	err = r.readDB.GetContext(ctx, &messagesSent, msgQuery, workspaceID, since)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get messages sent")
	}

	// Get files shared (from activities)
	fileQuery := `
		SELECT COUNT(*)
		FROM workspace_activities
		WHERE workspace_id = $1 
			AND activity_type = 'file_shared'
			AND timestamp > $2
	`
	var filesShared int64
	err = r.readDB.GetContext(ctx, &filesShared, fileQuery, workspaceID, since)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get files shared")
	}

	// Calculate average response time (simplified)
	metrics.AverageResponseTime = 5 * time.Minute // Placeholder

	// Set unique collaborators and peak concurrency
	metrics.UniqueCollaborators = activeUsers
	metrics.PeakConcurrency = activeUsers // Simplified

	// Initialize operations by type map
	metrics.OperationsByType = map[string]int64{
		"edit":    metrics.TotalOperations,
		"message": messagesSent,
		"file":    filesShared,
	}

	return metrics, nil
}

// GetPresence retrieves presence information for workspace members
func (r *workspaceRepository) GetPresence(ctx context.Context, workspaceID uuid.UUID) ([]*models.MemberPresence, error) {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.GetPresence")
	defer span.End()

	query := `
		SELECT
			wm.agent_id,
			wm.role,
			wp.status,
			wp.last_seen_at,
			wp.current_document_id,
			wp.cursor_position
		FROM workspace_members wm
		LEFT JOIN workspace_presence wp ON wp.workspace_id = wm.workspace_id AND wp.agent_id = wm.agent_id
		WHERE wm.workspace_id = $1
		ORDER BY wm.joined_at
	`

	rows, err := r.readDB.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get presence")
	}
	defer func() {
		if err := rows.Close(); err != nil {
			r.logger.Error("Failed to close rows", map[string]interface{}{"error": err.Error()})
		}
	}()

	var presences []*models.MemberPresence
	for rows.Next() {
		var presence models.MemberPresence
		var status sql.NullString
		var lastSeenAt sql.NullTime
		var currentDocumentID sql.NullString
		var cursorPosition sql.NullString

		var role string

		err := rows.Scan(
			&presence.AgentID,
			&role,
			&status,
			&lastSeenAt,
			&currentDocumentID,
			&cursorPosition,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan presence")
		}

		presence.WorkspaceID = workspaceID

		if status.Valid {
			presence.Status = status.String
		} else {
			presence.Status = "offline"
		}

		if lastSeenAt.Valid {
			presence.LastSeenAt = lastSeenAt.Time
		}

		if currentDocumentID.Valid {
			presence.Location = currentDocumentID.String
		}

		presenceCopy := presence
		presences = append(presences, &presenceCopy)
	}

	return presences, nil
}

// UpdatePresence updates presence status for a workspace member
func (r *workspaceRepository) UpdatePresence(ctx context.Context, workspaceID uuid.UUID, agentID string, status string) error {
	ctx, span := r.tracer(ctx, "WorkspaceRepository.UpdatePresence")
	defer span.End()

	query := `
		INSERT INTO workspace_presence (
			workspace_id, agent_id, status, last_seen_at
		) VALUES ($1, $2, $3, NOW())
		ON CONFLICT (workspace_id, agent_id) DO UPDATE SET
			status = EXCLUDED.status,
			last_seen_at = EXCLUDED.last_seen_at
	`

	_, err := r.writeDB.ExecContext(ctx, query, workspaceID, agentID, status)
	if err != nil {
		r.logger.Error("Failed to update presence", map[string]interface{}{
			"error":        err.Error(),
			"workspace_id": workspaceID,
			"agent_id":     agentID,
			"status":       status,
		})
		return errors.Wrap(err, "failed to update presence")
	}

	return nil
}
