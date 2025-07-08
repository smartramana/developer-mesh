package e2e

import (
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestE2ESetup verifies that the E2E test API key is properly configured in the database
func TestE2ESetup(t *testing.T) {
	// Skip in CI if no database URL is provided
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping database setup test")
	}

	// Connect to database
	db, err := sqlx.Connect("postgres", databaseURL)
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()

	// Get expected API key prefix from environment or config
	expectedKeyPrefix := os.Getenv("E2E_API_KEY_PREFIX")
	if expectedKeyPrefix == "" {
		// Default to the prefix used in setup_e2e_api_key.sh
		expectedKeyPrefix = "cacacb6b"
	}

	// Verify API key exists in database
	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1 FROM mcp.api_keys 
			WHERE key_prefix = $1
			AND key_type = 'admin'
			AND is_active = true
		)
	`
	err = db.Get(&exists, query, expectedKeyPrefix)
	require.NoError(t, err, "Failed to query API key existence")
	require.True(t, exists, "E2E API key not found in database with prefix: %s", expectedKeyPrefix)

	// Verify tenant exists
	var tenantExists bool
	tenantQuery := `
		SELECT EXISTS(
			SELECT 1 FROM mcp.tenants 
			WHERE id = (
				SELECT tenant_id FROM mcp.api_keys 
				WHERE key_prefix = $1
			)
			AND name = 'E2E Test Tenant'
		)
	`
	err = db.Get(&tenantExists, tenantQuery, expectedKeyPrefix)
	require.NoError(t, err, "Failed to query tenant existence")
	require.True(t, tenantExists, "E2E test tenant not found or misconfigured")

	// Verify API key has proper configuration
	type APIKeyInfo struct {
		KeyType                string         `db:"key_type"`
		TenantID               string         `db:"tenant_id"`
		Name                   string         `db:"name"`
		IsActive               bool           `db:"is_active"`
		Scopes                 sql.NullString `db:"scopes"`
		RateLimitRequests      int            `db:"rate_limit_requests"`
		RateLimitWindowSeconds int            `db:"rate_limit_window_seconds"`
	}

	var keyInfo APIKeyInfo
	infoQuery := `
		SELECT 
			key_type,
			tenant_id,
			name,
			is_active,
			array_to_string(scopes, ',') as scopes,
			rate_limit_requests,
			rate_limit_window_seconds
		FROM mcp.api_keys 
		WHERE key_prefix = $1
	`
	err = db.Get(&keyInfo, infoQuery, expectedKeyPrefix)
	require.NoError(t, err, "Failed to get API key info")

	// Verify key configuration
	require.Equal(t, "admin", keyInfo.KeyType, "API key should be admin type")
	require.Equal(t, "E2E Test Admin Key", keyInfo.Name)
	require.True(t, keyInfo.IsActive, "API key should be active")
	require.Equal(t, 1000, keyInfo.RateLimitRequests, "Admin key should have high rate limit")
	require.Equal(t, 60, keyInfo.RateLimitWindowSeconds, "Rate limit window should be 60 seconds")

	// Log success
	t.Logf("E2E setup verified: API key '%s' exists with proper configuration", expectedKeyPrefix)
}

// TestE2ETenantConfiguration verifies tenant-specific configuration
func TestE2ETenantConfiguration(t *testing.T) {
	// Skip in CI if no database URL is provided
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping tenant configuration test")
	}

	// Connect to database
	db, err := sqlx.Connect("postgres", databaseURL)
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()

	// Check if tenant config exists
	var configExists bool
	configQuery := `
		SELECT EXISTS(
			SELECT 1 FROM mcp.tenant_config tc
			JOIN mcp.tenants t ON t.id = tc.tenant_id
			WHERE t.name = 'E2E Test Tenant'
		)
	`
	err = db.Get(&configExists, configQuery)
	require.NoError(t, err, "Failed to query tenant config existence")

	if configExists {
		// Verify tenant config if it exists
		type TenantConfig struct {
			RateLimitConfig json.RawMessage `db:"rate_limit_config"`
			AllowedOrigins  pq.StringArray  `db:"allowed_origins"`
			Features        json.RawMessage `db:"features"`
		}

		var config TenantConfig
		getConfigQuery := `
			SELECT 
				tc.rate_limit_config,
				tc.allowed_origins,
				tc.features
			FROM mcp.tenant_config tc
			JOIN mcp.tenants t ON t.id = tc.tenant_id
			WHERE t.name = 'E2E Test Tenant'
		`
		err = db.Get(&config, getConfigQuery)
		require.NoError(t, err, "Failed to get tenant config")

		t.Logf("E2E tenant configuration found with %d allowed origins", len(config.AllowedOrigins))
	} else {
		t.Log("No tenant configuration found (using defaults)")
	}
}

// TestMultipleAPIKeyTypes verifies different API key types are supported
func TestMultipleAPIKeyTypes(t *testing.T) {
	// Skip in CI if no database URL is provided
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping API key types test")
	}

	// Connect to database
	db, err := sqlx.Connect("postgres", databaseURL)
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()

	// Check what key types exist in the database
	type KeyTypeCount struct {
		KeyType string `db:"key_type"`
		Count   int    `db:"count"`
	}

	var keyTypes []KeyTypeCount
	keyTypesQuery := `
		SELECT key_type, COUNT(*) as count
		FROM mcp.api_keys
		WHERE is_active = true
		GROUP BY key_type
		ORDER BY key_type
	`
	err = db.Select(&keyTypes, keyTypesQuery)
	require.NoError(t, err, "Failed to query key types")

	// Log the key types found
	t.Log("Active API key types in database:")
	for _, kt := range keyTypes {
		t.Logf("  %s: %d keys", kt.KeyType, kt.Count)
	}

	// Verify at least one admin key exists (for E2E tests)
	adminFound := false
	for _, kt := range keyTypes {
		if kt.KeyType == "admin" && kt.Count > 0 {
			adminFound = true
			break
		}
	}
	require.True(t, adminFound, "No admin API keys found in database")
}
