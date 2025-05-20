package safety

import (
	"testing"
)

func TestGitHubChecker(t *testing.T) {
	checker := NewGitHubChecker()
	
	tests := []struct {
		operation string
		params    map[string]interface{}
		expected  bool
	}{
		// Safe operations
		{"create_issue", nil, true},
		{"get_repository", nil, true},
		{"archive_repository", nil, true}, // Explicitly allowed "dangerous" operation
		
		// Unsafe operations
		{"delete_repository", nil, false},
		{"delete_organization", nil, false},
	}
	
	for _, test := range tests {
		result, err := checker.IsSafeOperation(test.operation, test.params)
		
		if result != test.expected {
			t.Errorf("IsSafeOperation(%s) = %v, expected %v (error: %v)",
				test.operation, result, test.expected, err)
		}
		
		// If expected unsafe, should return an error
		if !test.expected && err == nil {
			t.Errorf("IsSafeOperation(%s) should return an error for unsafe operations",
				test.operation)
		}
	}
}

func TestArtifactoryChecker(t *testing.T) {
	checker := NewArtifactoryChecker()
	
	tests := []struct {
		operation string
		params    map[string]interface{}
		expected  bool
	}{
		// Safe operations (read-only)
		{"get_artifact", nil, true},
		{"search_artifacts", nil, true},
		{"get_repository_info", nil, true},
		
		// Unsafe operations (write/delete)
		{"upload_artifact", nil, false},
		{"delete_artifact", nil, false},
		{"deploy_artifact", nil, false},
	}
	
	for _, test := range tests {
		result, err := checker.IsSafeOperation(test.operation, test.params)
		
		if result != test.expected {
			t.Errorf("IsSafeOperation(%s) = %v, expected %v (error: %v)",
				test.operation, result, test.expected, err)
		}
		
		// If expected unsafe, should return an error
		if !test.expected && err == nil {
			t.Errorf("IsSafeOperation(%s) should return an error for unsafe operations",
				test.operation)
		}
	}
}

func TestHarnessChecker(t *testing.T) {
	checker := NewHarnessChecker()
	
	tests := []struct {
		operation string
		params    map[string]interface{}
		expected  bool
	}{
		// Safe operations
		{"get_pipeline", nil, true},
		{"trigger_pipeline", nil, true},
		{"toggle_feature_flag", map[string]interface{}{"environment": "dev"}, true},
		
		// Unsafe operations
		{"delete_feature_flag", nil, false},
		{"delete_pipeline", nil, false},
		{"toggle_feature_flag", map[string]interface{}{"environment": "production"}, false},
	}
	
	for _, test := range tests {
		result, err := checker.IsSafeOperation(test.operation, test.params)
		
		if result != test.expected {
			t.Errorf("IsSafeOperation(%s) = %v, expected %v (error: %v)",
				test.operation, result, test.expected, err)
		}
		
		// If expected unsafe, should return an error
		if !test.expected && err == nil {
			t.Errorf("IsSafeOperation(%s) should return an error for unsafe operations",
				test.operation)
		}
	}
}

func TestGetCheckerForAdapter(t *testing.T) {
	// Test known adapters
	adapters := []string{"github", "artifactory", "harness"}
	for _, adapter := range adapters {
		checker := GetCheckerForAdapter(adapter)
		if checker == nil {
			t.Errorf("GetCheckerForAdapter(%s) returned nil", adapter)
		}
	}
	
	// Test unknown adapter - should return default checker
	checker := GetCheckerForAdapter("unknown")
	if checker == nil {
		t.Errorf("GetCheckerForAdapter(unknown) returned nil")
	}
	
	// Test default checker allows all operations
	result, err := checker.IsSafeOperation("any_operation", nil)
	if !result || err != nil {
		t.Errorf("Default checker should allow all operations, got result=%v, err=%v", result, err)
	}
}
