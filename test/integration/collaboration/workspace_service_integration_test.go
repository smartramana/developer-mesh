package collaboration

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
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

	// Create repositories
	workspaceRepo := postgres.NewWorkspaceRepository(db, s.logger)
	documentRepo := postgres.NewDocumentRepository(db, s.logger)

	// Create services
	s.documentService = services.NewDocumentServiceImpl(documentRepo, nil, s.logger)
	s.workspaceService = services.NewWorkspaceServiceImpl(
		workspaceRepo,
		s.documentService,
		nil, // conflict service
		s.logger,
	)
}

// TearDownSuite runs once after all tests
func (s *WorkspaceServiceIntegrationSuite) TearDownSuite() {
	s.cancel()
	if s.db != nil {
		// Clean up test data
		shared.CleanupTestData(s.db, s.tenantID)
		s.db.Close()
	}
}

// SetupTest runs before each test
func (s *WorkspaceServiceIntegrationSuite) SetupTest() {
	// Clean up any data from previous test
	shared.CleanupWorkspaceData(s.db, s.tenantID)
}

// TestCreateAndRetrieveWorkspace tests basic workspace creation and retrieval
func (s *WorkspaceServiceIntegrationSuite) TestCreateAndRetrieveWorkspace() {
	// Create workspace
	ownerID := uuid.New()
	workspace := &models.Workspace{
		TenantID:    s.tenantID,
		Name:        "Test Workspace",
		Description: "Integration test workspace",
		OwnerID:     ownerID,
		IsPublic:    false,
		Settings: map[string]interface{}{
			"theme":             "dark",
			"notifications":     true,
			"auto_save":         true,
			"collaboration_mode": "real-time",
		},
		Tags: []string{"test", "integration"},
	}

	err := s.workspaceService.CreateWorkspace(s.ctx, workspace)
	require.NoError(s.T(), err)
	assert.NotEqual(s.T(), uuid.Nil, workspace.ID)

	// Retrieve workspace
	retrieved, err := s.workspaceService.GetWorkspace(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), workspace.Name, retrieved.Name)
	assert.Equal(s.T(), workspace.Description, retrieved.Description)
	assert.Equal(s.T(), workspace.OwnerID, retrieved.OwnerID)
	assert.Equal(s.T(), workspace.Settings, retrieved.Settings)
	assert.ElementsMatch(s.T(), workspace.Tags, retrieved.Tags)
}

