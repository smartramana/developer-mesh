package credential

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// PostgresRepository implements credential repository using PostgreSQL
type PostgresRepository struct {
	db *sqlx.DB
}

// NewPostgresRepository creates a new PostgreSQL credential repository
func NewPostgresRepository(db *sqlx.DB) Repository {
	return &PostgresRepository{db: db}
}

// Create stores new encrypted credentials for a user
func (r *PostgresRepository) Create(ctx context.Context, credential *models.UserCredential) error {
	// Marshal metadata to JSON
	metadataJSON, err := json.Marshal(credential.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO mcp.user_credentials (
			id, tenant_id, user_id, service_type, encrypted_credentials,
			encryption_key_version, is_active, metadata, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, user_id, service_type)
		DO UPDATE SET
			encrypted_credentials = EXCLUDED.encrypted_credentials,
			encryption_key_version = EXCLUDED.encryption_key_version,
			metadata = EXCLUDED.metadata,
			expires_at = EXCLUDED.expires_at,
			is_active = true,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, created_at, updated_at
	`

	var id string
	var createdAt, updatedAt time.Time
	err = r.db.QueryRowContext(
		ctx, query,
		credential.ID,
		credential.TenantID,
		credential.UserID,
		credential.ServiceType,
		credential.EncryptedCredentials,
		credential.EncryptionKeyVersion,
		credential.IsActive,
		metadataJSON,
		credential.ExpiresAt,
	).Scan(&id, &createdAt, &updatedAt)

	if err != nil {
		return fmt.Errorf("failed to create credential: %w", err)
	}

	credential.ID = id
	credential.CreatedAt = createdAt
	credential.UpdatedAt = updatedAt

	return nil
}

// Get retrieves credentials for a specific service
func (r *PostgresRepository) Get(ctx context.Context, tenantID, userID string, serviceType models.ServiceType) (*models.UserCredential, error) {
	query := `
		SELECT
			id, tenant_id, user_id, service_type, encrypted_credentials,
			encryption_key_version, is_active, metadata,
			created_at, updated_at, last_used_at, expires_at
		FROM mcp.user_credentials
		WHERE tenant_id = $1 AND user_id = $2 AND service_type = $3 AND is_active = true
	`

	var cred models.UserCredential
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, tenantID, userID, serviceType).Scan(
		&cred.ID,
		&cred.TenantID,
		&cred.UserID,
		&cred.ServiceType,
		&cred.EncryptedCredentials,
		&cred.EncryptionKeyVersion,
		&cred.IsActive,
		&metadataJSON,
		&cred.CreatedAt,
		&cred.UpdatedAt,
		&cred.LastUsedAt,
		&cred.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("credentials not found for service %s", serviceType)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &cred.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &cred, nil
}

// GetByID retrieves credentials by ID
func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*models.UserCredential, error) {
	query := `
		SELECT
			id, tenant_id, user_id, service_type, encrypted_credentials,
			encryption_key_version, is_active, metadata,
			created_at, updated_at, last_used_at, expires_at
		FROM mcp.user_credentials
		WHERE id = $1 AND is_active = true
	`

	var cred models.UserCredential
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&cred.ID,
		&cred.TenantID,
		&cred.UserID,
		&cred.ServiceType,
		&cred.EncryptedCredentials,
		&cred.EncryptionKeyVersion,
		&cred.IsActive,
		&metadataJSON,
		&cred.CreatedAt,
		&cred.UpdatedAt,
		&cred.LastUsedAt,
		&cred.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("credential not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &cred.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &cred, nil
}

// List retrieves all credentials for a user
func (r *PostgresRepository) List(ctx context.Context, tenantID, userID string) ([]*models.UserCredential, error) {
	query := `
		SELECT
			id, tenant_id, user_id, service_type, encrypted_credentials,
			encryption_key_version, is_active, metadata,
			created_at, updated_at, last_used_at, expires_at
		FROM mcp.user_credentials
		WHERE tenant_id = $1 AND user_id = $2 AND is_active = true
		ORDER BY service_type ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}
	defer func() {
		_ = rows.Close() // Error intentionally ignored to not override return error
	}()

	var credentials []*models.UserCredential
	for rows.Next() {
		var cred models.UserCredential
		var metadataJSON []byte

		err := rows.Scan(
			&cred.ID,
			&cred.TenantID,
			&cred.UserID,
			&cred.ServiceType,
			&cred.EncryptedCredentials,
			&cred.EncryptionKeyVersion,
			&cred.IsActive,
			&metadataJSON,
			&cred.CreatedAt,
			&cred.UpdatedAt,
			&cred.LastUsedAt,
			&cred.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan credential: %w", err)
		}

		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &cred.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		credentials = append(credentials, &cred)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating credentials: %w", err)
	}

	return credentials, nil
}

