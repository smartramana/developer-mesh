package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// TenantConfigRepository defines the interface for tenant configuration operations
type TenantConfigRepository interface {
	GetByTenantID(ctx context.Context, tenantID string) (*models.TenantConfig, error)
	Create(ctx context.Context, config *models.TenantConfig) error
	Update(ctx context.Context, config *models.TenantConfig) error
	Delete(ctx context.Context, tenantID string) error
	Exists(ctx context.Context, tenantID string) (bool, error)
}

// tenantConfigRepository implements TenantConfigRepository
type tenantConfigRepository struct {
	db     *sqlx.DB
	logger observability.Logger
}

// NewTenantConfigRepository creates a new tenant config repository
func NewTenantConfigRepository(db *sqlx.DB, logger observability.Logger) TenantConfigRepository {
	return &tenantConfigRepository{
		db:     db,
		logger: logger,
	}
}

// GetByTenantID retrieves a tenant configuration by tenant ID
func (r *tenantConfigRepository) GetByTenantID(ctx context.Context, tenantID string) (*models.TenantConfig, error) {
	ctx, span := observability.StartSpan(ctx, "repository.tenant_config.GetByTenantID")
	defer span.End()

	query := `
		SELECT 
			id, tenant_id, rate_limit_config, service_tokens, 
			allowed_origins, features, created_at, updated_at
		FROM mcp.tenant_config
		WHERE tenant_id = $1
	`

	var config models.TenantConfig
	err := r.db.GetContext(ctx, &config, query, tenantID)
	if err != nil {
		if err == sql.ErrNoRows {
			r.logger.Debug("Tenant config not found", map[string]interface{}{
				"tenant_id": tenantID,
			})
			return nil, nil
		}
		r.logger.Error("Failed to get tenant config", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		return nil, errors.Wrap(err, "failed to get tenant config")
	}

	// Parse features JSON if present
	if len(config.FeaturesJSON) > 0 {
		if err := json.Unmarshal(config.FeaturesJSON, &config.Features); err != nil {
			r.logger.Warn("Failed to parse features JSON", map[string]interface{}{
				"tenant_id": tenantID,
				"error":     err.Error(),
			})
			config.Features = make(map[string]interface{})
		}
	} else {
		config.Features = make(map[string]interface{})
	}

	// Note: Service tokens remain encrypted in EncryptedTokens field
	// Decryption should be handled by the service layer

	return &config, nil
}

// Create creates a new tenant configuration
func (r *tenantConfigRepository) Create(ctx context.Context, config *models.TenantConfig) error {
	ctx, span := observability.StartSpan(ctx, "repository.tenant_config.Create")
	defer span.End()

	if config.ID == "" {
		config.ID = uuid.New().String()
	}

	// Marshal features to JSON
	featuresJSON, err := json.Marshal(config.Features)
	if err != nil {
		return errors.Wrap(err, "failed to marshal features")
	}

	// Ensure rate limit config has defaults
	if config.RateLimitConfig.DefaultRequestsPerMinute == 0 {
		config.RateLimitConfig = models.DefaultTenantConfig(config.TenantID).RateLimitConfig
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	query := `
		INSERT INTO mcp.tenant_config (
			id, tenant_id, rate_limit_config, service_tokens,
			allowed_origins, features, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
	`

	_, err = r.db.ExecContext(
		ctx, query,
		config.ID,
		config.TenantID,
		config.RateLimitConfig,
		config.EncryptedTokens,
		config.AllowedOrigins,
		featuresJSON,
		config.CreatedAt,
		config.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to create tenant config", map[string]interface{}{
			"tenant_id": config.TenantID,
			"error":     err.Error(),
		})
		return errors.Wrap(err, "failed to create tenant config")
	}

	r.logger.Info("Created tenant config", map[string]interface{}{
		"id":        config.ID,
		"tenant_id": config.TenantID,
	})

	return nil
}

// Update updates an existing tenant configuration
func (r *tenantConfigRepository) Update(ctx context.Context, config *models.TenantConfig) error {
	ctx, span := observability.StartSpan(ctx, "repository.tenant_config.Update")
	defer span.End()

	// Marshal features to JSON
	featuresJSON, err := json.Marshal(config.Features)
	if err != nil {
		return errors.Wrap(err, "failed to marshal features")
	}

	config.UpdatedAt = time.Now()

	query := `
		UPDATE mcp.tenant_config SET
			rate_limit_config = $2,
			service_tokens = $3,
			allowed_origins = $4,
			features = $5,
			updated_at = $6
		WHERE tenant_id = $1
	`

	result, err := r.db.ExecContext(
		ctx, query,
		config.TenantID,
		config.RateLimitConfig,
		config.EncryptedTokens,
		config.AllowedOrigins,
		featuresJSON,
		config.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to update tenant config", map[string]interface{}{
			"tenant_id": config.TenantID,
			"error":     err.Error(),
		})
		return errors.Wrap(err, "failed to update tenant config")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tenant config not found for tenant_id: %s", config.TenantID)
	}

	r.logger.Info("Updated tenant config", map[string]interface{}{
		"tenant_id": config.TenantID,
	})

	return nil
}

// Delete deletes a tenant configuration
func (r *tenantConfigRepository) Delete(ctx context.Context, tenantID string) error {
	ctx, span := observability.StartSpan(ctx, "repository.tenant_config.Delete")
	defer span.End()

	query := `DELETE FROM mcp.tenant_config WHERE tenant_id = $1`

	result, err := r.db.ExecContext(ctx, query, tenantID)
	if err != nil {
		r.logger.Error("Failed to delete tenant config", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		return errors.Wrap(err, "failed to delete tenant config")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tenant config not found for tenant_id: %s", tenantID)
	}

	r.logger.Info("Deleted tenant config", map[string]interface{}{
		"tenant_id": tenantID,
	})

	return nil
}

// Exists checks if a tenant configuration exists
func (r *tenantConfigRepository) Exists(ctx context.Context, tenantID string) (bool, error) {
	ctx, span := observability.StartSpan(ctx, "repository.tenant_config.Exists")
	defer span.End()

	query := `SELECT EXISTS(SELECT 1 FROM mcp.tenant_config WHERE tenant_id = $1)`

	var exists bool
	err := r.db.GetContext(ctx, &exists, query, tenantID)
	if err != nil {
		r.logger.Error("Failed to check tenant config existence", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		return false, errors.Wrap(err, "failed to check tenant config existence")
	}

	return exists, nil
}
