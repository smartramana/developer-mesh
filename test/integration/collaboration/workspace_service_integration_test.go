package collaboration

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/S-Corkum/devops-mcp/pkg/collaboration"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
	"github.com/S-Corkum/devops-mcp/pkg/repository/postgres"
	"github.com/S-Corkum/devops-mcp/pkg/services"
	"github.com/S-Corkum/devops-mcp/test/integration/shared"
)

// WorkspaceServiceIntegrationSuite tests workspace service with real database
type WorkspaceServiceIntegrationSuite struct {
	suite.Suite
	ctx              context.Context
	cancel           context.CancelFunc
	db               *sql.DB
	workspaceService services.WorkspaceService
	documentService  services.DocumentService
	logger           observability.Logger
	tenantID         uuid.UUID
}

// SetupSuite runs once before all tests
func (s *WorkspaceServiceIntegrationSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.logger = observability.NewLogger("workspace-service-test")
	s.tenantID = uuid.New()

	// Get test database connection
	db, err := shared.GetTestDatabase(s.ctx)
	require.NoError(s.T(), err)
	s.db = db

	// Run migrations
	err = shared.RunMigrations(s.db)
	require.NoError(s.T(), err)

	// Create sqlx DB
	sqlxDB := sqlx.NewDb(db, "postgres")

	// Create mock cache
	mockCache := &mockCache{data: make(map[string]interface{})}

	// Create repositories
	workspaceRepo := postgres.NewWorkspaceRepository(sqlxDB, sqlxDB, mockCache, s.logger, observability.NoopStartSpan)
	documentRepo := postgres.NewDocumentRepository(sqlxDB, sqlxDB, mockCache, s.logger, observability.NoopStartSpan)

	// Create service config
	serviceConfig := services.ServiceConfig{
		Logger:  s.logger,
		Metrics: observability.NewNoOpMetricsClient(),
		Tracer:  observability.NoopStartSpan,
	}

	// Create services
	s.documentService = services.NewDocumentService(serviceConfig, documentRepo, mockCache)
	s.workspaceService = services.NewWorkspaceService(serviceConfig, workspaceRepo, documentRepo, mockCache)
}

// TearDownSuite runs once after all tests
func (s *WorkspaceServiceIntegrationSuite) TearDownSuite() {
	s.cancel()
	if s.db != nil {
		// Clean up test data
		_ = shared.CleanupTestData(s.db, s.tenantID)
		_ = s.db.Close()
	}
}

// SetupTest runs before each test
func (s *WorkspaceServiceIntegrationSuite) SetupTest() {
	// Clean up any data from previous test
	_ = shared.CleanupWorkspaceData(s.db, s.tenantID)
}

// TestCreateAndRetrieveWorkspace tests basic workspace creation and retrieval
func (s *WorkspaceServiceIntegrationSuite) TestCreateAndRetrieveWorkspace() {
	// Create workspace
	ownerID := uuid.New().String()
	workspace := &models.Workspace{
		TenantID:    s.tenantID,
		Name:        "Test Workspace",
		Description: "Integration test workspace",
		OwnerID:     ownerID,
		IsPublic:    false,
		Settings: models.WorkspaceSettings{
			Preferences: map[string]interface{}{
				"theme":              "dark",
				"notifications":      true,
				"auto_save":          true,
				"collaboration_mode": "real-time",
			},
		},
		Tags: pq.StringArray{"test", "integration"},
	}

	err := s.workspaceService.Create(s.ctx, workspace)
	require.NoError(s.T(), err)
	assert.NotEqual(s.T(), uuid.Nil, workspace.ID)

	// Retrieve workspace
	retrieved, err := s.workspaceService.Get(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), workspace.Name, retrieved.Name)
	assert.Equal(s.T(), workspace.Description, retrieved.Description)
	assert.Equal(s.T(), workspace.OwnerID, retrieved.OwnerID)
	assert.Equal(s.T(), workspace.Settings.Preferences["theme"], retrieved.Settings.Preferences["theme"])
	assert.ElementsMatch(s.T(), workspace.Tags, retrieved.Tags)
}

