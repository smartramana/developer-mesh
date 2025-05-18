# Custom Tool Integration

This guide demonstrates how to extend the DevOps MCP platform by adding your own tool integrations beyond the built-in GitHub support.

## Overview

DevOps MCP uses an extensible architecture that allows you to integrate additional tools through the adapter pattern. This guide will walk you through creating a custom integration for a hypothetical CI/CD tool called "BuildMaster".

## Integration Architecture

When adding a new tool integration, you'll implement several components:

1. **API Client**: Handles communication with the external API
2. **Repository Interface**: Defines the contract for tool operations
3. **Repository Implementation**: Implements the interface for the specific tool
4. **Adapter**: Bridges between the API layer and repository implementation
5. **API Handlers**: Exposes the tool operations via the REST API

## Step-by-Step Implementation

### 1. Define Repository Interface

First, define the interface for your tool operations in the shared package:

```go
// pkg/repository/ci_repository.go
package repository

import "context"

// BuildStatus represents a CI build status
type BuildStatus string

const (
    BuildStatusPending   BuildStatus = "pending"
    BuildStatusRunning   BuildStatus = "running"
    BuildStatusSucceeded BuildStatus = "succeeded"
    BuildStatusFailed    BuildStatus = "failed"
)

// CIBuild represents a CI build
type CIBuild struct {
    ID        string      `json:"id"`
    ProjectID string      `json:"project_id"`
    Branch    string      `json:"branch"`
    Commit    string      `json:"commit"`
    Status    BuildStatus `json:"status"`
    URL       string      `json:"url"`
    CreatedAt string      `json:"created_at"`
    Metadata  map[string]interface{} `json:"metadata"`
}

// CIRepository defines operations for CI tools
type CIRepository interface {
    // TriggerBuild starts a new build
    TriggerBuild(ctx context.Context, projectID, branch, commit string) (*CIBuild, error)
    
    // GetBuildStatus retrieves the status of a build
    GetBuildStatus(ctx context.Context, buildID string) (*CIBuild, error)
    
    // ListBuilds retrieves a list of builds for a project
    ListBuilds(ctx context.Context, projectID, branch string, limit int) ([]*CIBuild, error)
    
    // CancelBuild cancels an in-progress build
    CancelBuild(ctx context.Context, buildID string) error
}
```

### 2. Create API Client

Create a client for the external tool API:

```go
// pkg/client/buildmaster/client.go
package buildmaster

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// Client provides methods to interact with BuildMaster API
type Client struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

// NewClient creates a new BuildMaster client
func NewClient(baseURL, apiKey string) *Client {
    return &Client{
        baseURL: baseURL,
        apiKey:  apiKey,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

// TriggerBuild starts a new build in BuildMaster
func (c *Client) TriggerBuild(ctx context.Context, projectID, branch, commit string) (map[string]interface{}, error) {
    url := fmt.Sprintf("%s/api/builds", c.baseURL)
    payload := map[string]string{
        "project_id": projectID,
        "branch":     branch,
        "commit":     commit,
    }
    
    // Implement the HTTP request to the BuildMaster API
    // Return the parsed response
    
    return buildResponse, nil
}

// Implement other API methods: GetBuildStatus, ListBuilds, CancelBuild
```

### 3. Create Repository Implementation

Implement the CIRepository interface for BuildMaster:

```go
// apps/mcp-server/internal/repository/buildmaster_repository.go
package repository

import (
    "context"
    "time"
    
    "github.com/S-Corkum/devops-mcp/pkg/client/buildmaster"
    "github.com/S-Corkum/devops-mcp/pkg/repository"
)

// BuildMasterRepository implements the CIRepository interface
type BuildMasterRepository struct {
    client *buildmaster.Client
}

// NewBuildMasterRepository creates a new BuildMaster repository
func NewBuildMasterRepository(baseURL, apiKey string) *BuildMasterRepository {
    return &BuildMasterRepository{
        client: buildmaster.NewClient(baseURL, apiKey),
    }
}

// TriggerBuild implements the CIRepository.TriggerBuild method
func (r *BuildMasterRepository) TriggerBuild(ctx context.Context, projectID, branch, commit string) (*repository.CIBuild, error) {
    // Call the BuildMaster API client
    buildResponse, err := r.client.TriggerBuild(ctx, projectID, branch, commit)
    if err != nil {
        return nil, err
    }
    
    // Convert the API response to a CIBuild struct
    build := &repository.CIBuild{
        ID:        buildResponse["id"].(string),
        ProjectID: projectID,
        Branch:    branch,
        Commit:    commit,
        Status:    repository.BuildStatusPending,
        URL:       buildResponse["url"].(string),
        CreatedAt: time.Now().Format(time.RFC3339),
        Metadata:  buildResponse,
    }
    
    return build, nil
}

// Implement other repository methods: GetBuildStatus, ListBuilds, CancelBuild
```

