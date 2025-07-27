package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/storage"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/developer-mesh/developer-mesh/pkg/tools/adapters"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// DynamicToolsServiceInterface defines the interface for dynamic tools operations
type DynamicToolsServiceInterface interface {
	ListTools(ctx context.Context, tenantID string, status string) ([]*models.DynamicTool, error)
	GetTool(ctx context.Context, tenantID, toolID string) (*models.DynamicTool, error)
	CreateTool(ctx context.Context, tenantID string, config tools.ToolConfig) (*models.DynamicTool, error)
	UpdateTool(ctx context.Context, tenantID, toolID string, config tools.ToolConfig) (*models.DynamicTool, error)
	DeleteTool(ctx context.Context, tenantID, toolID string) error

	// Discovery operations
	StartDiscovery(ctx context.Context, config tools.ToolConfig) (*models.DiscoverySession, error)
	GetDiscoverySession(ctx context.Context, sessionID string) (*models.DiscoverySession, error)
	ConfirmDiscovery(ctx context.Context, sessionID string, toolConfig tools.ToolConfig) (*models.DynamicTool, error)

	// Health check operations
	CheckToolHealth(ctx context.Context, tenantID, toolID string) (*models.ToolHealthStatus, error)
	RefreshToolHealth(ctx context.Context, tenantID, toolID string) (*models.ToolHealthStatus, error)

	// Action operations
	ListToolActions(ctx context.Context, tenantID, toolID string) ([]models.ToolAction, error)
	ExecuteToolAction(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}) (interface{}, error)

	// Credential operations
	UpdateToolCredentials(ctx context.Context, tenantID, toolID string, creds *models.TokenCredential) error
}

// DynamicToolsService implements the dynamic tools business logic
type DynamicToolsService struct {
	db               *sqlx.DB
	logger           observability.Logger
	metricsClient    observability.MetricsClient
	encryptionSvc    *security.EncryptionService
	discoveryService DiscoveryServiceInterface
	healthCheckMgr   *tools.HealthCheckManager
	toolCache        map[string]*models.DynamicTool // Simple in-memory cache
	patternRepo      *storage.DiscoveryPatternRepository
}

// NewDynamicToolsService creates a new dynamic tools service
func NewDynamicToolsService(
	db *sqlx.DB,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	encryptionSvc *security.EncryptionService,
	patternRepo *storage.DiscoveryPatternRepository,
) DynamicToolsServiceInterface {
	// Create discovery service with pattern repository
	discoveryService := NewEnhancedDiscoveryService(
		logger,
		metricsClient,
		patternRepo,
		storage.NewDiscoveryHintRepository(db.DB),
	)

	// Create health check manager with required dependencies
	cacheClient := cache.NewMemoryCache(1000, 24*time.Hour)
	openAPIHandler := adapters.NewOpenAPIAdapter(logger)
	healthCheckMgr := tools.NewHealthCheckManager(cacheClient, openAPIHandler, logger, metricsClient)

	return &DynamicToolsService{
		db:               db,
		logger:           logger,
		metricsClient:    metricsClient,
		encryptionSvc:    encryptionSvc,
		discoveryService: discoveryService,
		healthCheckMgr:   healthCheckMgr,
		toolCache:        make(map[string]*models.DynamicTool),
		patternRepo:      patternRepo,
	}
}

// ListTools lists all tools for a tenant
func (s *DynamicToolsService) ListTools(ctx context.Context, tenantID string, status string) ([]*models.DynamicTool, error) {
	query := `
		SELECT 
			id, tenant_id, tool_name, display_name, 
			config, auth_type, retry_policy, status, 
			health_status, last_health_check,
			created_at, updated_at, provider, passthrough_config
		FROM tool_configurations
		WHERE tenant_id = $1
	`
	args := []interface{}{tenantID}

	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tools: %w", err)
	}
	defer rows.Close()

	var tools []*models.DynamicTool
	for rows.Next() {
		tool, err := s.scanTool(rows)
		if err != nil {
			s.logger.Error("Failed to scan tool", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}
		tools = append(tools, tool)
	}

	return tools, nil
}

