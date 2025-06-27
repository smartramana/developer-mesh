package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/S-Corkum/devops-mcp/pkg/collaboration"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/interfaces"
)

// WorkspaceService manages shared workspaces with distributed state
type WorkspaceService interface {
	// Workspace lifecycle
	Create(ctx context.Context, workspace *models.Workspace) error
	Get(ctx context.Context, id uuid.UUID) (*models.Workspace, error)
	Update(ctx context.Context, workspace *models.Workspace) error
	Delete(ctx context.Context, id uuid.UUID) error
	Archive(ctx context.Context, id uuid.UUID) error

	// Member management with permissions
	AddMember(ctx context.Context, member *models.WorkspaceMember) error
	RemoveMember(ctx context.Context, workspaceID uuid.UUID, agentID string) error
	UpdateMemberRole(ctx context.Context, workspaceID uuid.UUID, agentID string, role string) error
	UpdateMemberPermissions(ctx context.Context, workspaceID uuid.UUID, agentID string, permissions []string) error
	ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]*models.WorkspaceMember, error)
	GetMemberActivity(ctx context.Context, workspaceID uuid.UUID) ([]*models.MemberActivity, error)

	// State management with CRDT
	GetState(ctx context.Context, workspaceID uuid.UUID) (*models.WorkspaceState, error)
	UpdateState(ctx context.Context, workspaceID uuid.UUID, operation *models.StateOperation) error
	MergeState(ctx context.Context, workspaceID uuid.UUID, remoteState *models.WorkspaceState) error
	GetStateHistory(ctx context.Context, workspaceID uuid.UUID, limit int) ([]*models.StateSnapshot, error)
	RestoreState(ctx context.Context, workspaceID uuid.UUID, snapshotID uuid.UUID) error

	// Document management
	CreateDocument(ctx context.Context, doc *models.SharedDocument) error
	GetDocument(ctx context.Context, docID uuid.UUID) (*models.SharedDocument, error)
	UpdateDocument(ctx context.Context, docID uuid.UUID, operation *collaboration.DocumentOperation) error
	ListDocuments(ctx context.Context, workspaceID uuid.UUID) ([]*models.SharedDocument, error)

	// Real-time collaboration
	BroadcastToMembers(ctx context.Context, workspaceID uuid.UUID, message interface{}) error
	SendToMember(ctx context.Context, workspaceID uuid.UUID, agentID string, message interface{}) error
	GetPresence(ctx context.Context, workspaceID uuid.UUID) ([]*models.MemberPresence, error)
	UpdatePresence(ctx context.Context, workspaceID uuid.UUID, agentID string, status string) error

	// Search and discovery
	ListByAgent(ctx context.Context, agentID string) ([]*models.Workspace, error)
	SearchWorkspaces(ctx context.Context, query string, filters interfaces.WorkspaceFilters) ([]*models.Workspace, error)
	GetRecommendedWorkspaces(ctx context.Context, agentID string) ([]*models.Workspace, error)

	// Analytics
	GetWorkspaceStats(ctx context.Context, workspaceID uuid.UUID) (*models.WorkspaceStats, error)
	GetCollaborationMetrics(ctx context.Context, workspaceID uuid.UUID, period time.Duration) (*models.CollaborationMetrics, error)
}