### 4. Create the Adapter

Following the adapter pattern established in the codebase:

```go
// apps/rest-api/internal/adapters/ci_adapter.go
package adapters

import (
    "context"
    
    "github.com/S-Corkum/devops-mcp/pkg/repository"
    "github.com/S-Corkum/devops-mcp/apps/rest-api/internal/models"
)

// CIAdapter adapts between API models and repository models
type CIAdapter struct {
    repo repository.CIRepository
}

// NewCIAdapter creates a new CIAdapter
func NewCIAdapter(repo repository.CIRepository) *CIAdapter {
    return &CIAdapter{
        repo: repo,
    }
}

// TriggerBuild adapts between API and repository models for triggering builds
func (a *CIAdapter) TriggerBuild(ctx context.Context, request *models.BuildRequest) (*models.BuildResponse, error) {
    // Call the repository
    build, err := a.repo.TriggerBuild(ctx, request.ProjectID, request.Branch, request.Commit)
    if err != nil {
        return nil, err
    }
    
    // Convert repository model to API model
    response := &models.BuildResponse{
        ID:        build.ID,
        ProjectID: build.ProjectID,
        Branch:    build.Branch,
        Commit:    build.Commit,
        Status:    string(build.Status),
        URL:       build.URL,
        CreatedAt: build.CreatedAt,
    }
    
    return response, nil
}

// Implement other adapter methods: GetBuildStatus, ListBuilds, CancelBuild
```

### 5. Create API Handlers

Create REST API handlers for the CI operations:

```go
// apps/rest-api/internal/api/ci_handlers.go
package api

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
    
    "github.com/S-Corkum/devops-mcp/apps/rest-api/internal/adapters"
    "github.com/S-Corkum/devops-mcp/apps/rest-api/internal/models"
)

// CIAPI handles CI-related API endpoints
type CIAPI struct {
    adapter *adapters.CIAdapter
}

// NewCIAPI creates a new CIAPI
func NewCIAPI(adapter *adapters.CIAdapter) *CIAPI {
    return &CIAPI{
        adapter: adapter,
    }
}

// RegisterRoutes registers the CI API routes
func (api *CIAPI) RegisterRoutes(router *gin.Engine) {
    ciGroup := router.Group("/api/v1/ci")
    
    ciGroup.POST("/builds", api.triggerBuild)
    ciGroup.GET("/builds/:id", api.getBuildStatus)
    ciGroup.GET("/projects/:id/builds", api.listBuilds)
    ciGroup.DELETE("/builds/:id", api.cancelBuild)
}

// triggerBuild handles the trigger build API endpoint
func (api *CIAPI) triggerBuild(c *gin.Context) {
    var request models.BuildRequest
    if err := c.ShouldBindJSON(&request); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    response, err := api.adapter.TriggerBuild(c.Request.Context(), &request)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, response)
}

// Implement other API handlers: getBuildStatus, listBuilds, cancelBuild
```

### 6. Register the Integration

Register your CI integration in the server initialization:

```go
// apps/mcp-server/cmd/server/main.go
func main() {
    // Existing initialization code...
    
    // Initialize BuildMaster repository
    buildMasterRepo := repository.NewBuildMasterRepository(
        config.GetString("buildmaster.url"),
        config.GetString("buildmaster.api_key"),
    )
    
    // Register the repository with the server
    server.RegisterCIRepository(buildMasterRepo)
    
    // Continue with server initialization...
}
```

### 7. Register API Routes

Register the new API endpoints:

```go
// apps/rest-api/cmd/server/main.go
func setupRouter(config *config.Config) *gin.Engine {
    router := gin.Default()
    
    // Existing initialization code...
    
    // Initialize BuildMaster adapter
    ciAdapter := adapters.NewCIAdapter(buildMasterRepo)
    ciAPI := api.NewCIAPI(ciAdapter)
    ciAPI.RegisterRoutes(router)
    
    // Continue with router setup...
    return router
}
```

