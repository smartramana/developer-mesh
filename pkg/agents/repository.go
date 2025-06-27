package agents

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Repository defines the interface for agent configuration storage
type Repository interface {
	// CreateConfig creates a new agent configuration
	CreateConfig(ctx context.Context, config *AgentConfig) error

	// GetConfig retrieves the active configuration for an agent
	GetConfig(ctx context.Context, agentID string) (*AgentConfig, error)

	// GetConfigByID retrieves a specific configuration by ID
	GetConfigByID(ctx context.Context, id uuid.UUID) (*AgentConfig, error)

	// GetConfigHistory retrieves configuration history for an agent
	GetConfigHistory(ctx context.Context, agentID string, limit int) ([]*AgentConfig, error)

	// UpdateConfig creates a new version of the configuration
	UpdateConfig(ctx context.Context, agentID string, update *ConfigUpdateRequest) (*AgentConfig, error)

	// ListConfigs lists configurations based on filter
	ListConfigs(ctx context.Context, filter ConfigFilter) ([]*AgentConfig, error)

	// DeactivateConfig deactivates a configuration
	DeactivateConfig(ctx context.Context, agentID string) error

	// DeleteConfig deletes a configuration (soft delete)
	DeleteConfig(ctx context.Context, id uuid.UUID) error
}

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db     *sqlx.DB
	schema string
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *sqlx.DB, schema string) *PostgresRepository {
	if schema == "" {
		schema = "mcp"
	}
	return &PostgresRepository{
		db:     db,
		schema: schema,
	}
}