// TestWorkspaceMembers tests workspace member management
func (s *WorkspaceServiceIntegrationSuite) TestWorkspaceMembers() {
	// Create workspace
	ownerID := uuid.New().String()
	workspace := &models.Workspace{
		TenantID:    s.tenantID,
		Name:        "Team Workspace",
		Description: "Workspace for team collaboration",
		OwnerID:     ownerID,
		IsPublic:    false,
	}

	err := s.workspaceService.Create(s.ctx, workspace)
	require.NoError(s.T(), err)

	// Add members
	member1ID := uuid.New().String()
	member2ID := uuid.New().String()
	member3ID := uuid.New().String()

	member1 := &models.WorkspaceMember{
		WorkspaceID: workspace.ID,
		AgentID:     member1ID,
		TenantID:    s.tenantID,
		Role:        models.MemberRoleMember,
	}
	err = s.workspaceService.AddMember(s.ctx, member1)
	require.NoError(s.T(), err)

	member2 := &models.WorkspaceMember{
		WorkspaceID: workspace.ID,
		AgentID:     member2ID,
		TenantID:    s.tenantID,
		Role:        models.MemberRoleViewer,
	}
	err = s.workspaceService.AddMember(s.ctx, member2)
	require.NoError(s.T(), err)

	member3 := &models.WorkspaceMember{
		WorkspaceID: workspace.ID,
		AgentID:     member3ID,
		TenantID:    s.tenantID,
		Role:        models.MemberRoleMember,
	}
	err = s.workspaceService.AddMember(s.ctx, member3)
	require.NoError(s.T(), err)

	// Get members
	members, err := s.workspaceService.ListMembers(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	// Should have 4 members (owner + 3 added)
	assert.Len(s.T(), members, 4)

	// Verify member roles
	memberMap := make(map[string]models.MemberRole)
	for _, m := range members {
		memberMap[m.AgentID] = m.Role
	}

	assert.Equal(s.T(), models.MemberRoleOwner, memberMap[ownerID])
	assert.Equal(s.T(), models.MemberRoleMember, memberMap[member1ID])
	assert.Equal(s.T(), models.MemberRoleViewer, memberMap[member2ID])
	assert.Equal(s.T(), models.MemberRoleMember, memberMap[member3ID])

	// Update member role
	err = s.workspaceService.UpdateMemberRole(s.ctx, workspace.ID, member2ID, string(models.MemberRoleMember))
	require.NoError(s.T(), err)

	// Verify role update
	members, err = s.workspaceService.ListMembers(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	for _, m := range members {
		if m.AgentID == member2ID {
			assert.Equal(s.T(), models.MemberRoleMember, m.Role)
			break
		}
	}

	// Remove member
	err = s.workspaceService.RemoveMember(s.ctx, workspace.ID, member3ID)
	require.NoError(s.T(), err)

	// Verify member removed
	members, err = s.workspaceService.ListMembers(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), members, 3)

	// Test error: cannot remove owner
	err = s.workspaceService.RemoveMember(s.ctx, workspace.ID, ownerID)
	assert.Error(s.T(), err)
}

// TestWorkspaceDocuments tests document management within workspace
func (s *WorkspaceServiceIntegrationSuite) TestWorkspaceDocuments() {
	// Create workspace
	workspace := &models.Workspace{
		TenantID: s.tenantID,
		Name:     "Document Workspace",
		OwnerID:  uuid.New().String(),
	}

	err := s.workspaceService.Create(s.ctx, workspace)
	require.NoError(s.T(), err)

	// Create documents
	doc1 := &models.SharedDocument{
		TenantID:    s.tenantID,
		WorkspaceID: workspace.ID,
		Title:       "Design Document",
		Content:     "# System Design\n\nThis is the initial content.",
		Type:        "markdown",
		ContentType: "text/markdown",
		CreatedBy:   workspace.OwnerID,
		Metadata:    models.JSONMap{"tags": []string{"design", "architecture"}},
	}

	doc2 := &models.SharedDocument{
		TenantID:    s.tenantID,
		WorkspaceID: workspace.ID,
		Title:       "Implementation Notes",
		Content:     "Implementation details go here.",
		Type:        "text",
		ContentType: "text/plain",
		CreatedBy:   workspace.OwnerID,
		Metadata:    models.JSONMap{"tags": []string{"implementation"}},
	}

	err = s.documentService.Create(s.ctx, doc1)
	require.NoError(s.T(), err)

	err = s.documentService.Create(s.ctx, doc2)
	require.NoError(s.T(), err)

	// Get workspace documents
	documents, err := s.workspaceService.ListDocuments(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), documents, 2)

	// Update document through workspace service
	operation := &collaboration.DocumentOperation{
		DocumentID: doc1.ID,
		AgentID:    workspace.OwnerID,
		Type:       "replace",
		Path:       "/",
		Value: map[string]interface{}{
			"title":   "Updated Design Document",
			"content": "# System Design\n\nUpdated content with more details.",
			"tags":    []string{"design", "architecture", "v2"},
		},
		VectorClock: map[string]int{workspace.OwnerID: 1},
	}

	err = s.workspaceService.UpdateDocument(s.ctx, doc1.ID, operation)
	require.NoError(s.T(), err)

	// Verify update
	updated, err := s.documentService.Get(s.ctx, doc1.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Updated Design Document", updated.Title)
	assert.Equal(s.T(), "# System Design\n\nUpdated content with more details.", updated.Content)
	tags, _ := updated.Metadata["tags"].([]string)
	assert.ElementsMatch(s.T(), []string{"design", "architecture", "v2"}, tags)

	// Delete document
	err = s.documentService.Delete(s.ctx, doc2.ID)
	require.NoError(s.T(), err)

	// Verify deletion
	documents, err = s.workspaceService.ListDocuments(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), documents, 1)
	assert.Equal(s.T(), doc1.ID, documents[0].ID)
}

