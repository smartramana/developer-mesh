package integration

import (
	"os"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestCredentialPassthrough(t *testing.T) {
	// Skip if not in integration test mode
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set INTEGRATION_TESTS=true to run")
	}

	// Test with user-provided GitHub PAT
	t.Run("GitHub with user PAT", func(t *testing.T) {
		// This would use a real test client in a full integration test
		// For now, we'll just verify the structures work
		creds := &models.ToolCredentials{
			GitHub: &models.TokenCredential{
				Token: os.Getenv("TEST_GITHUB_PAT"),
				Type:  "pat",
			},
		}

		assert.NotNil(t, creds)
		assert.True(t, creds.HasCredentialFor("github"))
	})

	// Test fallback to service account
	t.Run("Fallback to service account", func(t *testing.T) {
		// Test without credentials - should use service account
		var creds *models.ToolCredentials

		assert.Nil(t, creds)
		assert.False(t, creds.HasCredentialFor("github"))
	})

	// Test invalid credentials
	t.Run("Invalid credentials", func(t *testing.T) {
		creds := &models.ToolCredentials{
			GitHub: &models.TokenCredential{
				Token: "invalid-token",
			},
		}

		assert.NotNil(t, creds)
		assert.True(t, creds.HasCredentialFor("github"))
		// In a real test, this would fail when trying to use the token
	})
}