// CreateConfig creates a new agent configuration
func (r *PostgresRepository) CreateConfig(ctx context.Context, config *AgentConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Set defaults
	config.ID = uuid.New()
	config.Version = 1
	config.IsActive = true
	config.CreatedAt = time.Now()
	config.UpdatedAt = config.CreatedAt

	// Convert to JSON for storage
	modelPrefsJSON, err := json.Marshal(config.ModelPreferences)
	if err != nil {
		return fmt.Errorf("failed to marshal model preferences: %w", err)
	}

	constraintsJSON, err := json.Marshal(config.Constraints)
	if err != nil {
		return fmt.Errorf("failed to marshal constraints: %w", err)
	}

	fallbackJSON, err := json.Marshal(config.FallbackBehavior)
	if err != nil {
		return fmt.Errorf("failed to marshal fallback behavior: %w", err)
	}

	metadataJSON, err := json.Marshal(config.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.agent_configs (
			id, agent_id, version, embedding_strategy,
			model_preferences, constraints, fallback_behavior,
			metadata, is_active, created_at, updated_at, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
	`, r.schema)

	_, err = r.db.ExecContext(ctx, query,
		config.ID,
		config.AgentID,
		config.Version,
		config.EmbeddingStrategy,
		modelPrefsJSON,
		constraintsJSON,
		fallbackJSON,
		metadataJSON,
		config.IsActive,
		config.CreatedAt,
		config.UpdatedAt,
		config.CreatedBy,
	)

	if err != nil {
		return fmt.Errorf("failed to insert configuration: %w", err)
	}

	return nil
}

// GetConfig retrieves the active configuration for an agent
func (r *PostgresRepository) GetConfig(ctx context.Context, agentID string) (*AgentConfig, error) {
	query := fmt.Sprintf(`
		SELECT 
			id, agent_id, version, embedding_strategy,
			model_preferences, constraints, fallback_behavior,
			metadata, is_active, created_at, updated_at, created_by
		FROM %s.agent_configs
		WHERE agent_id = $1 AND is_active = true
		ORDER BY version DESC
		LIMIT 1
	`, r.schema)

	var config AgentConfig
	var modelPrefsJSON, constraintsJSON, fallbackJSON, metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, agentID).Scan(
		&config.ID,
		&config.AgentID,
		&config.Version,
		&config.EmbeddingStrategy,
		&modelPrefsJSON,
		&constraintsJSON,
		&fallbackJSON,
		&metadataJSON,
		&config.IsActive,
		&config.CreatedAt,
		&config.UpdatedAt,
		&config.CreatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("configuration not found for agent: %s", agentID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query configuration: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(modelPrefsJSON, &config.ModelPreferences); err != nil {
		return nil, fmt.Errorf("failed to unmarshal model preferences: %w", err)
	}

	if err := json.Unmarshal(constraintsJSON, &config.Constraints); err != nil {
		return nil, fmt.Errorf("failed to unmarshal constraints: %w", err)
	}

	if err := json.Unmarshal(fallbackJSON, &config.FallbackBehavior); err != nil {
		return nil, fmt.Errorf("failed to unmarshal fallback behavior: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &config.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &config, nil
}

// GetConfigByID retrieves a specific configuration by ID
func (r *PostgresRepository) GetConfigByID(ctx context.Context, id uuid.UUID) (*AgentConfig, error) {
	query := fmt.Sprintf(`
		SELECT 
			id, agent_id, version, embedding_strategy,
			model_preferences, constraints, fallback_behavior,
			metadata, is_active, created_at, updated_at, created_by
		FROM %s.agent_configs
		WHERE id = $1
	`, r.schema)

	var config AgentConfig
	var modelPrefsJSON, constraintsJSON, fallbackJSON, metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&config.ID,
		&config.AgentID,
		&config.Version,
		&config.EmbeddingStrategy,
		&modelPrefsJSON,
		&constraintsJSON,
		&fallbackJSON,
		&metadataJSON,
		&config.IsActive,
		&config.CreatedAt,
		&config.UpdatedAt,
		&config.CreatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("configuration not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query configuration: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(modelPrefsJSON, &config.ModelPreferences); err != nil {
		return nil, fmt.Errorf("failed to unmarshal model preferences: %w", err)
	}

	if err := json.Unmarshal(constraintsJSON, &config.Constraints); err != nil {
		return nil, fmt.Errorf("failed to unmarshal constraints: %w", err)
	}

	if err := json.Unmarshal(fallbackJSON, &config.FallbackBehavior); err != nil {
		return nil, fmt.Errorf("failed to unmarshal fallback behavior: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &config.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &config, nil
}

// GetConfigHistory retrieves configuration history for an agent
func (r *PostgresRepository) GetConfigHistory(ctx context.Context, agentID string, limit int) ([]*AgentConfig, error) {
	if limit <= 0 {
		limit = 10
	}

	query := fmt.Sprintf(`
		SELECT 
			id, agent_id, version, embedding_strategy,
			model_preferences, constraints, fallback_behavior,
			metadata, is_active, created_at, updated_at, created_by
		FROM %s.agent_configs
		WHERE agent_id = $1
		ORDER BY version DESC
		LIMIT $2
	`, r.schema)

	rows, err := r.db.QueryContext(ctx, query, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query configuration history: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var configs []*AgentConfig

	for rows.Next() {
		var config AgentConfig
		var modelPrefsJSON, constraintsJSON, fallbackJSON, metadataJSON []byte

		err := rows.Scan(
			&config.ID,
			&config.AgentID,
			&config.Version,
			&config.EmbeddingStrategy,
			&modelPrefsJSON,
			&constraintsJSON,
			&fallbackJSON,
			&metadataJSON,
			&config.IsActive,
			&config.CreatedAt,
			&config.UpdatedAt,
			&config.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan configuration: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(modelPrefsJSON, &config.ModelPreferences); err != nil {
			return nil, fmt.Errorf("failed to unmarshal model preferences: %w", err)
		}

		if err := json.Unmarshal(constraintsJSON, &config.Constraints); err != nil {
			return nil, fmt.Errorf("failed to unmarshal constraints: %w", err)
		}

		if err := json.Unmarshal(fallbackJSON, &config.FallbackBehavior); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fallback behavior: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &config.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		configs = append(configs, &config)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating configurations: %w", err)
	}

	return configs, nil
}

// UpdateConfig creates a new version of the configuration
func (r *PostgresRepository) UpdateConfig(ctx context.Context, agentID string, update *ConfigUpdateRequest) (*AgentConfig, error) {
	// Start transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // Rollback is a no-op if transaction was committed
	}()

	// Get current configuration
	current, err := r.GetConfig(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Deactivate current configuration
	deactivateQuery := fmt.Sprintf(`
		UPDATE %s.agent_configs
		SET is_active = false, updated_at = $1
		WHERE agent_id = $2 AND is_active = true
	`, r.schema)

	if _, err := tx.ExecContext(ctx, deactivateQuery, time.Now(), agentID); err != nil {
		return nil, fmt.Errorf("failed to deactivate current configuration: %w", err)
	}

	// Create new configuration based on current + updates
	newConfig := current.Clone()
	newConfig.ID = uuid.New()
	newConfig.Version = current.Version + 1
	newConfig.CreatedAt = time.Now()
	newConfig.UpdatedAt = newConfig.CreatedAt
	newConfig.CreatedBy = update.UpdatedBy

	// Apply updates
	if update.EmbeddingStrategy != nil {
		newConfig.EmbeddingStrategy = *update.EmbeddingStrategy
	}
	if update.ModelPreferences != nil {
		newConfig.ModelPreferences = update.ModelPreferences
	}
	if update.Constraints != nil {
		newConfig.Constraints = *update.Constraints
	}
	if update.FallbackBehavior != nil {
		newConfig.FallbackBehavior = *update.FallbackBehavior
	}
	if update.Metadata != nil {
		for k, v := range update.Metadata {
			if newConfig.Metadata == nil {
				newConfig.Metadata = make(map[string]interface{})
			}
			newConfig.Metadata[k] = v
		}
	}

	// Validate new configuration
	if err := newConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid updated configuration: %w", err)
	}

	// Insert new configuration
	modelPrefsJSON, _ := json.Marshal(newConfig.ModelPreferences)
	constraintsJSON, _ := json.Marshal(newConfig.Constraints)
	fallbackJSON, _ := json.Marshal(newConfig.FallbackBehavior)
	metadataJSON, _ := json.Marshal(newConfig.Metadata)

	insertQuery := fmt.Sprintf(`
		INSERT INTO %s.agent_configs (
			id, agent_id, version, embedding_strategy,
			model_preferences, constraints, fallback_behavior,
			metadata, is_active, created_at, updated_at, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
	`, r.schema)

	_, err = tx.ExecContext(ctx, insertQuery,
		newConfig.ID,
		newConfig.AgentID,
		newConfig.Version,
		newConfig.EmbeddingStrategy,
		modelPrefsJSON,
		constraintsJSON,
		fallbackJSON,
		metadataJSON,
		newConfig.IsActive,
		newConfig.CreatedAt,
		newConfig.UpdatedAt,
		newConfig.CreatedBy,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to insert new configuration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return newConfig, nil
}

// ListConfigs lists configurations based on filter
func (r *PostgresRepository) ListConfigs(ctx context.Context, filter ConfigFilter) ([]*AgentConfig, error) {
	// Build query
	query := fmt.Sprintf(`
		SELECT 
			id, agent_id, version, embedding_strategy,
			model_preferences, constraints, fallback_behavior,
			metadata, is_active, created_at, updated_at, created_by
		FROM %s.agent_configs
		WHERE 1=1
	`, r.schema)

	args := []interface{}{}
	argCount := 0

	if filter.AgentID != nil {
		argCount++
		query += fmt.Sprintf(" AND agent_id = $%d", argCount)
		args = append(args, *filter.AgentID)
	}

	if filter.IsActive != nil {
		argCount++
		query += fmt.Sprintf(" AND is_active = $%d", argCount)
		args = append(args, *filter.IsActive)
	}

	if filter.Strategy != nil {
		argCount++
		query += fmt.Sprintf(" AND embedding_strategy = $%d", argCount)
		args = append(args, *filter.Strategy)
	}

	if filter.CreatedBy != nil {
		argCount++
		query += fmt.Sprintf(" AND created_by = $%d", argCount)
		args = append(args, *filter.CreatedBy)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query configurations: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var configs []*AgentConfig

	for rows.Next() {
		var config AgentConfig
		var modelPrefsJSON, constraintsJSON, fallbackJSON, metadataJSON []byte

		err := rows.Scan(
			&config.ID,
			&config.AgentID,
			&config.Version,
			&config.EmbeddingStrategy,
			&modelPrefsJSON,
			&constraintsJSON,
			&fallbackJSON,
			&metadataJSON,
			&config.IsActive,
			&config.CreatedAt,
			&config.UpdatedAt,
			&config.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan configuration: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(modelPrefsJSON, &config.ModelPreferences); err != nil {
			return nil, fmt.Errorf("failed to unmarshal model preferences: %w", err)
		}
		if err := json.Unmarshal(constraintsJSON, &config.Constraints); err != nil {
			return nil, fmt.Errorf("failed to unmarshal constraints: %w", err)
		}
		if err := json.Unmarshal(fallbackJSON, &config.FallbackBehavior); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fallback behavior: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &config.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		configs = append(configs, &config)
	}

	return configs, nil
}

// DeactivateConfig deactivates a configuration
func (r *PostgresRepository) DeactivateConfig(ctx context.Context, agentID string) error {
	query := fmt.Sprintf(`
		UPDATE %s.agent_configs
		SET is_active = false, updated_at = $1
		WHERE agent_id = $2 AND is_active = true
	`, r.schema)

	result, err := r.db.ExecContext(ctx, query, time.Now(), agentID)
	if err != nil {
		return fmt.Errorf("failed to deactivate configuration: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no active configuration found for agent: %s", agentID)
	}

	return nil
}

// DeleteConfig deletes a configuration (soft delete by deactivating)
func (r *PostgresRepository) DeleteConfig(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf(`
		UPDATE %s.agent_configs
		SET is_active = false, updated_at = $1
		WHERE id = $2
	`, r.schema)

	result, err := r.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to delete configuration: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("configuration not found: %s", id)
	}

	return nil
}
