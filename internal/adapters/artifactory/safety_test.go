package artifactory

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSafeOperation_Safety(t *testing.T) {
	testCases := []struct {
		name          string
		operation     string
		expectedSafe  bool
		expectedError error
	}{
		{
			name:          "Allowed Get Operation",
			operation:     "get_artifact",
			expectedSafe:  true,
			expectedError: nil,
		},
		{
			name:          "Allowed Search Operation",
			operation:     "search_artifacts",
			expectedSafe:  true,
			expectedError: nil,
		},
		{
			name:          "Restricted Operation",
			operation:     "delete_repository",
			expectedSafe:  false,
			expectedError: ErrRestrictedOperation,
		},
		{
			name:          "Get Operation Not Explicitly Listed",
			operation:     "get_custom_data",
			expectedSafe:  true,
			expectedError: nil,
		},
		{
			name:          "Search Operation Not Explicitly Listed",
			operation:     "search_custom_items",
			expectedSafe:  true,
			expectedError: nil,
		},
		{
			name:          "Unknown Operation",
			operation:     "unknown_operation",
			expectedSafe:  false,
			expectedError: ErrRestrictedOperation,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isSafe, err := IsSafeOperation(tc.operation)
			assert.Equal(t, tc.expectedSafe, isSafe)
			
			if tc.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.True(t, errors.Is(err, tc.expectedError))
			}
		})
	}
}

func TestRestrictedOperations(t *testing.T) {
	// Test that all explicitly restricted operations are considered unsafe
	for _, op := range RestrictedOperations {
		t.Run(op, func(t *testing.T) {
			isSafe, err := IsSafeOperation(op)
			assert.False(t, isSafe)
			assert.Error(t, err)
			assert.Equal(t, ErrRestrictedOperation, err)
		})
	}
}

func TestReadOnlyOperations(t *testing.T) {
	// Test that all explicitly read-only operations are considered safe
	for _, op := range ReadOnlyOperations {
		t.Run(op, func(t *testing.T) {
			isSafe, err := IsSafeOperation(op)
			assert.True(t, isSafe)
			assert.NoError(t, err)
		})
	}
}

func TestAutomaticReadOnlyDetection(t *testing.T) {
	// Test operations with "get_" prefix
	getOperations := []string{
		"get_something_new",
		"get_repository_status",
		"get_artifact_details",
	}

	for _, op := range getOperations {
		t.Run(op, func(t *testing.T) {
			isSafe, err := IsSafeOperation(op)
			assert.True(t, isSafe, "Operations starting with 'get_' should be considered safe")
			assert.NoError(t, err)
		})
	}

	// Test operations with "search_" prefix
	searchOperations := []string{
		"search_something_new",
		"search_repository_artifacts",
		"search_by_property",
	}

	for _, op := range searchOperations {
		t.Run(op, func(t *testing.T) {
			isSafe, err := IsSafeOperation(op)
			assert.True(t, isSafe, "Operations starting with 'search_' should be considered safe")
			assert.NoError(t, err)
		})
	}
}
