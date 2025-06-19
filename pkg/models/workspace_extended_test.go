package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceIsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   WorkspaceStatus
		deletedAt *time.Time
		expected bool
	}{
		{"Active workspace", WorkspaceStatusActive, nil, true},
		{"Inactive workspace", WorkspaceStatusInactive, nil, false},
		{"Deleted active workspace", WorkspaceStatusActive, &time.Time{}, false},
		{"Archived workspace", WorkspaceStatusArchived, nil, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Workspace{
				Status:    tt.status,
				DeletedAt: tt.deletedAt,
			}
			assert.Equal(t, tt.expected, w.IsActive())
		})
	}
}

func TestWorkspaceHasFeature(t *testing.T) {
	w := &Workspace{
		Features: pq.StringArray{"collaboration", "analytics", "automation"},
	}
	
	assert.True(t, w.HasFeature("collaboration"))
	assert.True(t, w.HasFeature("analytics"))
	assert.False(t, w.HasFeature("billing"))
	assert.False(t, w.HasFeature(""))
}

func TestWorkspaceHasTag(t *testing.T) {
	w := &Workspace{
		Tags: pq.StringArray{"project", "dev", "priority"},
	}
	
	assert.True(t, w.HasTag("project"))
	assert.True(t, w.HasTag("dev"))
	assert.False(t, w.HasTag("prod"))
	assert.False(t, w.HasTag(""))
}

func TestWorkspaceSetDefaultValues(t *testing.T) {
	w := &Workspace{}
	w.SetDefaultValues()
	
	assert.Equal(t, WorkspaceStatusActive, w.Status)
	assert.Equal(t, WorkspaceVisibilityPrivate, w.Visibility)
	assert.NotNil(t, w.Configuration)
	assert.NotNil(t, w.State)
	assert.NotNil(t, w.Metadata)
	assert.NotNil(t, w.Tags)
	assert.NotNil(t, w.Features)
	
	// Check that settings get defaults
	assert.True(t, w.Settings.Notifications.Enabled)
	assert.Equal(t, 100, w.Limits.MaxMembers)
}

func TestWorkspaceValidate(t *testing.T) {
	tests := []struct {
		name      string
		workspace *Workspace
		wantErr   bool
		errMsg    string
	}{
		{
			name: "Valid workspace",
			workspace: &Workspace{
				Name:     "Test Workspace",
				TenantID: uuid.New(),
				Status:   WorkspaceStatusActive,
			},
			wantErr: false,
		},
		{
			name: "Missing name",
			workspace: &Workspace{
				TenantID: uuid.New(),
				Status:   WorkspaceStatusActive,
			},
			wantErr: true,
			errMsg:  "workspace name is required",
		},
		{
			name: "Missing tenant ID",
			workspace: &Workspace{
				Name:   "Test Workspace",
				Status: WorkspaceStatusActive,
			},
			wantErr: true,
			errMsg:  "tenant ID is required",
		},
		{
			name: "Invalid status",
			workspace: &Workspace{
				Name:     "Test Workspace",
				TenantID: uuid.New(),
				Status:   WorkspaceStatus("invalid"),
			},
			wantErr: true,
			errMsg:  "invalid workspace status",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.workspace.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWorkspaceWithAllFields(t *testing.T) {
	// Test creating a workspace with all new fields
	w := &Workspace{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Name:     "Production Workspace",
		Type:     "development",
		OwnerID:  "user-123",
		Description: "Main development workspace",
		IsPublic: true,
		Owner:    "john.doe@example.com",
		Status:   WorkspaceStatusActive,
		Features: pq.StringArray{"collaboration", "automation", "analytics"},
		Tags:     pq.StringArray{"dev", "team-alpha", "priority"},
		Settings: WorkspaceSettings{
			Notifications: NotificationSettings{
				Enabled:        true,
				EmailEnabled:   true,
				WebhookEnabled: true,
				WebhookURL:    "https://example.com/webhook",
			},
			Collaboration: CollaborationSettings{
				AllowGuestAccess:   false,
				RequireApproval:    true,
				DefaultMemberRole:  "viewer",
				MaxMembers:         50,
				EnablePresence:     true,
				ConflictResolution: "auto_merge",
			},
			Security: SecuritySettings{
				RequireMFA:     true,
				AllowAPIAccess: true,
				SessionTimeout: 30,
				DataEncryption: true,
				AuditLogging:   true,
				ComplianceMode: "SOC2",
				RetentionDays:  365,
			},
		},
		Limits: WorkspaceLimits{
			MaxMembers:       50,
			MaxDocuments:     500,
			MaxStorageBytes:  5 * 1024 * 1024 * 1024, // 5 GB
			MaxOperationsDay: 5000,
			MaxAgents:        25,
			MaxConcurrent:    10,
		},
		Metadata: JSONMap{
			"created_by": "admin",
			"department": "engineering",
			"cost_center": "CC-123",
		},
		Visibility: WorkspaceVisibilityTeam,
		Configuration: JSONMap{
			"theme": "dark",
			"layout": "modern",
		},
		State: JSONMap{
			"active_sessions": 5,
			"last_backup": "2024-01-01T00:00:00Z",
		},
		StateVersion: 42,
		Stats: &WorkspaceStats{
			WorkspaceID:    uuid.New(),
			TotalMembers:   25,
			ActiveMembers:  15,
			TotalDocuments: 150,
			TotalOperations: 1000,
			StorageUsedBytes: 1024 * 1024 * 100, // 100 MB
		},
	}
	
	// Validate the workspace
	err := w.Validate()
	require.NoError(t, err)
	
	// Test the helper methods
	assert.True(t, w.IsActive())
	assert.True(t, w.HasFeature("collaboration"))
	assert.True(t, w.HasTag("dev"))
	assert.False(t, w.IsLocked())
}