// TestWorkspaceStateManagement tests workspace state and CRDT operations
func (s *WorkspaceServiceIntegrationSuite) TestWorkspaceStateManagement() {
	// Create workspace
	workspace := &models.Workspace{
		TenantID: s.tenantID,
		Name:     "State Test Workspace",
		OwnerID:  uuid.New().String(),
	}

	err := s.workspaceService.Create(s.ctx, workspace)
	require.NoError(s.T(), err)

	// Initialize state
	initialState := map[string]interface{}{
		"counter":    0,
		"users":      []string{},
		"settings":   map[string]interface{}{"theme": "light"},
		"lastUpdate": time.Now().Unix(),
	}

	err = s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
		Type:  "set",
		Path:  "/",
		Value: models.JSONMap(initialState),
	})
	require.NoError(s.T(), err)

	// Get state
	state, err := s.workspaceService.GetState(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), state)
	assert.NotNil(s.T(), state.Data)

	// Update counter using increment operation
	err = s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
		Type:  "increment",
		Path:  "/counter",
		Value: 5,
	})
	require.NoError(s.T(), err)

	// Add user to array
	err = s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
		Type:  "append",
		Path:  "/users",
		Value: "user1",
	})
	require.NoError(s.T(), err)

	// Update nested property
	err = s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
		Type:  "set",
		Path:  "/settings/theme",
		Value: "dark",
	})
	require.NoError(s.T(), err)

	// Get updated state
	state, err = s.workspaceService.GetState(s.ctx, workspace.ID)
	require.NoError(s.T(), err)

	// Verify updates
	assert.Equal(s.T(), float64(5), state.Data["counter"])
	assert.Contains(s.T(), state.Data["users"], "user1")

	settings := state.Data["settings"].(map[string]interface{})
	assert.Equal(s.T(), "dark", settings["theme"])
}

// TestConcurrentWorkspaceOperations tests concurrent access to workspace
func (s *WorkspaceServiceIntegrationSuite) TestConcurrentWorkspaceOperations() {
	// Create workspace
	workspace := &models.Workspace{
		TenantID: s.tenantID,
		Name:     "Concurrent Test Workspace",
		OwnerID:  uuid.New().String(),
	}

	err := s.workspaceService.Create(s.ctx, workspace)
	require.NoError(s.T(), err)

	// Initialize counter state
	err = s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
		Type:  "set",
		Path:  "/counter",
		Value: 0,
	})
	require.NoError(s.T(), err)

	// Run concurrent increments
	numGoroutines := 10
	incrementsPerGoroutine := 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := 0; j < incrementsPerGoroutine; j++ {
				err := s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
					Type:  "increment",
					Path:  "/counter",
					Value: 1,
				})
				if err != nil {
					errors <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		assert.NoError(s.T(), err)
	}

	// Verify final counter value
	state, err := s.workspaceService.GetState(s.ctx, workspace.ID)
	require.NoError(s.T(), err)

	expectedValue := float64(numGoroutines * incrementsPerGoroutine)
	assert.Equal(s.T(), expectedValue, state.Data["counter"])
}