// TestWorkspaceMembers tests workspace member management
func (s *WorkspaceServiceIntegrationSuite) TestWorkspaceMembers() {
	// Create workspace
	ownerID := uuid.New()
	workspace := &models.Workspace{
		TenantID:    s.tenantID,
		Name:        "Team Workspace",
		Description: "Workspace for team collaboration",
		OwnerID:     ownerID,
		IsPublic:    false,
	}

	err := s.workspaceService.CreateWorkspace(s.ctx, workspace)
	require.NoError(s.T(), err)

	// Add members
	member1ID := uuid.New()
	member2ID := uuid.New()
	member3ID := uuid.New()

	err = s.workspaceService.AddMember(s.ctx, workspace.ID, member1ID, models.WorkspaceMemberRoleEditor)
	require.NoError(s.T(), err)

	err = s.workspaceService.AddMember(s.ctx, workspace.ID, member2ID, models.WorkspaceMemberRoleViewer)
	require.NoError(s.T(), err)

	err = s.workspaceService.AddMember(s.ctx, workspace.ID, member3ID, models.WorkspaceMemberRoleEditor)
	require.NoError(s.T(), err)

	// Get members
	members, err := s.workspaceService.GetMembers(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	// Should have 4 members (owner + 3 added)
	assert.Len(s.T(), members, 4)

	// Verify member roles
	memberMap := make(map[uuid.UUID]models.WorkspaceMemberRole)
	for _, m := range members {
		memberMap[m.UserID] = m.Role
	}

	assert.Equal(s.T(), models.WorkspaceMemberRoleOwner, memberMap[ownerID])
	assert.Equal(s.T(), models.WorkspaceMemberRoleEditor, memberMap[member1ID])
	assert.Equal(s.T(), models.WorkspaceMemberRoleViewer, memberMap[member2ID])
	assert.Equal(s.T(), models.WorkspaceMemberRoleEditor, memberMap[member3ID])

	// Update member role
	err = s.workspaceService.UpdateMemberRole(s.ctx, workspace.ID, member2ID, models.WorkspaceMemberRoleEditor)
	require.NoError(s.T(), err)

	// Verify role update
	member, err := s.workspaceService.GetMember(s.ctx, workspace.ID, member2ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.WorkspaceMemberRoleEditor, member.Role)

	// Remove member
	err = s.workspaceService.RemoveMember(s.ctx, workspace.ID, member3ID)
	require.NoError(s.T(), err)

	// Verify member removed
	members, err = s.workspaceService.GetMembers(s.ctx, workspace.ID)
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
		OwnerID:  uuid.New(),
	}

	err := s.workspaceService.CreateWorkspace(s.ctx, workspace)
	require.NoError(s.T(), err)

	// Create documents
	doc1 := &models.Document{
		TenantID:    s.tenantID,
		WorkspaceID: workspace.ID,
		Title:       "Design Document",
		Content:     "# System Design\n\nThis is the initial content.",
		Type:        models.DocumentTypeMarkdown,
		Tags:        []string{"design", "architecture"},
		CreatedBy:   workspace.OwnerID,
	}

	doc2 := &models.Document{
		TenantID:    s.tenantID,
		WorkspaceID: workspace.ID,
		Title:       "Implementation Notes",
		Content:     "Implementation details go here.",
		Type:        models.DocumentTypeText,
		Tags:        []string{"implementation"},
		CreatedBy:   workspace.OwnerID,
	}

	err = s.documentService.CreateDocument(s.ctx, doc1)
	require.NoError(s.T(), err)

	err = s.documentService.CreateDocument(s.ctx, doc2)
	require.NoError(s.T(), err)

	// Get workspace documents
	documents, err := s.workspaceService.GetDocuments(s.ctx, workspace.ID, services.DocumentFilters{
		WorkspaceID: workspace.ID,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), documents, 2)

	// Update document through workspace service
	update := &models.DocumentUpdate{
		Title:   "Updated Design Document",
		Content: "# System Design\n\nUpdated content with more details.",
		Tags:    []string{"design", "architecture", "v2"},
	}

	err = s.workspaceService.UpdateDocument(s.ctx, workspace.ID, doc1.ID, update)
	require.NoError(s.T(), err)

	// Verify update
	updated, err := s.documentService.GetDocument(s.ctx, doc1.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), update.Title, updated.Title)
	assert.Equal(s.T(), update.Content, updated.Content)
	assert.ElementsMatch(s.T(), update.Tags, updated.Tags)

	// Delete document
	err = s.workspaceService.DeleteDocument(s.ctx, workspace.ID, doc2.ID)
	require.NoError(s.T(), err)

	// Verify deletion
	documents, err = s.workspaceService.GetDocuments(s.ctx, workspace.ID, services.DocumentFilters{
		WorkspaceID: workspace.ID,
	})
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
		OwnerID:  uuid.New(),
	}

	err := s.workspaceService.CreateWorkspace(s.ctx, workspace)
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
	stateMap := state.(map[string]interface{})
	assert.Equal(s.T(), float64(5), stateMap["counter"])
	assert.Contains(s.T(), stateMap["users"], "user1")
	
	settings := stateMap["settings"].(map[string]interface{})
	assert.Equal(s.T(), "dark", settings["theme"])
}

// TestConcurrentWorkspaceOperations tests concurrent access to workspace
func (s *WorkspaceServiceIntegrationSuite) TestConcurrentWorkspaceOperations() {
	// Create workspace
	workspace := &models.Workspace{
		TenantID: s.tenantID,
		Name:     "Concurrent Test Workspace",
		OwnerID:  uuid.New(),
	}

	err := s.workspaceService.CreateWorkspace(s.ctx, workspace)
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
	
	stateMap := state.(map[string]interface{})
	expectedValue := float64(numGoroutines * incrementsPerGoroutine)
	assert.Equal(s.T(), expectedValue, stateMap["counter"])
}

