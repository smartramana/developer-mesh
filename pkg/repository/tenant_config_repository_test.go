package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTenantConfigRepository(t *testing.T) {
	db := &sqlx.DB{}
	logger := observability.NewLogger("test")

	repo := NewTenantConfigRepository(db, logger)
	assert.NotNil(t, repo)
	assert.IsType(t, &tenantConfigRepository{}, repo)
}

func TestTenantConfigRepository_GetByTenantID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")
	logger := observability.NewLogger("test")
	repo := NewTenantConfigRepository(sqlxDB, logger)

	ctx := context.Background()
	tenantID := "test-tenant-123"
	configID := uuid.New().String()

	rateLimitConfig := models.RateLimitConfig{
		DefaultRequestsPerMinute: 100,
		DefaultRequestsPerHour:   5000,
		DefaultRequestsPerDay:    50000,
		KeyTypeOverrides: map[string]models.KeyTypeRateLimit{
			"admin": {
				RequestsPerMinute: 1000,
				RequestsPerHour:   50000,
				RequestsPerDay:    500000,
			},
		},
		EndpointOverrides: map[string]models.EndpointRateLimit{
			"/api/v1/expensive": {
				RequestsPerMinute: 10,
				BurstSize:         20,
			},
		},
	}

	rateLimitJSON, _ := json.Marshal(rateLimitConfig)
	encryptedTokens := json.RawMessage(`{"encrypted": "data"}`)
	featuresJSON := json.RawMessage(`{"feature1": true, "feature2": "enabled"}`)
	allowedOrigins := pq.StringArray{"https://app.example.com"}
	createdAt := time.Now()
	updatedAt := time.Now()

	t.Run("successful retrieval", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "tenant_id", "rate_limit_config", "service_tokens",
			"allowed_origins", "features", "created_at", "updated_at",
		}).AddRow(
			configID, tenantID, rateLimitJSON, encryptedTokens,
			allowedOrigins, featuresJSON, createdAt, updatedAt,
		)

		mock.ExpectQuery("SELECT (.+) FROM mcp.tenant_config WHERE tenant_id = \\$1").
			WithArgs(tenantID).
			WillReturnRows(rows)

		config, err := repo.GetByTenantID(ctx, tenantID)
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, configID, config.ID)
		assert.Equal(t, tenantID, config.TenantID)
		assert.Equal(t, 100, config.RateLimitConfig.DefaultRequestsPerMinute)
		assert.Equal(t, 1000, config.RateLimitConfig.KeyTypeOverrides["admin"].RequestsPerMinute)
		assert.Equal(t, encryptedTokens, config.EncryptedTokens)
		assert.Equal(t, allowedOrigins, config.AllowedOrigins)
		assert.NotNil(t, config.Features)
		assert.Equal(t, true, config.Features["feature1"])
		assert.Equal(t, "enabled", config.Features["feature2"])

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM mcp.tenant_config WHERE tenant_id = \\$1").
			WithArgs(tenantID).
			WillReturnError(sql.ErrNoRows)

		config, err := repo.GetByTenantID(ctx, tenantID)
		assert.NoError(t, err)
		assert.Nil(t, config)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM mcp.tenant_config WHERE tenant_id = \\$1").
			WithArgs(tenantID).
			WillReturnError(sql.ErrConnDone)

		config, err := repo.GetByTenantID(ctx, tenantID)
		assert.Error(t, err)
		assert.Nil(t, config)

		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTenantConfigRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")
	logger := observability.NewLogger("test")
	repo := NewTenantConfigRepository(sqlxDB, logger)

	ctx := context.Background()

	t.Run("successful creation with ID", func(t *testing.T) {
		config := &models.TenantConfig{
			ID:       uuid.New().String(),
			TenantID: "test-tenant-123",
			RateLimitConfig: models.RateLimitConfig{
				DefaultRequestsPerMinute: 100,
				DefaultRequestsPerHour:   5000,
				DefaultRequestsPerDay:    50000,
			},
			EncryptedTokens: json.RawMessage(`{"encrypted": "data"}`),
			AllowedOrigins:  pq.StringArray{"https://app.example.com"},
			Features: map[string]interface{}{
				"feature1": true,
			},
		}

		featuresJSON, _ := json.Marshal(config.Features)

		mock.ExpectExec("INSERT INTO mcp.tenant_config").
			WithArgs(
				config.ID,
				config.TenantID,
				sqlmock.AnyArg(), // rate_limit_config (JSON)
				config.EncryptedTokens,
				config.AllowedOrigins,
				featuresJSON,
				sqlmock.AnyArg(), // created_at
				sqlmock.AnyArg(), // updated_at
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.Create(ctx, config)
		assert.NoError(t, err)
		assert.NotEmpty(t, config.ID)
		assert.False(t, config.CreatedAt.IsZero())
		assert.False(t, config.UpdatedAt.IsZero())

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("successful creation without ID", func(t *testing.T) {
		config := &models.TenantConfig{
			TenantID: "test-tenant-456",
			Features: map[string]interface{}{},
		}

		featuresJSON, _ := json.Marshal(config.Features)

		mock.ExpectExec("INSERT INTO mcp.tenant_config").
			WithArgs(
				sqlmock.AnyArg(), // generated ID
				config.TenantID,
				sqlmock.AnyArg(), // rate_limit_config (JSON)
				sqlmock.AnyArg(), // encrypted_tokens
				sqlmock.AnyArg(), // allowed_origins
				featuresJSON,
				sqlmock.AnyArg(), // created_at
				sqlmock.AnyArg(), // updated_at
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.Create(ctx, config)
		assert.NoError(t, err)
		assert.NotEmpty(t, config.ID)
		assert.Equal(t, 60, config.RateLimitConfig.DefaultRequestsPerMinute) // defaults applied

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		config := &models.TenantConfig{
			TenantID: "test-tenant-789",
			Features: map[string]interface{}{},
		}

		mock.ExpectExec("INSERT INTO mcp.tenant_config").
			WillReturnError(sql.ErrConnDone)

		err := repo.Create(ctx, config)
		assert.Error(t, err)

		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTenantConfigRepository_Update(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")
	logger := observability.NewLogger("test")
	repo := NewTenantConfigRepository(sqlxDB, logger)

	ctx := context.Background()

	config := &models.TenantConfig{
		TenantID: "test-tenant-123",
		RateLimitConfig: models.RateLimitConfig{
			DefaultRequestsPerMinute: 200,
		},
		EncryptedTokens: json.RawMessage(`{"encrypted": "updated"}`),
		AllowedOrigins:  pq.StringArray{"https://new.example.com"},
		Features: map[string]interface{}{
			"new_feature": true,
		},
	}

	t.Run("successful update", func(t *testing.T) {
		featuresJSON, _ := json.Marshal(config.Features)

		mock.ExpectExec("UPDATE mcp.tenant_config SET").
			WithArgs(
				config.TenantID,
				sqlmock.AnyArg(), // rate_limit_config (JSON)
				config.EncryptedTokens,
				config.AllowedOrigins,
				featuresJSON,
				sqlmock.AnyArg(), // updated_at
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.Update(ctx, config)
		assert.NoError(t, err)
		assert.False(t, config.UpdatedAt.IsZero())

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		featuresJSON, _ := json.Marshal(config.Features)

		mock.ExpectExec("UPDATE mcp.tenant_config SET").
			WithArgs(
				config.TenantID,
				sqlmock.AnyArg(),
				config.EncryptedTokens,
				config.AllowedOrigins,
				featuresJSON,
				sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.Update(ctx, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectExec("UPDATE mcp.tenant_config SET").
			WillReturnError(sql.ErrConnDone)

		err := repo.Update(ctx, config)
		assert.Error(t, err)

		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTenantConfigRepository_Delete(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")
	logger := observability.NewLogger("test")
	repo := NewTenantConfigRepository(sqlxDB, logger)

	ctx := context.Background()
	tenantID := "test-tenant-123"

	t.Run("successful deletion", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM mcp.tenant_config WHERE tenant_id = \\$1").
			WithArgs(tenantID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.Delete(ctx, tenantID)
		assert.NoError(t, err)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM mcp.tenant_config WHERE tenant_id = \\$1").
			WithArgs(tenantID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.Delete(ctx, tenantID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM mcp.tenant_config WHERE tenant_id = \\$1").
			WithArgs(tenantID).
			WillReturnError(sql.ErrConnDone)

		err := repo.Delete(ctx, tenantID)
		assert.Error(t, err)

		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTenantConfigRepository_Exists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sqlxDB := sqlx.NewDb(db, "postgres")
	logger := observability.NewLogger("test")
	repo := NewTenantConfigRepository(sqlxDB, logger)

	ctx := context.Background()
	tenantID := "test-tenant-123"

	t.Run("exists", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"exists"}).AddRow(true)

		mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM mcp.tenant_config WHERE tenant_id = \\$1\\)").
			WithArgs(tenantID).
			WillReturnRows(rows)

		exists, err := repo.Exists(ctx, tenantID)
		assert.NoError(t, err)
		assert.True(t, exists)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("does not exist", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"exists"}).AddRow(false)

		mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM mcp.tenant_config WHERE tenant_id = \\$1\\)").
			WithArgs(tenantID).
			WillReturnRows(rows)

		exists, err := repo.Exists(ctx, tenantID)
		assert.NoError(t, err)
		assert.False(t, exists)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM mcp.tenant_config WHERE tenant_id = \\$1\\)").
			WithArgs(tenantID).
			WillReturnError(sql.ErrConnDone)

		exists, err := repo.Exists(ctx, tenantID)
		assert.Error(t, err)
		assert.False(t, exists)

		require.NoError(t, mock.ExpectationsWereMet())
	})
}
