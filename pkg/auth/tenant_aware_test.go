package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/auth"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/services"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock tenant config service
type mockTenantConfigService struct {
	mock.Mock
}

func (m *mockTenantConfigService) GetConfig(ctx context.Context, tenantID string) (*models.TenantConfig, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TenantConfig), args.Error(1)
}

func (m *mockTenantConfigService) CreateConfig(ctx context.Context, config *models.TenantConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *mockTenantConfigService) UpdateConfig(ctx context.Context, config *models.TenantConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *mockTenantConfigService) DeleteConfig(ctx context.Context, tenantID string) error {
	args := m.Called(ctx, tenantID)
	return args.Error(0)
}

func (m *mockTenantConfigService) SetServiceToken(ctx context.Context, tenantID, provider, token string) error {
	args := m.Called(ctx, tenantID, provider, token)
	return args.Error(0)
}

func (m *mockTenantConfigService) RemoveServiceToken(ctx context.Context, tenantID, provider string) error {
	args := m.Called(ctx, tenantID, provider)
	return args.Error(0)
}

func (m *mockTenantConfigService) SetFeature(ctx context.Context, tenantID, feature string, value interface{}) error {
	args := m.Called(ctx, tenantID, feature, value)
	return args.Error(0)
}

func (m *mockTenantConfigService) IsFeatureEnabled(ctx context.Context, tenantID, feature string) (bool, error) {
	args := m.Called(ctx, tenantID, feature)
	return args.Bool(0), args.Error(1)
}

func (m *mockTenantConfigService) SetRateLimitForKeyType(ctx context.Context, tenantID, keyType string, limit models.KeyTypeRateLimit) error {
	args := m.Called(ctx, tenantID, keyType, limit)
	return args.Error(0)
}

func (m *mockTenantConfigService) SetRateLimitForEndpoint(ctx context.Context, tenantID, endpoint string, limit models.EndpointRateLimit) error {
	args := m.Called(ctx, tenantID, endpoint, limit)
	return args.Error(0)
}

// Ensure mock implements the interface
var _ services.TenantConfigService = (*mockTenantConfigService)(nil)

func TestNewTenantAwareService(t *testing.T) {
	authService := &auth.Service{}
	tenantConfigService := &mockTenantConfigService{}

	tas := auth.NewTenantAwareService(authService, tenantConfigService)
	assert.NotNil(t, tas)
}

func TestTenantAwareService_CheckFeatureEnabled(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	feature := "advanced_analytics"

	t.Run("feature enabled", func(t *testing.T) {
		mockTenantConfig := &mockTenantConfigService{}
		authService := &auth.Service{}
		tas := auth.NewTenantAwareService(authService, mockTenantConfig)

		mockTenantConfig.On("IsFeatureEnabled", ctx, tenantID, feature).Return(true, nil)

		enabled, err := tas.CheckFeatureEnabled(ctx, tenantID, feature)
		assert.NoError(t, err)
		assert.True(t, enabled)

		mockTenantConfig.AssertExpectations(t)
	})

	t.Run("feature disabled", func(t *testing.T) {
		mockTenantConfig := &mockTenantConfigService{}
		authService := &auth.Service{}
		tas := auth.NewTenantAwareService(authService, mockTenantConfig)

		mockTenantConfig.On("IsFeatureEnabled", ctx, tenantID, feature).Return(false, nil)

		enabled, err := tas.CheckFeatureEnabled(ctx, tenantID, feature)
		assert.NoError(t, err)
		assert.False(t, enabled)

		mockTenantConfig.AssertExpectations(t)
	})

	t.Run("error checking feature", func(t *testing.T) {
		mockTenantConfig := &mockTenantConfigService{}
		authService := &auth.Service{}
		tas := auth.NewTenantAwareService(authService, mockTenantConfig)

		mockTenantConfig.On("IsFeatureEnabled", ctx, tenantID, feature).
			Return(false, errors.New("service error"))

		enabled, err := tas.CheckFeatureEnabled(ctx, tenantID, feature)
		assert.Error(t, err)
		assert.False(t, enabled)

		mockTenantConfig.AssertExpectations(t)
	})
}

func TestTenantAwareService_GetServiceToken(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	provider := "github"
	token := "ghp_secret_token"

	t.Run("token exists", func(t *testing.T) {
		mockTenantConfig := &mockTenantConfigService{}
		authService := &auth.Service{}
		tas := auth.NewTenantAwareService(authService, mockTenantConfig)

		config := &models.TenantConfig{
			TenantID: tenantID,
			ServiceTokens: map[string]string{
				provider: token,
			},
		}

		mockTenantConfig.On("GetConfig", ctx, tenantID).Return(config, nil)

		resultToken, err := tas.GetServiceToken(ctx, tenantID, provider)
		assert.NoError(t, err)
		assert.Equal(t, token, resultToken)

		mockTenantConfig.AssertExpectations(t)
	})

	t.Run("token not found", func(t *testing.T) {
		mockTenantConfig := &mockTenantConfigService{}
		authService := &auth.Service{}
		tas := auth.NewTenantAwareService(authService, mockTenantConfig)

		config := &models.TenantConfig{
			TenantID:      tenantID,
			ServiceTokens: map[string]string{},
		}

		mockTenantConfig.On("GetConfig", ctx, tenantID).Return(config, nil)

		resultToken, err := tas.GetServiceToken(ctx, tenantID, provider)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no service token found")
		assert.Empty(t, resultToken)

		mockTenantConfig.AssertExpectations(t)
	})

	t.Run("config load error", func(t *testing.T) {
		mockTenantConfig := &mockTenantConfigService{}
		authService := &auth.Service{}
		tas := auth.NewTenantAwareService(authService, mockTenantConfig)

		mockTenantConfig.On("GetConfig", ctx, tenantID).
			Return(nil, errors.New("config error"))

		resultToken, err := tas.GetServiceToken(ctx, tenantID, provider)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get tenant config")
		assert.Empty(t, resultToken)

		mockTenantConfig.AssertExpectations(t)
	})
}

func TestTenantAwareService_GetAllowedOrigins(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-123"
	allowedOrigins := []string{"https://app.example.com", "https://dev.example.com"}

	t.Run("get allowed origins successfully", func(t *testing.T) {
		mockTenantConfig := &mockTenantConfigService{}
		authService := &auth.Service{}
		tas := auth.NewTenantAwareService(authService, mockTenantConfig)

		config := &models.TenantConfig{
			TenantID:       tenantID,
			AllowedOrigins: pq.StringArray(allowedOrigins),
		}

		mockTenantConfig.On("GetConfig", ctx, tenantID).Return(config, nil)

		origins, err := tas.GetAllowedOrigins(ctx, tenantID)
		assert.NoError(t, err)
		assert.Equal(t, allowedOrigins, origins)

		mockTenantConfig.AssertExpectations(t)
	})

	t.Run("config load error", func(t *testing.T) {
		mockTenantConfig := &mockTenantConfigService{}
		authService := &auth.Service{}
		tas := auth.NewTenantAwareService(authService, mockTenantConfig)

		mockTenantConfig.On("GetConfig", ctx, tenantID).
			Return(nil, errors.New("config error"))

		origins, err := tas.GetAllowedOrigins(ctx, tenantID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get tenant config")
		assert.Nil(t, origins)

		mockTenantConfig.AssertExpectations(t)
	})
}

// Integration tests for ValidateAPIKeyWithTenantConfig and ValidateWithEndpointRateLimit
// would require a full auth service setup with database, which is beyond the scope
// of unit tests. These should be tested in integration tests instead.

