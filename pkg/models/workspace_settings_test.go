package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   WorkspaceStatus
		expected bool
	}{
		{"Active status", WorkspaceStatusActive, true},
		{"Inactive status", WorkspaceStatusInactive, true},
		{"Archived status", WorkspaceStatusArchived, true},
		{"Deleted status", WorkspaceStatusDeleted, true},
		{"Invalid status", WorkspaceStatus("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsValid())
		})
	}
}

func TestWorkspaceSettingsJSON(t *testing.T) {
	settings := GetDefaultWorkspaceSettings()
	
	// Test marshaling
	data, err := json.Marshal(settings)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	
	// Test unmarshaling
	var decoded WorkspaceSettings
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	
	assert.Equal(t, settings.Notifications.Enabled, decoded.Notifications.Enabled)
	assert.Equal(t, settings.Collaboration.ConflictResolution, decoded.Collaboration.ConflictResolution)
	assert.Equal(t, settings.Security.AuditLogging, decoded.Security.AuditLogging)
}

func TestWorkspaceSettingsValueScan(t *testing.T) {
	settings := GetDefaultWorkspaceSettings()
	
	// Test Value method
	value, err := settings.Value()
	require.NoError(t, err)
	assert.NotNil(t, value)
	
	// Test Scan method with byte slice
	var scanned WorkspaceSettings
	err = scanned.Scan(value)
	require.NoError(t, err)
	assert.Equal(t, settings.Notifications.Enabled, scanned.Notifications.Enabled)
	
	// Test Scan with string
	jsonStr := `{"notifications":{"enabled":false},"collaboration":{},"security":{}}`
	var fromString WorkspaceSettings
	err = fromString.Scan(jsonStr)
	require.NoError(t, err)
	assert.False(t, fromString.Notifications.Enabled)
	
	// Test Scan with nil
	var nilSettings WorkspaceSettings
	err = nilSettings.Scan(nil)
	require.NoError(t, err)
	assert.Equal(t, WorkspaceSettings{}, nilSettings)
}

func TestWorkspaceLimitsValueScan(t *testing.T) {
	limits := GetDefaultWorkspaceLimits()
	
	// Test Value method
	value, err := limits.Value()
	require.NoError(t, err)
	assert.NotNil(t, value)
	
	// Test Scan method
	var scanned WorkspaceLimits
	err = scanned.Scan(value)
	require.NoError(t, err)
	assert.Equal(t, limits.MaxMembers, scanned.MaxMembers)
	assert.Equal(t, limits.MaxStorageBytes, scanned.MaxStorageBytes)
}

func TestWorkspaceLimitsIsWithinLimits(t *testing.T) {
	limits := WorkspaceLimits{
		MaxMembers:      10,
		MaxDocuments:    100,
		MaxStorageBytes: 1000,
	}
	
	tests := []struct {
		name         string
		members      int
		documents    int
		storageBytes int64
		expected     bool
	}{
		{"Within all limits", 5, 50, 500, true},
		{"At member limit", 10, 50, 500, true},
		{"Exceeds member limit", 11, 50, 500, false},
		{"Exceeds document limit", 5, 101, 500, false},
		{"Exceeds storage limit", 5, 50, 1001, false},
		{"Zero limit means no limit", 5, 50, 500, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := limits.IsWithinLimits(tt.members, tt.documents, tt.storageBytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDefaultWorkspaceSettings(t *testing.T) {
	settings := GetDefaultWorkspaceSettings()
	
	// Verify notifications defaults
	assert.True(t, settings.Notifications.Enabled)
	assert.False(t, settings.Notifications.EmailEnabled)
	assert.Equal(t, "immediate", settings.Notifications.DigestFrequency)
	assert.Contains(t, settings.Notifications.EventTypes, "member_joined")
	
	// Verify collaboration defaults
	assert.False(t, settings.Collaboration.AllowGuestAccess)
	assert.True(t, settings.Collaboration.RequireApproval)
	assert.Equal(t, "member", settings.Collaboration.DefaultMemberRole)
	assert.Equal(t, "manual", settings.Collaboration.ConflictResolution)
	
	// Verify security defaults
	assert.False(t, settings.Security.RequireMFA)
	assert.True(t, settings.Security.AllowAPIAccess)
	assert.Equal(t, 60, settings.Security.SessionTimeout)
	assert.True(t, settings.Security.DataEncryption)
	assert.True(t, settings.Security.AuditLogging)
	assert.Equal(t, 90, settings.Security.RetentionDays)
}

func TestGetDefaultWorkspaceLimits(t *testing.T) {
	limits := GetDefaultWorkspaceLimits()
	
	assert.Equal(t, 100, limits.MaxMembers)
	assert.Equal(t, 1000, limits.MaxDocuments)
	assert.Equal(t, int64(10*1024*1024*1024), limits.MaxStorageBytes) // 10 GB
	assert.Equal(t, 10000, limits.MaxOperationsDay)
	assert.Equal(t, 50, limits.MaxAgents)
	assert.Equal(t, 20, limits.MaxConcurrent)
}