// GetTool gets a specific tool
func (s *DynamicToolsService) GetTool(ctx context.Context, tenantID, toolID string) (*models.DynamicTool, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", tenantID, toolID)
	if cached, ok := s.toolCache[cacheKey]; ok {
		return cached, nil
	}

	query := `
		SELECT 
			id, tenant_id, tool_name, display_name, 
			config, auth_type, retry_policy, status, 
			health_status, last_health_check,
			created_at, updated_at, provider, passthrough_config
		FROM tool_configurations
		WHERE tenant_id = $1 AND id = $2
	`

	var tool models.DynamicTool
	err := s.db.GetContext(ctx, &tool, query, tenantID, toolID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tool not found")
		}
		return nil, fmt.Errorf("failed to get tool: %w", err)
	}

	// Cache the result
	s.toolCache[cacheKey] = &tool

	return &tool, nil
}

// CreateTool creates a new tool with discovery
func (s *DynamicToolsService) CreateTool(ctx context.Context, tenantID string, config tools.ToolConfig) (*models.DynamicTool, error) {
	// Perform discovery first
	result, err := s.discoveryService.DiscoverTool(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to discover tool: %w", err)
	}

	if result.Status != tools.DiscoveryStatusSuccess {
		return nil, fmt.Errorf("discovery failed with status: %s", result.Status)
	}

	// Extract tool information from discovery result
	toolID := uuid.New().String()
	now := time.Now()

	// Prepare config with discovered information
	if config.Config == nil {
		config.Config = make(map[string]interface{})
	}
	config.Config["discovery_result"] = result
	config.Config["spec_url"] = result.SpecURL
	config.Config["discovered_urls"] = result.DiscoveredURLs

	configJSON, err := json.Marshal(config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Encrypt credentials if provided
	var encryptedCreds []byte
	if config.Credential != nil {
		encryptedJSON, err := s.encryptionSvc.EncryptJSON(config.Credential, config.TenantID)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt credentials: %w", err)
		}
		encryptedCreds = []byte(encryptedJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt credentials: %w", err)
		}
	}

	// Insert tool into database
	query := `
		INSERT INTO tool_configurations (
			id, tenant_id, tool_name, display_name, config,
			auth_type, credentials_encrypted, retry_policy, status,
			health_status, last_health_check, created_at, updated_at,
			provider, passthrough_config
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		) RETURNING *
	`

	tool := &models.DynamicTool{
		ID:                toolID,
		TenantID:          tenantID,
		ToolName:          config.Name,
		DisplayName:       config.Name,
		Config:            config.Config,
		AuthType:          "bearer", // Default, should be extracted from discovery
		Status:            "active",
		HealthStatus:      json.RawMessage(`{"status": "unknown"}`),
		LastHealthCheck:   &now,
		CreatedAt:         now,
		UpdatedAt:         now,
		Provider:          config.Provider,
		PassthroughConfig: (*models.PassthroughConfig)(config.PassthroughConfig),
	}

	err = s.db.GetContext(ctx, tool, query,
		toolID, tenantID, config.Name, config.Name, configJSON,
		tool.AuthType, encryptedCreds, nil, tool.Status,
		tool.HealthStatus, tool.LastHealthCheck, tool.CreatedAt, tool.UpdatedAt,
		tool.Provider, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool: %w", err)
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("%s:%s", tenantID, toolID)
	s.toolCache[cacheKey] = tool

	return tool, nil
}

// UpdateTool updates an existing tool
func (s *DynamicToolsService) UpdateTool(ctx context.Context, tenantID, toolID string, config tools.ToolConfig) (*models.DynamicTool, error) {
	// Similar to CreateTool but with UPDATE query
	// Implementation would follow the same pattern
	return nil, fmt.Errorf("not implemented")
}

// DeleteTool deletes a tool
func (s *DynamicToolsService) DeleteTool(ctx context.Context, tenantID, toolID string) error {
	query := `
		DELETE FROM tool_configurations
		WHERE tenant_id = $1 AND id = $2
	`

	result, err := s.db.ExecContext(ctx, query, tenantID, toolID)
	if err != nil {
		return fmt.Errorf("failed to delete tool: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("tool not found")
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("%s:%s", tenantID, toolID)
	delete(s.toolCache, cacheKey)

	return nil
}

// StartDiscovery starts a discovery session
func (s *DynamicToolsService) StartDiscovery(ctx context.Context, config tools.ToolConfig) (*models.DiscoverySession, error) {
	// Implementation similar to MCP server version
	return nil, fmt.Errorf("not implemented")
}

// GetDiscoverySession gets a discovery session
func (s *DynamicToolsService) GetDiscoverySession(ctx context.Context, sessionID string) (*models.DiscoverySession, error) {
	return nil, fmt.Errorf("not implemented")
}

// ConfirmDiscovery confirms a discovery session
func (s *DynamicToolsService) ConfirmDiscovery(ctx context.Context, sessionID string, toolConfig tools.ToolConfig) (*models.DynamicTool, error) {
	return nil, fmt.Errorf("not implemented")
}

// CheckToolHealth checks tool health
func (s *DynamicToolsService) CheckToolHealth(ctx context.Context, tenantID, toolID string) (*models.ToolHealthStatus, error) {
	tool, err := s.GetTool(ctx, tenantID, toolID)
	if err != nil {
		return nil, err
	}

	// Convert to tools.ToolConfig
	config := tools.ToolConfig{
		ID:       tool.ID,
		TenantID: tool.TenantID,
		Name:     tool.ToolName,
		BaseURL:  "",
		Config:   tool.Config,
	}

	// Extract base URL from config
	if baseURL, ok := tool.Config["base_url"].(string); ok {
		config.BaseURL = baseURL
	}

	// Use health check manager
	status, err := s.healthCheckMgr.CheckHealth(ctx, config, false)
	if err != nil {
		return nil, err
	}

	// Convert to models.ToolHealthStatus
	return &models.ToolHealthStatus{
		IsHealthy:    status.IsHealthy,
		LastChecked:  status.LastChecked,
		ResponseTime: status.ResponseTime,
		Error:        status.Error,
		Details:      status.Details,
	}, nil
}

// RefreshToolHealth refreshes tool health
func (s *DynamicToolsService) RefreshToolHealth(ctx context.Context, tenantID, toolID string) (*models.ToolHealthStatus, error) {
	// Force refresh by clearing cache
	return s.CheckToolHealth(ctx, tenantID, toolID)
}

// ListToolActions lists available actions for a tool
func (s *DynamicToolsService) ListToolActions(ctx context.Context, tenantID, toolID string) ([]models.ToolAction, error) {
	return nil, fmt.Errorf("not implemented")
}

// ExecuteToolAction executes a tool action
func (s *DynamicToolsService) ExecuteToolAction(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// UpdateToolCredentials updates tool credentials
func (s *DynamicToolsService) UpdateToolCredentials(ctx context.Context, tenantID, toolID string, creds *models.TokenCredential) error {
	return fmt.Errorf("not implemented")
}

// scanTool scans a tool from database row
func (s *DynamicToolsService) scanTool(rows *sql.Rows) (*models.DynamicTool, error) {
	var tool models.DynamicTool
	var configJSON, healthStatusJSON []byte
	var retryPolicyJSON, passthroughConfigJSON sql.NullString

	err := rows.Scan(
		&tool.ID,
		&tool.TenantID,
		&tool.ToolName,
		&tool.DisplayName,
		&configJSON,
		&tool.AuthType,
		&retryPolicyJSON,
		&tool.Status,
		&healthStatusJSON,
		&tool.LastHealthCheck,
		&tool.CreatedAt,
		&tool.UpdatedAt,
		&tool.Provider,
		&passthroughConfigJSON,
	)
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(configJSON, &tool.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	tool.HealthStatus = healthStatusJSON

	if retryPolicyJSON.Valid {
		if err := json.Unmarshal([]byte(retryPolicyJSON.String), &tool.RetryPolicy); err != nil {
			s.logger.Warn("Failed to unmarshal retry policy", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	if passthroughConfigJSON.Valid {
		if err := json.Unmarshal([]byte(passthroughConfigJSON.String), &tool.PassthroughConfig); err != nil {
			s.logger.Warn("Failed to unmarshal passthrough config", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return &tool, nil
}