// TestWorkspaceSearch tests workspace search functionality
func (s *WorkspaceServiceIntegrationSuite) TestWorkspaceSearch() {
	ownerID := uuid.New()

	// Create multiple workspaces
	workspaces := []*models.Workspace{
		{
			TenantID:    s.tenantID,
			Name:        "Engineering Team Workspace",
			Description: "Workspace for engineering projects",
			OwnerID:     ownerID,
			Tags:        []string{"engineering", "development"},
			IsPublic:    true,
		},
		{
			TenantID:    s.tenantID,
			Name:        "Marketing Campaign Hub",
			Description: "Central hub for marketing campaigns",
			OwnerID:     ownerID,
			Tags:        []string{"marketing", "campaigns"},
			IsPublic:    false,
		},
		{
			TenantID:    s.tenantID,
			Name:        "Product Design Studio",
			Description: "Design collaboration workspace",
			OwnerID:     uuid.New(), // Different owner
			Tags:        []string{"design", "product"},
			IsPublic:    true,
		},
		{
			TenantID:    s.tenantID,
			Name:        "Engineering Documentation",
			Description: "Technical documentation workspace",
			OwnerID:     ownerID,
			Tags:        []string{"engineering", "docs"},
			IsPublic:    true,
		},
	}

	// Create all workspaces
	for _, ws := range workspaces {
		err := s.workspaceService.CreateWorkspace(s.ctx, ws)
		require.NoError(s.T(), err)
	}

	// Search by name
	results, err := s.workspaceService.SearchWorkspaces(s.ctx, "Engineering", services.WorkspaceFilters{
		TenantID: s.tenantID,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), results, 2)

	// Search by owner
	results, err = s.workspaceService.SearchWorkspaces(s.ctx, "", services.WorkspaceFilters{
		TenantID: s.tenantID,
		OwnerID:  ownerID,
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), results, 3)

	// Search by tag
	results, err = s.workspaceService.SearchWorkspaces(s.ctx, "", services.WorkspaceFilters{
		TenantID: s.tenantID,
		Tags:     []string{"engineering"},
	})
	require.NoError(s.T(), err)
	assert.Len(s.T(), results, 2)

	// Search public workspaces
	results, err = s.workspaceService.SearchWorkspaces(s.ctx, "", services.WorkspaceFilters{
		TenantID: s.tenantID,
		IsPublic: true,
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
		OwnerID:  uuid.New(),
	}

	err := s.workspaceService.CreateWorkspace(s.ctx, workspace)
	require.NoError(s.T(), err)

	// Track various activities
	userID := uuid.New()

	// Member joined
	err = s.workspaceService.AddMember(s.ctx, workspace.ID, userID, models.WorkspaceMemberRoleEditor)
	require.NoError(s.T(), err)

	// Document created
	doc := &models.Document{
		TenantID:    s.tenantID,
		WorkspaceID: workspace.ID,
		Title:       "Activity Test Document",
		Content:     "Test content",
		CreatedBy:   userID,
	}
	err = s.documentService.CreateDocument(s.ctx, doc)
	require.NoError(s.T(), err)

	// State updated
	err = s.workspaceService.UpdateState(s.ctx, workspace.ID, &models.StateOperation{
		Type:  "set",
		Path:  "/lastActivity",
		Value: time.Now().Unix(),
	})
	require.NoError(s.T(), err)

	// Get workspace activity
	activities, err := s.workspaceService.GetActivities(s.ctx, workspace.ID, 10, 0)
	require.NoError(s.T(), err)
	assert.GreaterOrEqual(s.T(), len(activities), 3)

	// Verify activity types
	activityTypes := make(map[string]bool)
	for _, activity := range activities {
		activityTypes[activity.Type] = true
	}

	assert.True(s.T(), activityTypes["member_added"])
	assert.True(s.T(), activityTypes["document_created"])
	assert.True(s.T(), activityTypes["state_updated"])
}

// TestWorkspaceArchival tests workspace archival and restoration
func (s *WorkspaceServiceIntegrationSuite) TestWorkspaceArchival() {
	// Create workspace
	workspace := &models.Workspace{
		TenantID:    s.tenantID,
		Name:        "Archive Test Workspace",
		Description: "Workspace to test archival",
		OwnerID:     uuid.New(),
	}

	err := s.workspaceService.CreateWorkspace(s.ctx, workspace)
	require.NoError(s.T(), err)

	// Add some data
	doc := &models.Document{
		TenantID:    s.tenantID,
		WorkspaceID: workspace.ID,
		Title:       "Document in archived workspace",
		Content:     "This document will be archived with the workspace",
		CreatedBy:   workspace.OwnerID,
	}
	err = s.documentService.CreateDocument(s.ctx, doc)
	require.NoError(s.T(), err)

	// Archive workspace
	err = s.workspaceService.ArchiveWorkspace(s.ctx, workspace.ID)
	require.NoError(s.T(), err)

	// Verify workspace is archived
	archived, err := s.workspaceService.GetWorkspace(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	assert.True(s.T(), archived.IsArchived)
	assert.NotNil(s.T(), archived.ArchivedAt)

	// Verify cannot modify archived workspace
	err = s.workspaceService.UpdateWorkspace(s.ctx, workspace.ID, &models.WorkspaceUpdate{
		Name: "Updated Name",
	})
	assert.Error(s.T(), err)

	// Restore workspace
	err = s.workspaceService.RestoreWorkspace(s.ctx, workspace.ID)
	require.NoError(s.T(), err)

	// Verify workspace is restored
	restored, err := s.workspaceService.GetWorkspace(s.ctx, workspace.ID)
	require.NoError(s.T(), err)
	assert.False(s.T(), restored.IsArchived)
	assert.Nil(s.T(), restored.ArchivedAt)

	// Verify can modify restored workspace
	err = s.workspaceService.UpdateWorkspace(s.ctx, workspace.ID, &models.WorkspaceUpdate{
		Name: "Restored Workspace Name",
	})
	require.NoError(s.T(), err)
}

// TestWorkspaceServiceIntegration runs the test suite
func TestWorkspaceServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(WorkspaceServiceIntegrationSuite))
}