// Update updates existing credentials
func (r *PostgresRepository) Update(ctx context.Context, credential *models.UserCredential) error {
	// Marshal metadata to JSON
	metadataJSON, err := json.Marshal(credential.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE mcp.user_credentials
		SET
			encrypted_credentials = $1,
			encryption_key_version = $2,
			metadata = $3,
			expires_at = $4,
			is_active = $5
		WHERE id = $6
	`

	result, err := r.db.ExecContext(
		ctx, query,
		credential.EncryptedCredentials,
		credential.EncryptionKeyVersion,
		metadataJSON,
		credential.ExpiresAt,
		credential.IsActive,
		credential.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update credential: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("credential not found: %s", credential.ID)
	}

	return nil
}

// Delete removes credentials (soft delete by setting is_active=false)
func (r *PostgresRepository) Delete(ctx context.Context, tenantID, userID string, serviceType models.ServiceType) error {
	query := `
		UPDATE mcp.user_credentials
		SET is_active = false, updated_at = CURRENT_TIMESTAMP
		WHERE tenant_id = $1 AND user_id = $2 AND service_type = $3
	`

	result, err := r.db.ExecContext(ctx, query, tenantID, userID, serviceType)
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("credential not found for service %s", serviceType)
	}

	return nil
}

// HardDelete permanently removes credentials (for compliance/user deletion)
func (r *PostgresRepository) HardDelete(ctx context.Context, id string) error {
	query := `DELETE FROM mcp.user_credentials WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to hard delete credential: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("credential not found: %s", id)
	}

	return nil
}

// RecordUsage updates last_used_at timestamp
func (r *PostgresRepository) RecordUsage(ctx context.Context, id string) error {
	query := `
		UPDATE mcp.user_credentials
		SET last_used_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to record usage: %w", err)
	}

	return nil
}

// ListByTenant retrieves all credentials for a tenant (admin operation)
func (r *PostgresRepository) ListByTenant(ctx context.Context, tenantID string) ([]*models.UserCredential, error) {
	query := `
		SELECT
			id, tenant_id, user_id, service_type, encrypted_credentials,
			encryption_key_version, is_active, metadata,
			created_at, updated_at, last_used_at, expires_at
		FROM mcp.user_credentials
		WHERE tenant_id = $1 AND is_active = true
		ORDER BY user_id ASC, service_type ASC
	`

	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenant credentials: %w", err)
	}
	defer func() {
		_ = rows.Close() // Error intentionally ignored to not override return error
	}()

	var credentials []*models.UserCredential
	for rows.Next() {
		var cred models.UserCredential
		var metadataJSON []byte

		err := rows.Scan(
			&cred.ID,
			&cred.TenantID,
			&cred.UserID,
			&cred.ServiceType,
			&cred.EncryptedCredentials,
			&cred.EncryptionKeyVersion,
			&cred.IsActive,
			&metadataJSON,
			&cred.CreatedAt,
			&cred.UpdatedAt,
			&cred.LastUsedAt,
			&cred.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan credential: %w", err)
		}

		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &cred.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		credentials = append(credentials, &cred)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating credentials: %w", err)
	}

	return credentials, nil
}

// AuditLog records an audit entry for credential operation
func (r *PostgresRepository) AuditLog(ctx context.Context, audit *models.UserCredentialAudit) error {
	// Marshal metadata to JSON
	metadataJSON, err := json.Marshal(audit.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal audit metadata: %w", err)
	}

	query := `
		INSERT INTO mcp.user_credentials_audit (
			id, credential_id, tenant_id, user_id, service_type,
			operation, success, error_message, ip_address, user_agent, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err = r.db.ExecContext(
		ctx, query,
		audit.ID,
		audit.CredentialID,
		audit.TenantID,
		audit.UserID,
		audit.ServiceType,
		audit.Operation,
		audit.Success,
		audit.ErrorMessage,
		audit.IPAddress,
		audit.UserAgent,
		metadataJSON,
	)
	if err != nil {
		// Don't fail the operation if audit logging fails, but log it
		if pqErr, ok := err.(*pq.Error); ok {
			return fmt.Errorf("failed to insert audit log (pq error %s): %w", pqErr.Code, err)
		}
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	return nil
}

// GetAuditHistory retrieves audit history for a credential
func (r *PostgresRepository) GetAuditHistory(ctx context.Context, credentialID string, limit int) ([]*models.UserCredentialAudit, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	query := `
		SELECT
			id, credential_id, tenant_id, user_id, service_type,
			operation, success, error_message, ip_address, user_agent, metadata, created_at
		FROM mcp.user_credentials_audit
		WHERE credential_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, credentialID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit history: %w", err)
	}
	defer func() {
		_ = rows.Close() // Error intentionally ignored to not override return error
	}()

	var audits []*models.UserCredentialAudit
	for rows.Next() {
		var audit models.UserCredentialAudit
		var metadataJSON []byte

		err := rows.Scan(
			&audit.ID,
			&audit.CredentialID,
			&audit.TenantID,
			&audit.UserID,
			&audit.ServiceType,
			&audit.Operation,
			&audit.Success,
			&audit.ErrorMessage,
			&audit.IPAddress,
			&audit.UserAgent,
			&metadataJSON,
			&audit.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit entry: %w", err)
		}

		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &audit.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal audit metadata: %w", err)
			}
		}

		audits = append(audits, &audit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit entries: %w", err)
	}

	return audits, nil
}
