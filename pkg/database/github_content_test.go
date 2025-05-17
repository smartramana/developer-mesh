package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/storage"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreGitHubContent(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	database := &Database{db: db}

	// Create test context
	ctx := context.Background()

	// Create test metadata
	now := time.Now().UTC()
	expires := now.Add(24 * time.Hour)
	testMetadata := &storage.ContentMetadata{
		Owner:       "owner",
		Repo:        "repo",
		ContentType: storage.ContentTypeIssue,
		ContentID:   "123",
		Checksum:    "9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13",
		URI:         "s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13",
		Size:        100,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   &expires,
		Metadata:    map[string]interface{}{"test": "value"},
	}

	// Test successful storage
	// Set up expectations
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO mcp.github_content_metadata").
		WithArgs(
			"gh-owner-repo-issue-123", // ID
			"owner",                   // Owner
			"repo",                    // Repo
			"issue",                   // ContentType
			"123",                     // ContentID
			"9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", // Checksum
			"s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", // URI
			int64(100),                // Size
			now,                       // CreatedAt
			now,                       // UpdatedAt
			expires,                   // ExpiresAt
			sqlmock.AnyArg(),          // Metadata JSON
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Call the method
	err = database.StoreGitHubContent(ctx, testMetadata)

	// Verify results
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())

	// Test with nil metadata (should use empty JSON object)
	testMetadata.Metadata = nil

	// Set up expectations
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO mcp.github_content_metadata").
		WithArgs(
			"gh-owner-repo-issue-123", // ID
			"owner",                   // Owner
			"repo",                    // Repo
			"issue",                   // ContentType
			"123",                     // ContentID
			"9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", // Checksum
			"s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", // URI
			int64(100),                // Size
			now,                       // CreatedAt
			now,                       // UpdatedAt
			expires,                   // ExpiresAt
			[]byte("{}"),              // Empty JSON object
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Call the method
	err = database.StoreGitHubContent(ctx, testMetadata)

	// Verify results
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())

	// Test with nil ExpiresAt
	testMetadata.ExpiresAt = nil

	// Set up expectations
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO mcp.github_content_metadata").
		WithArgs(
			"gh-owner-repo-issue-123", // ID
			"owner",                   // Owner
			"repo",                    // Repo
			"issue",                   // ContentType
			"123",                     // ContentID
			"9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", // Checksum
			"s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", // URI
			int64(100),                // Size
			now,                       // CreatedAt
			now,                       // UpdatedAt
			sql.NullTime{Valid: false}, // Nil ExpiresAt
			[]byte("{}"),              // Empty JSON object
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Call the method
	err = database.StoreGitHubContent(ctx, testMetadata)

	// Verify results
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())

	// Test transaction error
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO mcp.github_content_metadata").
		WithArgs(
			"gh-owner-repo-issue-123", // ID
			"owner",                   // Owner
			"repo",                    // Repo
			"issue",                   // ContentType
			"123",                     // ContentID
			"9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", // Checksum
			"s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", // URI
			int64(100),                // Size
			now,                       // CreatedAt
			now,                       // UpdatedAt
			sql.NullTime{Valid: false}, // Nil ExpiresAt
			[]byte("{}"),              // Empty JSON object
		).
		WillReturnError(sql.ErrTxDone)
	mock.ExpectRollback()

	// Call the method
	err = database.StoreGitHubContent(ctx, testMetadata)

	// Verify results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to insert GitHub content metadata")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetGitHubContent(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	database := &Database{db: db}

	// Create test context
	ctx := context.Background()

	// Test successful retrieval
	now := time.Now().UTC()
	expires := now.Add(24 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"id", "owner", "repo", "content_type", "content_id", "checksum", "uri", 
		"size", "created_at", "updated_at", "expires_at", "metadata"}).
		AddRow(
			"gh-owner-repo-issue-123", // ID
			"owner",                   // Owner
			"repo",                    // Repo
			"issue",                   // ContentType
			"123",                     // ContentID
			"9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", // Checksum
			"s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", // URI
			int64(100),                // Size
			now,                       // CreatedAt
			now,                       // UpdatedAt
			expires,                   // ExpiresAt
			[]byte(`{"test":"value"}`), // Metadata
		)

	// Set up expectations
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT (.+) FROM mcp.github_content_metadata WHERE owner = (.+) AND repo = (.+) AND content_type = (.+) AND content_id = (.+)").
		WithArgs("owner", "repo", "issue", "123").
		WillReturnRows(rows)
	mock.ExpectCommit()

	// Call the method
	metadata, err := database.GetGitHubContent(ctx, "owner", "repo", "issue", "123")

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "owner", metadata.Owner)
	assert.Equal(t, "repo", metadata.Repo)
	assert.Equal(t, storage.ContentTypeIssue, metadata.ContentType)
	assert.Equal(t, "123", metadata.ContentID)
	assert.Equal(t, "9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", metadata.Checksum)
	assert.Equal(t, "s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", metadata.URI)
	assert.Equal(t, int64(100), metadata.Size)
	assert.Equal(t, now.Format(time.RFC3339), metadata.CreatedAt.Format(time.RFC3339))
	assert.Equal(t, now.Format(time.RFC3339), metadata.UpdatedAt.Format(time.RFC3339))
	assert.NotNil(t, metadata.ExpiresAt)
	assert.Equal(t, expires.Format(time.RFC3339), metadata.ExpiresAt.Format(time.RFC3339))
	assert.Equal(t, "value", metadata.Metadata["test"])
	assert.NoError(t, mock.ExpectationsWereMet())

	// Test not found
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT (.+) FROM mcp.github_content_metadata WHERE owner = (.+) AND repo = (.+) AND content_type = (.+) AND content_id = (.+)").
		WithArgs("owner", "repo", "issue", "123").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	// Call the method
	metadata, err = database.GetGitHubContent(ctx, "owner", "repo", "issue", "123")

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "GitHub content metadata not found")
	assert.NoError(t, mock.ExpectationsWereMet())

	// Test other database error
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT (.+) FROM mcp.github_content_metadata WHERE owner = (.+) AND repo = (.+) AND content_type = (.+) AND content_id = (.+)").
		WithArgs("owner", "repo", "issue", "123").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	// Call the method
	metadata, err = database.GetGitHubContent(ctx, "owner", "repo", "issue", "123")

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "failed to get GitHub content metadata")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteGitHubContent(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	database := &Database{db: db}

	// Create test context
	ctx := context.Background()

	// Test successful deletion
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM mcp.github_content_metadata WHERE owner = (.+) AND repo = (.+) AND content_type = (.+) AND content_id = (.+)").
		WithArgs("owner", "repo", "issue", "123").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// Call the method
	err = database.DeleteGitHubContent(ctx, "owner", "repo", "issue", "123")

	// Verify results
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())

	// Test deletion error
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM mcp.github_content_metadata WHERE owner = (.+) AND repo = (.+) AND content_type = (.+) AND content_id = (.+)").
		WithArgs("owner", "repo", "issue", "123").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	// Call the method
	err = database.DeleteGitHubContent(ctx, "owner", "repo", "issue", "123")

	// Verify results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete GitHub content metadata")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListGitHubContent(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	database := &Database{db: db}

	// Create test context
	ctx := context.Background()

	// Test successful listing
	now := time.Now().UTC()
	expires := now.Add(24 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"id", "owner", "repo", "content_type", "content_id", "checksum", "uri", 
		"size", "created_at", "updated_at", "expires_at", "metadata"}).
		AddRow(
			"gh-owner-repo-issue-123", // ID
			"owner",                   // Owner
			"repo",                    // Repo
			"issue",                   // ContentType
			"123",                     // ContentID
			"hash1",                   // Checksum
			"s3://test-bucket/content/hash1", // URI
			int64(100),                // Size
			now,                       // CreatedAt
			now,                       // UpdatedAt
			expires,                   // ExpiresAt
			[]byte(`{"test":"value1"}`), // Metadata
		).
		AddRow(
			"gh-owner-repo-issue-456", // ID
			"owner",                   // Owner
			"repo",                    // Repo
			"issue",                   // ContentType
			"456",                     // ContentID
			"hash2",                   // // Checksum
			"s3://test-bucket/content/hash2", // URI
			int64(200),                // Size
			now,                       // CreatedAt
			now,                       // UpdatedAt
			expires,                   // ExpiresAt
			[]byte(`{"test":"value2"}`), // Metadata
		)

	// Set up expectations for listing with content type
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT (.+) FROM mcp.github_content_metadata WHERE owner = (.+) AND repo = (.+) AND content_type = (.+)").
		WithArgs("owner", "repo", "issue").
		WillReturnRows(rows)
	mock.ExpectCommit()

	// Call the method
	metadata, err := database.ListGitHubContent(ctx, "owner", "repo", "issue", 0)

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Len(t, metadata, 2)
	assert.Equal(t, "123", metadata[0].ContentID)
	assert.Equal(t, "456", metadata[1].ContentID)
	assert.NoError(t, mock.ExpectationsWereMet())

	// Test listing with limit
	mock.ExpectBegin()
	// Create a new row with only one item for the limit test
	limitedRows := sqlmock.NewRows([]string{"id", "owner", "repo", "content_type", "content_id", "checksum", "uri", "size", "created_at", "updated_at", "expires_at", "metadata"}).
		AddRow("gh-owner-repo-issue-123", "owner", "repo", "issue", "123", "hash1", "s3://test-bucket/content/hash1", int64(100), now, now, expires, []byte(`{"test":"value1"}`)) 
	mock.ExpectQuery("SELECT (.+) FROM mcp.github_content_metadata WHERE owner = (.+) AND repo = (.+) AND content_type = (.+) ORDER BY updated_at DESC LIMIT (.+)").
		WithArgs("owner", "repo", "issue", 1).
		WillReturnRows(limitedRows)
	mock.ExpectCommit()

	// Call the method
	metadata, err = database.ListGitHubContent(ctx, "owner", "repo", "issue", 1)

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Len(t, metadata, 1)
	assert.NoError(t, mock.ExpectationsWereMet())

	// Test listing without content type
	// We need to recreate the rows since the previous query consumed them
	rows = sqlmock.NewRows([]string{"id", "owner", "repo", "content_type", "content_id", "checksum", "uri", "size", "created_at", "updated_at", "expires_at", "metadata"})
	rows.AddRow("gh-owner-repo-issue-123", "owner", "repo", "issue", "123", "hash1", "s3://test-bucket/content/hash1", int64(100), now, now, expires, []byte(`{"test":"value1"}`)) 
	rows.AddRow("gh-owner-repo-issue-456", "owner", "repo", "issue", "456", "hash2", "s3://test-bucket/content/hash2", int64(200), now, now, expires, []byte(`{"test":"value2"}`))

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT (.+) FROM mcp.github_content_metadata WHERE owner = (.+) AND repo = (.+) ORDER BY updated_at DESC").
		WithArgs("owner", "repo").
		WillReturnRows(rows)
	mock.ExpectCommit()

	// Call the method
	metadata, err = database.ListGitHubContent(ctx, "owner", "repo", "", 0)

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Len(t, metadata, 2)
	assert.NoError(t, mock.ExpectationsWereMet())

	// Test database error
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT (.+) FROM mcp.github_content_metadata WHERE owner = (.+) AND repo = (.+) AND content_type = (.+)").
		WithArgs("owner", "repo", "issue").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	// Call the method
	metadata, err = database.ListGitHubContent(ctx, "owner", "repo", "issue", 0)

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "failed to query GitHub content metadata")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureGitHubContentTables(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	database := &Database{db: db}

	// Create test context
	ctx := context.Background()

	// Test successful table creation
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS mcp.github_content_metadata").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Call the method
	err = database.ensureGitHubContentTables(ctx)

	// Verify results
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())

	// Test table creation error
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS mcp.github_content_metadata").
		WillReturnError(sql.ErrConnDone)

	// Call the method
	err = database.ensureGitHubContentTables(ctx)

	// Verify results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create github_content_metadata table")
	assert.NoError(t, mock.ExpectationsWereMet())

	// Test index creation error
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS mcp.github_content_metadata").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS").
		WillReturnError(sql.ErrConnDone)

	// Call the method
	err = database.ensureGitHubContentTables(ctx)

	// Verify results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create indexes")
	assert.NoError(t, mock.ExpectationsWereMet())
}
