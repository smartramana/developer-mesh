package adapters

import (
	"context"
	"fmt"

	"github.com/S-Corkum/mcp-server/internal/adapters/artifactory"
	"github.com/S-Corkum/mcp-server/internal/adapters/github"
	"github.com/S-Corkum/mcp-server/internal/adapters/harness"
	"github.com/S-Corkum/mcp-server/internal/adapters/sonarqube"
	"github.com/S-Corkum/mcp-server/internal/adapters/xray"
)

// Factory creates and initializes adapters
type Factory struct {
	configs map[string]interface{}
}

// NewFactory creates a new adapter factory
func NewFactory(configs map[string]interface{}) *Factory {
	return &Factory{
		configs: configs,
	}
}

// CreateAdapter creates an adapter by type
func (f *Factory) CreateAdapter(ctx context.Context, adapterType string) (Adapter, error) {
	var adapter Adapter
	var err error

	// Get configuration for the adapter type
	config, ok := f.configs[adapterType]
	if !ok {
		return nil, fmt.Errorf("configuration not found for adapter type: %s", adapterType)
	}

	// Create adapter based on type
	switch adapterType {
	case "github":
		githubConfig, ok := config.(github.Config)
		if !ok {
			return nil, fmt.Errorf("invalid configuration type for github adapter")
		}
		adapter, err = github.NewAdapter(githubConfig)

	case "harness":
		harnessConfig, ok := config.(harness.Config)
		if !ok {
			return nil, fmt.Errorf("invalid configuration type for harness adapter")
		}
		adapter, err = harness.NewAdapter(harnessConfig)

	case "sonarqube":
		sonarqubeConfig, ok := config.(sonarqube.Config)
		if !ok {
			return nil, fmt.Errorf("invalid configuration type for sonarqube adapter")
		}
		adapter, err = sonarqube.NewAdapter(sonarqubeConfig)

	case "artifactory":
		artifactoryConfig, ok := config.(artifactory.Config)
		if !ok {
			return nil, fmt.Errorf("invalid configuration type for artifactory adapter")
		}
		adapter, err = artifactory.NewAdapter(artifactoryConfig)

	case "xray":
		xrayConfig, ok := config.(xray.Config)
		if !ok {
			return nil, fmt.Errorf("invalid configuration type for xray adapter")
		}
		adapter, err = xray.NewAdapter(xrayConfig)

	default:
		return nil, fmt.Errorf("unsupported adapter type: %s", adapterType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create adapter %s: %v", adapterType, err)
	}

	// Initialize the adapter
	if err := adapter.Initialize(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to initialize adapter %s: %v", adapterType, err)
	}

	return adapter, nil
}
