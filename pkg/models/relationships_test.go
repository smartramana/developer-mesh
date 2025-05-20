package models

import (
	"testing"
)

func TestNewEntityID(t *testing.T) {
	entityID := NewEntityID(EntityTypeRepository, "S-Corkum", "devops-mcp", "123")
	
	if entityID.Type != EntityTypeRepository {
		t.Errorf("Expected entity type %s, got %s", EntityTypeRepository, entityID.Type)
	}
	
	if entityID.Owner != "S-Corkum" {
		t.Errorf("Expected owner S-Corkum, got %s", entityID.Owner)
	}
	
	if entityID.Repo != "devops-mcp" {
		t.Errorf("Expected repo devops-mcp, got %s", entityID.Repo)
	}
	
	if entityID.ID != "123" {
		t.Errorf("Expected ID 123, got %s", entityID.ID)
	}
}

func TestEntityID_WithQualifiers(t *testing.T) {
	qualifiers := map[string]string{
		"branch": "main",
		"path":   "pkg/models/relationships.go",
	}
	
	entityID := NewEntityID(EntityTypeRepository, "S-Corkum", "devops-mcp", "123").
		WithQualifiers(qualifiers)
	
	if entityID.Qualifiers["branch"] != "main" {
		t.Errorf("Expected qualifier branch=main, got %s", entityID.Qualifiers["branch"])
	}
	
	if entityID.Qualifiers["path"] != "pkg/models/relationships.go" {
		t.Errorf("Expected qualifier path=pkg/models/relationships.go, got %s", entityID.Qualifiers["path"])
	}
}

func TestEntityIDFromContentMetadata(t *testing.T) {
	tests := []struct {
		contentType string
		expectedType EntityType
	}{
		{"issue", EntityTypeIssue},
		{"pull_request", EntityTypePullRequest},
		{"commit", EntityTypeCommit},
		{"file", EntityTypeFile},
		{"release", EntityTypeRelease},
		{"code_chunk", EntityTypeCodeChunk},
		{"comment", EntityTypeComment},
		{"repository", EntityTypeRepository},
		{"user", EntityTypeUser},
		{"organization", EntityTypeOrganization},
		{"discussion", EntityTypeDiscussion},
		{"unknown", EntityType("unknown")},
	}
	
	for _, test := range tests {
		entityID := EntityIDFromContentMetadata(test.contentType, "owner", "repo", "contentID")
		if entityID.Type != test.expectedType {
			t.Errorf("For content type %s, expected entity type %s, got %s", 
				test.contentType, test.expectedType, entityID.Type)
		}
	}
}

func TestNewEntityRelationship(t *testing.T) {
	source := NewEntityID(EntityTypeRepository, "S-Corkum", "devops-mcp", "123")
	target := NewEntityID(EntityTypeIssue, "S-Corkum", "devops-mcp", "456")
	
	relationship := NewEntityRelationship(
		RelationshipTypeContains,
		source,
		target,
		DirectionOutgoing,
		0.8,
	)
	
	if relationship.Type != RelationshipTypeContains {
		t.Errorf("Expected relationship type %s, got %s", RelationshipTypeContains, relationship.Type)
	}
	
	if relationship.Source != source {
		t.Errorf("Expected source %v, got %v", source, relationship.Source)
	}
	
	if relationship.Target != target {
		t.Errorf("Expected target %v, got %v", target, relationship.Target)
	}
	
	if relationship.Direction != DirectionOutgoing {
		t.Errorf("Expected direction %s, got %s", DirectionOutgoing, relationship.Direction)
	}
	
	if relationship.Strength != 0.8 {
		t.Errorf("Expected strength 0.8, got %f", relationship.Strength)
	}
	
	// Test invalid direction default
	relationship = NewEntityRelationship(
		RelationshipTypeContains,
		source,
		target,
		"invalid",
		0.8,
	)
	
	if relationship.Direction != DirectionOutgoing {
		t.Errorf("Expected default direction %s for invalid input, got %s", 
			DirectionOutgoing, relationship.Direction)
	}
}

func TestEntityRelationship_WithContext(t *testing.T) {
	source := NewEntityID(EntityTypeRepository, "S-Corkum", "devops-mcp", "123")
	target := NewEntityID(EntityTypeIssue, "S-Corkum", "devops-mcp", "456")
	
	relationship := NewEntityRelationship(
		RelationshipTypeContains,
		source,
		target,
		DirectionOutgoing,
		0.8,
	).WithContext("Test context")
	
	if relationship.Context != "Test context" {
		t.Errorf("Expected context 'Test context', got '%s'", relationship.Context)
	}
}

func TestEntityRelationship_WithMetadata(t *testing.T) {
	source := NewEntityID(EntityTypeRepository, "S-Corkum", "devops-mcp", "123")
	target := NewEntityID(EntityTypeIssue, "S-Corkum", "devops-mcp", "456")
	
	metadata := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}
	
	relationship := NewEntityRelationship(
		RelationshipTypeContains,
		source,
		target,
		DirectionOutgoing,
		0.8,
	).WithMetadata(metadata)
	
	if relationship.Metadata["key1"] != "value1" {
		t.Errorf("Expected metadata key1=value1, got %v", relationship.Metadata["key1"])
	}
	
	if relationship.Metadata["key2"] != 42 {
		t.Errorf("Expected metadata key2=42, got %v", relationship.Metadata["key2"])
	}
	
	// Test empty metadata initialization
	relationship.Metadata = nil
	relationship = relationship.WithMetadata(metadata)
	
	if relationship.Metadata["key1"] != "value1" {
		t.Errorf("Expected metadata key1=value1 after nil initialization, got %v", 
			relationship.Metadata["key1"])
	}
}

func TestGenerateRelationshipID(t *testing.T) {
	source := NewEntityID(EntityTypeRepository, "S-Corkum", "devops-mcp", "123")
	target := NewEntityID(EntityTypeIssue, "S-Corkum", "devops-mcp", "456")
	
	id := GenerateRelationshipID(
		RelationshipTypeContains,
		source,
		target,
		DirectionOutgoing,
	)
	
	expectedID := "repository:S-Corkum/devops-mcp/123-contains:outgoing-issue:S-Corkum/devops-mcp/456"
	
	if id != expectedID {
		t.Errorf("Expected relationship ID '%s', got '%s'", expectedID, id)
	}
}
