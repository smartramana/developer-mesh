package integration

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/database"
	"github.com/S-Corkum/devops-mcp/internal/models"
	"github.com/S-Corkum/devops-mcp/internal/relationship"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRelationshipServiceIntegration tests the relationship service against a real database
func TestRelationshipServiceIntegration(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Connect to test database
	dbConnStr := os.Getenv("TEST_DB_CONNECTION")
	if dbConnStr == "" {
		dbConnStr = "postgres://postgres:postgres@localhost:5432/testdb?sslmode=disable"
	}

	// Create the database connection
	sqlDB, err := sql.Open("postgres", dbConnStr)
	require.NoError(t, err, "Failed to open database connection")
	defer sqlDB.Close()

	// Create the sqlx database
	sqlxDB := sqlx.NewDb(sqlDB, "postgres")
	require.NoError(t, err, "Failed to create sqlx database")

	// Create the database instance
	db := database.NewDatabaseWithConnection(sqlxDB)
	require.NotNil(t, db, "Failed to create database instance")

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Initialize database tables
	err = db.InitializeTables(ctx)
	require.NoError(t, err, "Failed to initialize database tables")

	// Clean up any existing test data
	cleanupTestData(t, db)

	// Create a repository
	repo := database.NewRelationshipRepository(db)
	require.NotNil(t, repo, "Failed to create relationship repository")

	// Create a service
	service := relationship.NewService(repo)
	require.NotNil(t, service, "Failed to create relationship service")

	// Create test entities
	issueEntity := models.NewEntityID(models.EntityTypeIssue, "test-owner", "test-repo", "1")
	prEntity := models.NewEntityID(models.EntityTypePullRequest, "test-owner", "test-repo", "2")
	commitEntity := models.NewEntityID(models.EntityTypeCommit, "test-owner", "test-repo", "abc123")
	fileEntity := models.NewEntityID(models.EntityTypeFile, "test-owner", "test-repo", "path/to/file.go")

	// Test: Create relationships
	t.Run("CreateRelationship", func(t *testing.T) {
		// Create a relationship between an issue and a PR
		rel := models.NewEntityRelationship(
			models.RelationshipTypeReferences,
			issueEntity,
			prEntity,
			models.DirectionOutgoing,
			0.8,
		).WithContext("Issue references PR in its description")

		err := service.CreateRelationship(ctx, rel)
		assert.NoError(t, err, "Failed to create relationship")

		// Create another relationship
		rel2 := models.NewEntityRelationship(
			models.RelationshipTypeModifies,
			prEntity,
			fileEntity,
			models.DirectionOutgoing,
			1.0,
		).WithContext("PR modifies this file")

		err = service.CreateRelationship(ctx, rel2)
		assert.NoError(t, err, "Failed to create second relationship")

		// Create a bidirectional relationship
		err = service.CreateBidirectionalRelationship(
			ctx,
			models.RelationshipTypeAssociates,
			prEntity,
			commitEntity,
			0.9,
			map[string]interface{}{"commit_sha": "abc123"},
		)
		assert.NoError(t, err, "Failed to create bidirectional relationship")
	})

	// Test: Get relationship by ID
	t.Run("GetRelationship", func(t *testing.T) {
		// Generate the relationship ID
		relID := models.GenerateRelationshipID(
			models.RelationshipTypeReferences,
			issueEntity,
			prEntity,
			models.DirectionOutgoing,
		)

		// Retrieve the relationship
		rel, err := service.GetRelationship(ctx, relID)
		assert.NoError(t, err, "Failed to get relationship")
		assert.NotNil(t, rel, "Relationship should not be nil")
		assert.Equal(t, models.RelationshipTypeReferences, rel.Type)
		assert.Equal(t, issueEntity.ID, rel.Source.ID)
		assert.Equal(t, prEntity.ID, rel.Target.ID)
		assert.Equal(t, "Issue references PR in its description", rel.Context)
	})

	// Test: Get direct relationships
	t.Run("GetDirectRelationships", func(t *testing.T) {
		// Get outgoing relationships for the PR
		rels, err := service.GetDirectRelationships(
			ctx,
			prEntity,
			models.DirectionOutgoing,
			nil,
		)
		assert.NoError(t, err, "Failed to get direct relationships")
		assert.Len(t, rels, 2, "PR should have 2 outgoing relationships")

		// Check relationship types
		hasModifies := false
		hasAssociates := false
		for _, rel := range rels {
			if rel.Type == models.RelationshipTypeModifies {
				hasModifies = true
			}
			if rel.Type == models.RelationshipTypeAssociates {
				hasAssociates = true
			}
		}
		assert.True(t, hasModifies, "Should have modifies relationship")
		assert.True(t, hasAssociates, "Should have associates relationship")

		// Get incoming relationships for the PR
		rels, err = service.GetDirectRelationships(
			ctx,
			prEntity,
			models.DirectionIncoming,
			nil,
		)
		assert.NoError(t, err, "Failed to get incoming relationships")
		assert.Len(t, rels, 1, "PR should have 1 incoming relationship")
		assert.Equal(t, models.RelationshipTypeReferences, rels[0].Type)

		// Get bidirectional relationships for the PR
		rels, err = service.GetDirectRelationships(
			ctx,
			prEntity,
			models.DirectionBidirectional,
			nil,
		)
		assert.NoError(t, err, "Failed to get bidirectional relationships")
		assert.Len(t, rels, 3, "PR should have 3 total relationships")
	})

	// Test: Get related entities
	t.Run("GetRelatedEntities", func(t *testing.T) {
		// Get entities related to the PR
		entities, err := service.GetRelatedEntities(
			ctx,
			prEntity,
			nil,
			1,
		)
		assert.NoError(t, err, "Failed to get related entities")
		assert.Len(t, entities, 3, "PR should be related to 3 entities")

		// Check entity types
		hasIssue := false
		hasCommit := false
		hasFile := false
		for _, entity := range entities {
			if entity.Type == models.EntityTypeIssue {
				hasIssue = true
			}
			if entity.Type == models.EntityTypeCommit {
				hasCommit = true
			}
			if entity.Type == models.EntityTypeFile {
				hasFile = true
			}
		}
		assert.True(t, hasIssue, "Should be related to issue")
		assert.True(t, hasCommit, "Should be related to commit")
		assert.True(t, hasFile, "Should be related to file")
	})

	// Test: Get relationship graph
	t.Run("GetRelationshipGraph", func(t *testing.T) {
		// Create additional relationships to test graph traversal
		fileToCommitRel := models.NewEntityRelationship(
			models.RelationshipTypeContains,
			commitEntity,
			fileEntity,
			models.DirectionOutgoing,
			0.7,
		)
		err := service.CreateRelationship(ctx, fileToCommitRel)
		assert.NoError(t, err, "Failed to create file-to-commit relationship")

		// Get the relationship graph starting from issue with depth 2
		graph, err := service.GetRelationshipGraph(
			ctx,
			issueEntity,
			2,
		)
		assert.NoError(t, err, "Failed to get relationship graph")
		assert.GreaterOrEqual(t, len(graph), 4, "Should have at least 4 relationships in the graph")
	})

	// Test: Delete relationships
	t.Run("DeleteRelationship", func(t *testing.T) {
		// Delete the issue-to-PR relationship
		relID := models.GenerateRelationshipID(
			models.RelationshipTypeReferences,
			issueEntity,
			prEntity,
			models.DirectionOutgoing,
		)
		err := service.DeleteRelationship(ctx, relID)
		assert.NoError(t, err, "Failed to delete relationship")

		// Verify it's deleted
		_, err = service.GetRelationship(ctx, relID)
		assert.Error(t, err, "Relationship should be deleted")

		// Delete all relationships between PR and commit
		err = service.DeleteRelationshipsBetween(
			ctx,
			prEntity,
			commitEntity,
		)
		assert.NoError(t, err, "Failed to delete relationships between PR and commit")

		// Verify deletion
		rels, err := service.GetDirectRelationships(
			ctx,
			prEntity,
			models.DirectionBidirectional,
			nil,
		)
		assert.NoError(t, err, "Failed to get relationships")
		assert.Len(t, rels, 1, "Should only have 1 relationship left")
	})
}

// cleanupTestData removes any test data from previous test runs
func cleanupTestData(t *testing.T, db *database.Database) {
	ctx := context.Background()
	_, err := db.GetDB().ExecContext(ctx, `
		DELETE FROM mcp.entity_relationships 
		WHERE source_owner = 'test-owner' AND source_repo = 'test-repo'
	`)
	assert.NoError(t, err, "Failed to cleanup test data")

	_, err = db.GetDB().ExecContext(ctx, `
		DELETE FROM mcp.entity_relationships 
		WHERE target_owner = 'test-owner' AND target_repo = 'test-repo'
	`)
	assert.NoError(t, err, "Failed to cleanup test data")
}
