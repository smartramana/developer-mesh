package xray

import (
	"context"
	"testing"

	"github.com/S-Corkum/mcp-server/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestCreateVulnerabilitiesRequest(t *testing.T) {
	// Create an adapter with test configuration
	adapter := &Adapter{
		config: Config{
			BaseURL: "https://xray.example.com",
		},
	}

	ctx := context.Background()

	// Test case 1: Basic query without CVE
	query1 := models.XrayQuery{
		Type: models.XrayQueryTypeVulnerabilities,
	}

	req1, err := adapter.createVulnerabilitiesRequest(ctx, query1)
	assert.NoError(t, err)
	assert.Equal(t, "GET", req1.Method)
	assert.Equal(t, "https://xray.example.com/api/v1/vulnerabilities", req1.URL.String())

	// Test case 2: Query with CVE
	query2 := models.XrayQuery{
		Type: models.XrayQueryTypeVulnerabilities,
		CVE:  "CVE-2021-12345",
	}

	req2, err := adapter.createVulnerabilitiesRequest(ctx, query2)
	assert.NoError(t, err)
	assert.Equal(t, "GET", req2.Method)
	assert.Equal(t, "https://xray.example.com/api/v1/vulnerabilities/CVE-2021-12345", req2.URL.String())
}

func TestCreateLicensesRequest(t *testing.T) {
	// Create an adapter with test configuration
	adapter := &Adapter{
		config: Config{
			BaseURL: "https://xray.example.com",
		},
	}

	ctx := context.Background()

	// Test case 1: Basic query without license ID
	query1 := models.XrayQuery{
		Type: models.XrayQueryTypeLicenses,
	}

	req1, err := adapter.createLicensesRequest(ctx, query1)
	assert.NoError(t, err)
	assert.Equal(t, "GET", req1.Method)
	assert.Equal(t, "https://xray.example.com/api/v1/licenses", req1.URL.String())

	// Test case 2: Query with license ID
	query2 := models.XrayQuery{
		Type:      models.XrayQueryTypeLicenses,
		LicenseID: "MIT",
	}

	req2, err := adapter.createLicensesRequest(ctx, query2)
	assert.NoError(t, err)
	assert.Equal(t, "GET", req2.Method)
	assert.Equal(t, "https://xray.example.com/api/v1/licenses/MIT", req2.URL.String())
}

func TestCreateScansRequest(t *testing.T) {
	// Create an adapter with test configuration
	adapter := &Adapter{
		config: Config{
			BaseURL: "https://xray.example.com",
		},
	}

	ctx := context.Background()

	// Test scan query
	query := models.XrayQuery{
		Type:         models.XrayQueryTypeScans,
		ArtifactPath: "com.example:library:1.0.0",
	}

	req, err := adapter.createScansRequest(ctx, query)
	assert.NoError(t, err)
	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://xray.example.com/api/v1/scans", req.URL.String())
}