## Example Usage

Here's a Python example of using the custom CI integration:

```python
import requests
import json

API_KEY = "your-api-key"
MCP_BASE_URL = "http://localhost:8080/api/v1"

headers = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}

def trigger_build(project_id, branch, commit):
    """Trigger a build using the BuildMaster integration"""
    data = {
        "project_id": project_id,
        "branch": branch,
        "commit": commit
    }
    
    response = requests.post(
        f"{MCP_BASE_URL}/ci/builds",
        headers=headers,
        data=json.dumps(data)
    )
    
    if response.status_code == 200:
        return response.json()
    else:
        print(f"Error triggering build: {response.text}")
        return None

def get_build_status(build_id):
    """Get the status of a build"""
    response = requests.get(
        f"{MCP_BASE_URL}/ci/builds/{build_id}",
        headers=headers
    )
    
    if response.status_code == 200:
        return response.json()
    else:
        print(f"Error getting build status: {response.text}")
        return None

# Usage example
build = trigger_build("my-project", "main", "abc123")
if build:
    print(f"Build triggered: {build['id']}")
    print(f"Build URL: {build['url']}")
    
    # Check status after some time
    status = get_build_status(build['id'])
    print(f"Build status: {status['status']}")
```

## Integration with the Adapter Pattern

The adapter pattern used throughout DevOps MCP is particularly valuable for tool integrations:

1. **Loose Coupling**: The API layer doesn't depend directly on tool-specific implementations
2. **Consistent Interface**: All tools follow a similar pattern and interface
3. **Easy Extension**: New tools can be added without changing existing code
4. **Type Safety**: Explicit conversions between API and repository types

As shown in the adapter implementation:

```go
// Converting API request to repository model
build, err := a.repo.TriggerBuild(ctx, request.ProjectID, request.Branch, request.Commit)

// Converting repository model to API response
response := &models.BuildResponse{
    ID:        build.ID,
    ProjectID: build.ProjectID,
    // Additional fields...
}
```

## Testing Your Integration

Create comprehensive tests for your implementation:

```go
// apps/mcp-server/internal/repository/buildmaster_repository_test.go
package repository_test

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/assert"
    
    "github.com/S-Corkum/devops-mcp/pkg/repository"
)

// MockBuildMasterClient is a mock of the BuildMaster client
type MockBuildMasterClient struct {
    mock.Mock
}

// Implement the mock methods...

func TestBuildMasterRepository_TriggerBuild(t *testing.T) {
    mockClient := new(MockBuildMasterClient)
    repo := NewBuildMasterRepository("", "")
    repo.client = mockClient
    
    // Set expectations
    mockClient.On("TriggerBuild", mock.Anything, "project-1", "main", "abc123").
        Return(map[string]interface{}{
            "id":  "build-123",
            "url": "https://buildmaster.example.com/builds/123",
        }, nil)
    
    // Call the repository method
    build, err := repo.TriggerBuild(context.Background(), "project-1", "main", "abc123")
    
    // Assertions
    assert.NoError(t, err)
    assert.NotNil(t, build)
    assert.Equal(t, "build-123", build.ID)
    assert.Equal(t, "project-1", build.ProjectID)
    assert.Equal(t, repository.BuildStatusPending, build.Status)
    
    // Verify expectations
    mockClient.AssertExpectations(t)
}
```

## Docker Configuration

When integrating a new tool, you might need to update your Docker configuration to include any new dependencies or settings. Use the `docker-compose.local.yml` file:

```yaml
# Update docker-compose.local.yml to include tool-specific configuration
services:
  mcp-server:
    environment:
      - BUILDMASTER_URL=https://buildmaster.example.com
      - BUILDMASTER_API_KEY=${BUILDMASTER_API_KEY}
```

## Best Practices for Tool Integration

1. **Follow the Adapter Pattern**: Maintain consistency with the existing architecture
2. **Use Standard Error Handling**: Implement proper error types and handling
3. **Implement Comprehensive Tests**: Test both success and failure cases
4. **Document API Endpoints**: Provide clear documentation for users
5. **Add Configuration Options**: Make integration configurable
6. **Handle Authentication Securely**: Follow secure practices for API keys
7. **Implement Rate Limiting**: Add mechanisms to respect API rate limits
8. **Ensure Type Safety**: Use explicit type conversions in adapters
9. **Write Integration Tests**: Test the full integration path
