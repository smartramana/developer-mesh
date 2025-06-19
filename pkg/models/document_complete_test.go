package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestDocumentType_Validate(t *testing.T) {
	tests := []struct {
		name    string
		docType DocumentType
		wantErr bool
	}{
		{"valid markdown", DocumentTypeMarkdown, false},
		{"valid json", DocumentTypeJSON, false},
		{"valid yaml", DocumentTypeYAML, false},
		{"valid code", DocumentTypeCode, false},
		{"valid diagram", DocumentTypeDiagram, false},
		{"valid runbook", DocumentTypeRunbook, false},
		{"valid playbook", DocumentTypePlaybook, false},
		{"valid template", DocumentTypeTemplate, false},
		{"valid config", DocumentTypeConfig, false},
		{"invalid type", DocumentType("invalid"), true},
		{"empty type", DocumentType(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.docType.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateType_Validate(t *testing.T) {
	tests := []struct {
		name       string
		updateType UpdateType
		wantErr    bool
	}{
		{"valid insert", UpdateTypeInsert, false},
		{"valid delete", UpdateTypeDelete, false},
		{"valid replace", UpdateTypeReplace, false},
		{"valid move", UpdateTypeMove, false},
		{"valid merge", UpdateTypeMerge, false},
		{"invalid type", UpdateType("invalid"), true},
		{"empty type", UpdateType(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.updateType.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConflictStrategy_Validate(t *testing.T) {
	tests := []struct {
		name     string
		strategy ConflictStrategy
		wantErr  bool
	}{
		{"valid latest wins", ConflictStrategyLatestWins, false},
		{"valid merge", ConflictStrategyMerge, false},
		{"valid manual", ConflictStrategyManual, false},
		{"valid custom", ConflictStrategyCustom, false},
		{"invalid strategy", ConflictStrategy("invalid"), true},
		{"empty strategy", ConflictStrategy(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.strategy.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkspaceMemberRole_Validate(t *testing.T) {
	tests := []struct {
		name    string
		role    WorkspaceMemberRole
		wantErr bool
	}{
		{"valid owner", WorkspaceMemberRoleOwner, false},
		{"valid admin", WorkspaceMemberRoleAdmin, false},
		{"valid editor", WorkspaceMemberRoleEditor, false},
		{"valid commenter", WorkspaceMemberRoleCommenter, false},
		{"valid viewer", WorkspaceMemberRoleViewer, false},
		{"valid guest", WorkspaceMemberRoleGuest, false},
		{"invalid role", WorkspaceMemberRole("invalid"), true},
		{"empty role", WorkspaceMemberRole(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.role.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkspaceMemberRole_HasPermission(t *testing.T) {
	tests := []struct {
		name       string
		role       WorkspaceMemberRole
		permission string
		hasPerms   bool
	}{
		// Owner tests
		{"owner has all permissions", WorkspaceMemberRoleOwner, "anything", true},
		{"owner can read", WorkspaceMemberRoleOwner, "read", true},
		{"owner can write", WorkspaceMemberRoleOwner, "write", true},
		{"owner can delete", WorkspaceMemberRoleOwner, "delete", true},

		// Admin tests
		{"admin can read", WorkspaceMemberRoleAdmin, "read", true},
		{"admin can write", WorkspaceMemberRoleAdmin, "write", true},
		{"admin can delete", WorkspaceMemberRoleAdmin, "delete", true},
		{"admin can invite", WorkspaceMemberRoleAdmin, "invite", true},
		{"admin can settings", WorkspaceMemberRoleAdmin, "settings", true},
		{"admin cannot arbitrary", WorkspaceMemberRoleAdmin, "arbitrary", false},

		// Editor tests
		{"editor can read", WorkspaceMemberRoleEditor, "read", true},
		{"editor can write", WorkspaceMemberRoleEditor, "write", true},
		{"editor can comment", WorkspaceMemberRoleEditor, "comment", true},
		{"editor cannot delete", WorkspaceMemberRoleEditor, "delete", false},
		{"editor cannot invite", WorkspaceMemberRoleEditor, "invite", false},

		// Commenter tests
		{"commenter can read", WorkspaceMemberRoleCommenter, "read", true},
		{"commenter can comment", WorkspaceMemberRoleCommenter, "comment", true},
		{"commenter cannot write", WorkspaceMemberRoleCommenter, "write", false},
		{"commenter cannot delete", WorkspaceMemberRoleCommenter, "delete", false},

		// Viewer tests
		{"viewer can read", WorkspaceMemberRoleViewer, "read", true},
		{"viewer cannot write", WorkspaceMemberRoleViewer, "write", false},
		{"viewer cannot comment", WorkspaceMemberRoleViewer, "comment", false},

		// Guest tests
		{"guest can read:public", WorkspaceMemberRoleGuest, "read:public", true},
		{"guest cannot read", WorkspaceMemberRoleGuest, "read", false},
		{"guest cannot write", WorkspaceMemberRoleGuest, "write", false},

		// Invalid role
		{"invalid role has no permissions", WorkspaceMemberRole("invalid"), "read", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.role.HasPermission(tt.permission)
			assert.Equal(t, tt.hasPerms, result)
		})
	}
}

func TestWorkspaceMemberRole_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name         string
		from         WorkspaceMemberRole
		to           WorkspaceMemberRole
		canTransition bool
	}{
		// Owner transitions
		{"owner to admin", WorkspaceMemberRoleOwner, WorkspaceMemberRoleAdmin, true},
		{"owner to editor", WorkspaceMemberRoleOwner, WorkspaceMemberRoleEditor, true},
		{"owner to commenter", WorkspaceMemberRoleOwner, WorkspaceMemberRoleCommenter, true},
		{"owner to viewer", WorkspaceMemberRoleOwner, WorkspaceMemberRoleViewer, true},
		{"owner to guest", WorkspaceMemberRoleOwner, WorkspaceMemberRoleGuest, true},
		{"owner to owner", WorkspaceMemberRoleOwner, WorkspaceMemberRoleOwner, true},

		// Admin transitions
		{"admin to editor", WorkspaceMemberRoleAdmin, WorkspaceMemberRoleEditor, true},
		{"admin to commenter", WorkspaceMemberRoleAdmin, WorkspaceMemberRoleCommenter, true},
		{"admin to viewer", WorkspaceMemberRoleAdmin, WorkspaceMemberRoleViewer, true},
		{"admin to guest", WorkspaceMemberRoleAdmin, WorkspaceMemberRoleGuest, true},
		{"admin to admin", WorkspaceMemberRoleAdmin, WorkspaceMemberRoleAdmin, true},
		{"admin cannot promote to owner", WorkspaceMemberRoleAdmin, WorkspaceMemberRoleOwner, false},

		// Other roles cannot change roles
		{"editor cannot change roles", WorkspaceMemberRoleEditor, WorkspaceMemberRoleAdmin, false},
		{"commenter cannot change roles", WorkspaceMemberRoleCommenter, WorkspaceMemberRoleViewer, false},
		{"viewer cannot change roles", WorkspaceMemberRoleViewer, WorkspaceMemberRoleEditor, false},
		{"guest cannot change roles", WorkspaceMemberRoleGuest, WorkspaceMemberRoleViewer, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.from.CanTransitionTo(tt.to)
			assert.Equal(t, tt.canTransition, result)
		})
	}
}

func TestWorkspaceMemberRole_IsHigherThan(t *testing.T) {
	tests := []struct {
		name     string
		role     WorkspaceMemberRole
		other    WorkspaceMemberRole
		isHigher bool
	}{
		// Owner comparisons
		{"owner higher than admin", WorkspaceMemberRoleOwner, WorkspaceMemberRoleAdmin, true},
		{"owner higher than editor", WorkspaceMemberRoleOwner, WorkspaceMemberRoleEditor, true},
		{"owner higher than commenter", WorkspaceMemberRoleOwner, WorkspaceMemberRoleCommenter, true},
		{"owner higher than viewer", WorkspaceMemberRoleOwner, WorkspaceMemberRoleViewer, true},
		{"owner higher than guest", WorkspaceMemberRoleOwner, WorkspaceMemberRoleGuest, true},
		{"owner not higher than owner", WorkspaceMemberRoleOwner, WorkspaceMemberRoleOwner, false},

		// Admin comparisons
		{"admin not higher than owner", WorkspaceMemberRoleAdmin, WorkspaceMemberRoleOwner, false},
		{"admin higher than editor", WorkspaceMemberRoleAdmin, WorkspaceMemberRoleEditor, true},
		{"admin higher than commenter", WorkspaceMemberRoleAdmin, WorkspaceMemberRoleCommenter, true},
		{"admin higher than viewer", WorkspaceMemberRoleAdmin, WorkspaceMemberRoleViewer, true},
		{"admin higher than guest", WorkspaceMemberRoleAdmin, WorkspaceMemberRoleGuest, true},

		// Editor comparisons
		{"editor not higher than admin", WorkspaceMemberRoleEditor, WorkspaceMemberRoleAdmin, false},
		{"editor higher than commenter", WorkspaceMemberRoleEditor, WorkspaceMemberRoleCommenter, true},
		{"editor higher than viewer", WorkspaceMemberRoleEditor, WorkspaceMemberRoleViewer, true},
		{"editor higher than guest", WorkspaceMemberRoleEditor, WorkspaceMemberRoleGuest, true},

		// Other comparisons
		{"commenter higher than viewer", WorkspaceMemberRoleCommenter, WorkspaceMemberRoleViewer, true},
		{"commenter higher than guest", WorkspaceMemberRoleCommenter, WorkspaceMemberRoleGuest, true},
		{"viewer higher than guest", WorkspaceMemberRoleViewer, WorkspaceMemberRoleGuest, true},
		{"guest not higher than anyone", WorkspaceMemberRoleGuest, WorkspaceMemberRoleViewer, false},

		// Invalid role comparisons
		{"invalid role not higher", WorkspaceMemberRole("invalid"), WorkspaceMemberRoleGuest, false},
		{"valid role not higher than invalid", WorkspaceMemberRoleOwner, WorkspaceMemberRole("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.role.IsHigherThan(tt.other)
			assert.Equal(t, tt.isHigher, result)
		})
	}
}

func TestDocumentUpdate_Structure(t *testing.T) {
	now := time.Now()
	docID := uuid.New()
	updateID := uuid.New()

	update := DocumentUpdate{
		ID:         updateID,
		DocumentID: docID,
		Version:    2,
		UpdateType: UpdateTypeReplace,
		Path:       "/content/paragraph[2]",
		OldValue:   "old text",
		NewValue:   "new text",
		UpdatedBy:  "user123",
		UpdatedAt:  now,
		Metadata: map[string]interface{}{
			"source": "web_editor",
			"ip":     "192.168.1.1",
		},
		Checksum: "abc123def456",
		ConflictResolution: DocumentConflictResolution{
			Strategy:      ConflictStrategyMerge,
			ResolvedBy:    "user456",
			ResolvedAt:    now.Add(time.Minute),
			OriginalValue: "original text",
		},
	}

	assert.Equal(t, updateID, update.ID)
	assert.Equal(t, docID, update.DocumentID)
	assert.Equal(t, 2, update.Version)
	assert.Equal(t, UpdateTypeReplace, update.UpdateType)
	assert.Equal(t, "/content/paragraph[2]", update.Path)
	assert.Equal(t, "old text", update.OldValue)
	assert.Equal(t, "new text", update.NewValue)
	assert.Equal(t, "user123", update.UpdatedBy)
	assert.Equal(t, now, update.UpdatedAt)
	assert.Len(t, update.Metadata, 2)
	assert.Equal(t, "abc123def456", update.Checksum)
	assert.Equal(t, ConflictStrategyMerge, update.ConflictResolution.Strategy)
}

func TestConflictResolution_Structure(t *testing.T) {
	now := time.Now()
	resolution := DocumentConflictResolution{
		Strategy:      ConflictStrategyManual,
		ResolvedBy:    "admin@example.com",
		ResolvedAt:    now,
		OriginalValue: map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, ConflictStrategyManual, resolution.Strategy)
	assert.Equal(t, "admin@example.com", resolution.ResolvedBy)
	assert.Equal(t, now, resolution.ResolvedAt)
	assert.NotNil(t, resolution.OriginalValue)
}

func TestDocumentTypeConstants(t *testing.T) {
	// Ensure all document type constants are defined
	types := []DocumentType{
		DocumentTypeMarkdown,
		DocumentTypeJSON,
		DocumentTypeYAML,
		DocumentTypeCode,
		DocumentTypeDiagram,
		DocumentTypeRunbook,
		DocumentTypePlaybook,
		DocumentTypeTemplate,
		DocumentTypeConfig,
	}

	for _, docType := range types {
		t.Run(string(docType), func(t *testing.T) {
			assert.NotEmpty(t, docType)
		})
	}
}

func TestUpdateTypeConstants(t *testing.T) {
	// Ensure all update type constants are defined
	types := []UpdateType{
		UpdateTypeInsert,
		UpdateTypeDelete,
		UpdateTypeReplace,
		UpdateTypeMove,
		UpdateTypeMerge,
	}

	for _, updateType := range types {
		t.Run(string(updateType), func(t *testing.T) {
			assert.NotEmpty(t, updateType)
		})
	}
}

func TestConflictStrategyConstants(t *testing.T) {
	// Ensure all conflict strategy constants are defined
	strategies := []ConflictStrategy{
		ConflictStrategyLatestWins,
		ConflictStrategyMerge,
		ConflictStrategyManual,
		ConflictStrategyCustom,
	}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			assert.NotEmpty(t, strategy)
		})
	}
}

func TestRolePermissions_Completeness(t *testing.T) {
	// Ensure all roles have permissions defined
	roles := []WorkspaceMemberRole{
		WorkspaceMemberRoleOwner,
		WorkspaceMemberRoleAdmin,
		WorkspaceMemberRoleEditor,
		WorkspaceMemberRoleCommenter,
		WorkspaceMemberRoleViewer,
		WorkspaceMemberRoleGuest,
	}

	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			perms, exists := RolePermissions[role]
			assert.True(t, exists, "Role %s should have permissions defined", role)
			assert.NotEmpty(t, perms, "Role %s should have at least one permission", role)
		})
	}
}