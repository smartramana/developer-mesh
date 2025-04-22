package database

import (
	"context"
	"testing"
	"time"
	
	"github.com/DATA-DOG/go-sqlmock"
	commonConfig "github.com/S-Corkum/mcp-server/internal/common/config"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVectorDatabase_Initialize(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	
	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	
	// Create logger
	logger := observability.NewLogger("test")
	
	// Create config
	cfg := &commonConfig.DatabaseConfig{
		Driver:          "postgres",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		Vector: commonConfig.DatabaseVectorConfig{
			Enabled:         true,
			Dimensions:      1536,
			SimilarityMetric: "cosine",
		},
	}
	
	// Create vector database
	vdb, err := NewVectorDatabase(db, cfg, logger)
	require.NoError(t, err)
	
	// Set up expectations for checking pgvector extension
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	
	// Set up expectations for checking embeddings table
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	
	// Initialize vector database
	err = vdb.Initialize(context.Background())
	assert.NoError(t, err)
	
	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

// Test Initialize when the embeddings table doesn't exist
func TestVectorDatabase_Initialize_CreateTable(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	
	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	
	// Create logger
	logger := observability.NewLogger("test")
	
	// Create config
	cfg := &commonConfig.DatabaseConfig{
		Driver:          "postgres",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		Vector: commonConfig.DatabaseVectorConfig{
			Enabled:         true,
			Dimensions:      1536,
			SimilarityMetric: "cosine",
		},
	}
	
	// Create vector database
	vdb, err := NewVectorDatabase(db, cfg, logger)
	require.NoError(t, err)
	
	// Set up expectations for checking pgvector extension
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	
	// Set up expectations for checking embeddings table (not found)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	
	// Set up expectations for table creation transaction
	mock.ExpectBegin()
	mock.ExpectExec(`CREATE SCHEMA IF NOT EXISTS mcp`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS mcp.embeddings`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`DO \$\$`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	
	// Initialize vector database
	err = vdb.Initialize(context.Background())
	assert.NoError(t, err)
	
	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestVectorDatabase_CheckVectorDimensions(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	
	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	
	// Create logger
	logger := observability.NewLogger("test")
	
	// Create config
	cfg := &commonConfig.DatabaseConfig{
		Driver:          "postgres",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		Vector: commonConfig.DatabaseVectorConfig{
			Enabled:         true,
			Dimensions:      1536,
			SimilarityMetric: "cosine",
		},
	}
	
	// Create vector database
	vdb, err := NewVectorDatabase(db, cfg, logger)
	require.NoError(t, err)
	
	// Set up expectations for checking pgvector extension
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	
	// Set up expectations for checking embeddings table
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	
	// Set up expectations for checking vector dimensions
	mock.ExpectQuery(`SELECT DISTINCT vector_dimensions`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"vector_dimensions"}).
			AddRow(384).
			AddRow(768).
			AddRow(1536))
	
	// Check vector dimensions
	dimensions, err := vdb.CheckVectorDimensions(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, []int{384, 768, 1536}, dimensions)
	
	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestVectorDatabase_Transaction(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	
	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	
	// Create logger
	logger := observability.NewLogger("test")
	
	// Create config
	cfg := &commonConfig.DatabaseConfig{
		Driver:          "postgres",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		Vector: commonConfig.DatabaseVectorConfig{
			Enabled:         true,
			Dimensions:      1536,
			SimilarityMetric: "cosine",
		},
	}
	
	// Create vector database
	vdb, err := NewVectorDatabase(db, cfg, logger)
	require.NoError(t, err)
	
	// Set up expectations for transaction
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectCommit()
	
	// Run transaction
	err = vdb.Transaction(context.Background(), func(tx *sqlx.Tx) error {
		var value int
		return tx.QueryRowContext(context.Background(), "SELECT 1").Scan(&value)
	})
	assert.NoError(t, err)
	
	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

// Test transaction rollback
func TestVectorDatabase_Transaction_Error(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	
	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	
	// Create logger
	logger := observability.NewLogger("test")
	
	// Create config
	cfg := &commonConfig.DatabaseVectorConfig{
		Enabled:         true,
		Dimensions:      1536,
		SimilarityMetric: "cosine",
	}
	
	// Create vector database
	vdb, err := NewVectorDatabase(db, cfg, logger)
	require.NoError(t, err)
	
	// Set up expectations for transaction
	mock.ExpectBegin()
	mock.ExpectRollback()
	
	// Run transaction with error
	err = vdb.Transaction(context.Background(), func(tx *sqlx.Tx) error {
		return assert.AnError
	})
	assert.Error(t, err)
	assert.Equal(t, assert.AnError, err)
	
	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestVectorDatabase_CreateVector(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	
	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	
	// Create logger
	logger := observability.NewLogger("test")
	
	// Create config
	cfg := &commonConfig.DatabaseVectorConfig{
		Enabled:         true,
		Dimensions:      1536,
		SimilarityMetric: "cosine",
	}
	
	// Create vector database
	vdb, err := NewVectorDatabase(db, cfg, logger)
	require.NoError(t, err)
	
	// Set up expectations for checking pgvector extension
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	
	// Set up expectations for checking embeddings table
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	
	// Set up expectations for creating vector
	mock.ExpectQuery(`SELECT \$1::float4\[\]::vector::text`).
		WithArgs("'{1.000000,2.000000,3.000000}'").
		WillReturnRows(sqlmock.NewRows([]string{"vector"}).AddRow("[1.0,2.0,3.0]"))
	
	// Create vector
	vector := []float32{1.0, 2.0, 3.0}
	vectorStr, err := vdb.CreateVector(context.Background(), vector)
	assert.NoError(t, err)
	assert.Equal(t, "[1.0,2.0,3.0]", vectorStr)
	
	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestVectorDatabase_CalculateSimilarity(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	
	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	
	// Create logger
	logger := observability.NewLogger("test")
	
	// Create config
	cfg := &commonConfig.DatabaseVectorConfig{
		Enabled:         true,
		Dimensions:      1536,
		SimilarityMetric: "cosine",
	}
	
	// Create vector database
	vdb, err := NewVectorDatabase(db, cfg, logger)
	require.NoError(t, err)
	
	// Set up expectations for checking pgvector extension
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	
	// Set up expectations for checking embeddings table
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	
	// Set up expectations for creating vectors
	mock.ExpectQuery(`SELECT \$1::float4\[\]::vector::text`).
		WithArgs("'{1.000000,2.000000,3.000000}'").
		WillReturnRows(sqlmock.NewRows([]string{"vector"}).AddRow("[1.0,2.0,3.0]"))
	
	mock.ExpectQuery(`SELECT \$1::float4\[\]::vector::text`).
		WithArgs("'{4.000000,5.000000,6.000000}'").
		WillReturnRows(sqlmock.NewRows([]string{"vector"}).AddRow("[4.0,5.0,6.0]"))
	
	// Set up expectations for calculating similarity
	mock.ExpectQuery(`SELECT 1 - \(\$1::vector <=> \$2::vector\)`).
		WithArgs("[1.0,2.0,3.0]", "[4.0,5.0,6.0]").
		WillReturnRows(sqlmock.NewRows([]string{"similarity"}).AddRow(0.97))
	
	// Calculate similarity
	vector1 := []float32{1.0, 2.0, 3.0}
	vector2 := []float32{4.0, 5.0, 6.0}
	similarity, err := vdb.CalculateSimilarity(context.Background(), vector1, vector2, "cosine")
	assert.NoError(t, err)
	assert.InDelta(t, 0.97, similarity, 0.001)
	
	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

// Test different similarity methods
func TestVectorDatabase_CalculateSimilarity_Methods(t *testing.T) {
	// Create a mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	
	// Create sqlx DB wrapper
	db := sqlx.NewDb(mockDB, "sqlmock")
	
	// Create logger
	logger := observability.NewLogger("test")
	
	// Create config
	cfg := &commonConfig.DatabaseVectorConfig{
		Enabled:         true,
		Dimensions:      3,
		SimilarityMetric: "cosine",
	}
	
	// Create vector database
	vdb, err := NewVectorDatabase(db, cfg, logger)
	require.NoError(t, err)
	
	testCases := []struct {
		name   string
		method string
		query  string
		result float64
	}{
		{
			name:   "Dot product",
			method: "dot",
			query:  `SELECT \$1::vector <#> \$2::vector`,
			result: 32.0,
		},
		{
			name:   "Euclidean distance",
			method: "euclidean",
			query:  `SELECT -\(\$1::vector <-> \$2::vector\)`,
			result: -5.2,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up expectations for checking pgvector extension
			mock.ExpectQuery(`SELECT EXISTS`).
				WithArgs().
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			
			// Set up expectations for checking embeddings table
			mock.ExpectQuery(`SELECT EXISTS`).
				WithArgs().
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			
			// Set up expectations for creating vectors
			mock.ExpectQuery(`SELECT \$1::float4\[\]::vector::text`).
				WithArgs("'{1.000000,2.000000,3.000000}'").
				WillReturnRows(sqlmock.NewRows([]string{"vector"}).AddRow("[1.0,2.0,3.0]"))
			
			mock.ExpectQuery(`SELECT \$1::float4\[\]::vector::text`).
				WithArgs("'{4.000000,5.000000,6.000000}'").
				WillReturnRows(sqlmock.NewRows([]string{"vector"}).AddRow("[4.0,5.0,6.0]"))
			
			// Set up expectations for calculating similarity
			mock.ExpectQuery(tc.query).
				WithArgs("[1.0,2.0,3.0]", "[4.0,5.0,6.0]").
				WillReturnRows(sqlmock.NewRows([]string{"similarity"}).AddRow(tc.result))
			
			// Calculate similarity
			vector1 := []float32{1.0, 2.0, 3.0}
			vector2 := []float32{4.0, 5.0, 6.0}
			similarity, err := vdb.CalculateSimilarity(context.Background(), vector1, vector2, tc.method)
			assert.NoError(t, err)
			assert.InDelta(t, tc.result, similarity, 0.001)
		})
	}
}