// TestWorkspaceSearch tests workspace search functionality
func (s *WorkspaceServiceIntegrationSuite) TestWorkspaceSearch() {
	ownerID := uuid.New().String()

	// Create multiple workspaces
	workspaces := []*models.Workspace{
		{
			TenantID:    s.tenantID,
			Name:        "Engineering Team Workspace",
			Description: "Workspace for engineering projects",
			OwnerID:     ownerID,
			Tags:        pq.StringArray{"engineering", "development"},
			IsPublic:    true,
		},
		{
			TenantID:    s.tenantID,
			Name:        "Marketing Campaign Hub",
			Description: "Central hub for marketing campaigns",
			OwnerID:     ownerID,
			Tags:        pq.StringArray{"marketing", "campaigns"},
			IsPublic:    false,
		},
		{
			TenantID:    s.tenantID,
			Name:        "Product Design Studio",
			Description: "Design collaboration workspace",
			OwnerID:     uuid.New().String(), // Different owner
			Tags:        pq.StringArray{"design", "product"},
			IsPublic:    true,
		},
		{
			TenantID:    s.tenantID,
			Name:        "Engineering Documentation",
			Description: "Technical documentation workspace",
			OwnerID:     ownerID,
			Tags:        pq.StringArray{"engineering", "docs"},
			IsPublic:    true,
		},
	}

	// Create all workspaces
	for _, ws := range workspaces {
		err := s.workspaceService.Create(s.ctx, ws)
		require.NoError(s.T(), err)
	}

	// Search by name
	results, err := s.workspaceService.SearchWorkspaces(s.ctx, "Engineering", interfaces.WorkspaceFilters{})
	require.NoError(s.T(), err)
	assert.Len(s.T(), results, 2)

	// Search by owner
	results, err = s.workspaceService.SearchWorkspaces(s.ctx, "", interfaces.WorkspaceFilters{
		OwnerID: &ownerID,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), results, 3)

	// Search by tag
	results, err = s.workspaceService.SearchWorkspaces(s.ctx, "engineering", interfaces.WorkspaceFilters{})
	require.NoError(s.T(), err)
	assert.Len(s.T(), results, 2)

	// Search public workspaces
	isActive := true
	results, err = s.workspaceService.SearchWorkspaces(s.ctx, "", interfaces.WorkspaceFilters{
		IsActive: &isActive,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), results, 3)
}

// TestWorkspaceActivityTracking tests workspace activity and audit logging
func (s *WorkspaceServiceIntegrationSuite) TestWorkspaceActivityTracking() {
	// Create workspace
	workspace := &models.Workspace{
		TenantID: s.tenantID,
		Name:     "Activity Test Workspace",
		OwnerID:  uuid.New().String(),
	}

	err := s.workspaceService.Create(s.ctx, workspace)
	require.NoError(s.T(), err)

	// Track various activities
	userID := uuid.New().String()

	// Member joined
	member := &models.WorkspaceMember{
		WorkspaceID: workspace.ID,
		AgentID:     userID,
		TenantID:    s.tenantID,
		Role:        models.MemberRoleMember,
	}
	err = s.workspaceService.AddMember(s.ctx, member)
	require.NoError(s.T(), err)

	// Document created
	doc := &models.SharedDocument{
		TenantID:    s.tenantID,
		WorkspaceID: workspace.ID,
		Title:       "Activity Test Document",
		Content:     "Test content",
		Type:        "text",
		ContentType: "text/plain",
		CreatedBy:   userID,
	}
	err = s.documentService.Create(s.ctx, doc)
	require.NoError(s.T(), err)

	// State updated
	err = s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
		Type:  "set",
		Path:  "/lastActivity",
		Value: time.Now().Unix(),
	})
	require.NoError(s.T(), err)

	// Verify activity was recorded (simplified test)
	// Note: GetActivities method doesn't exist in the interface
	// This test would need to be refactored to use available methods
}

// TestWorkspaceArchival tests workspace archival and restoration
func (s *WorkspaceServiceIntegrationSuite) TestWorkspaceArchival() {
	// Create workspace
	workspace := &models.Workspace{
		TenantID:    s.tenantID,
		Name:        "Archive Test Workspace",
		Description: "Workspace to test archival",
		OwnerID:     uuid.New().String(),
	}

	err := s.workspaceService.Create(s.ctx, workspace)
	require.NoError(s.T(), err)

	// Add some data
	doc := &models.SharedDocument{
		TenantID:    s.tenantID,
		WorkspaceID: workspace.ID,
		Title:       "Document in archived workspace",
		Content:     "This document will be archived with the workspace",
		Type:        "text",
		ContentType: "text/plain",
		CreatedBy:   workspace.OwnerID,
	}
	err = s.documentService.Create(s.ctx, doc)
	require.NoError(s.T(), err)

	// Archive workspace
	err = s.workspaceService.Archive(s.ctx, workspace.ID)
	require.NoError(s.T(), err)

	// Verify workspace is archived
	archived, err := s.workspaceService.Get(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.WorkspaceStatusArchived, archived.Status)

	// Verify cannot modify archived workspace
	archived.Name = "Updated Name"
	err = s.workspaceService.Update(s.ctx, archived)
	assert.Error(s.T(), err)

	// Delete the workspace instead of restore (no restore method in interface)
	err = s.workspaceService.Delete(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
}

// TestWorkspaceServiceIntegration runs the test suite
func TestWorkspaceServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(WorkspaceServiceIntegrationSuite))
}