func TestNormalizeVector(t *testing.T) {
	testCases := []struct {
		name        string
		vector      []float32
		method      string
		expected    []float32
		expectError bool
	}{
		{
			name:     "Cosine normalization",
			vector:   []float32{1.0, 2.0, 3.0},
			method:   "cosine",
			expected: []float32{0.26726124, 0.5345225, 0.8017837},
		},
		{
			name:     "Dot product normalization (no change)",
			vector:   []float32{1.0, 2.0, 3.0},
			method:   "dot",
			expected: []float32{1.0, 2.0, 3.0},
		},
		{
			name:     "Euclidean normalization (no change)",
			vector:   []float32{1.0, 2.0, 3.0},
			method:   "euclidean",
			expected: []float32{1.0, 2.0, 3.0},
		},
		{
			name:        "Unsupported method",
			vector:      []float32{1.0, 2.0, 3.0},
			method:      "unknown",
			expectError: true,
		},
		{
			name:     "Empty vector",
			vector:   []float32{},
			method:   "cosine",
			expected: []float32{},
		},
		{
			name:     "Zero vector",
			vector:   []float32{0.0, 0.0, 0.0},
			method:   "cosine",
			expected: []float32{0.0, 0.0, 0.0},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			normalized, err := NormalizeVector(tc.vector, tc.method)
			
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				
				if len(tc.expected) > 0 {
					assert.Equal(t, len(tc.expected), len(normalized))
					
					for i := range tc.expected {
						assert.InDelta(t, tc.expected[i], normalized[i], 0.0001)
					}
				} else {
					assert.Empty(t, normalized)
				}
			}
		})
	}
}
