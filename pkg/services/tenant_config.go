package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/common/cache"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/pkg/errors"
)

// TenantConfigService handles tenant configuration management
type TenantConfigService interface {
	GetConfig(ctx context.Context, tenantID string) (*models.TenantConfig, error)
	CreateConfig(ctx context.Context, config *models.TenantConfig) error
	UpdateConfig(ctx context.Context, config *models.TenantConfig) error
	DeleteConfig(ctx context.Context, tenantID string) error

	// Service token management
	SetServiceToken(ctx context.Context, tenantID, provider, token string) error
	RemoveServiceToken(ctx context.Context, tenantID, provider string) error

	// Feature flag management
	SetFeature(ctx context.Context, tenantID, feature string, value interface{}) error
	IsFeatureEnabled(ctx context.Context, tenantID, feature string) (bool, error)

	// Rate limit management
	SetRateLimitForKeyType(ctx context.Context, tenantID, keyType string, limit models.KeyTypeRateLimit) error
	SetRateLimitForEndpoint(ctx context.Context, tenantID, endpoint string, limit models.EndpointRateLimit) error
}

type tenantConfigService struct {
	repo       repository.TenantConfigRepository
	cache      cache.Cache
	encryption EncryptionService
	logger     observability.Logger
	cacheTTL   time.Duration
}

// NewTenantConfigService creates a new tenant configuration service
func NewTenantConfigService(
	repo repository.TenantConfigRepository,
	cache cache.Cache,
	encryption EncryptionService,
	logger observability.Logger,
) TenantConfigService {
	return &tenantConfigService{
		repo:       repo,
		cache:      cache,
		encryption: encryption,
		logger:     logger,
		cacheTTL:   5 * time.Minute,
	}
}

// GetConfig retrieves tenant configuration with caching
func (s *tenantConfigService) GetConfig(ctx context.Context, tenantID string) (*models.TenantConfig, error) {
	ctx, span := observability.StartSpan(ctx, "service.tenant_config.GetConfig")
	defer span.End()

	// Check cache first
	cacheKey := fmt.Sprintf("tenant:config:%s", tenantID)
	var config models.TenantConfig

	if s.cache != nil {
		if err := s.cache.Get(ctx, cacheKey, &config); err == nil {
			s.logger.Debug("Tenant config retrieved from cache", map[string]interface{}{
				"tenant_id": tenantID,
			})
			return &config, nil
		}
	}

	// Get from repository
	dbConfig, err := s.repo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant config from repository")
	}

	// If not found, return default config
	if dbConfig == nil {
		s.logger.Info("Tenant config not found, returning default", map[string]interface{}{
			"tenant_id": tenantID,
		})
		return models.DefaultTenantConfig(tenantID), nil
	}

	// Decrypt service tokens
	if err := s.decryptServiceTokens(ctx, dbConfig); err != nil {
		s.logger.Error("Failed to decrypt service tokens", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		// Continue without tokens rather than failing completely
		dbConfig.ServiceTokens = make(map[string]string)
	}

	// Cache the config
	if s.cache != nil {
		if err := s.cache.Set(ctx, cacheKey, dbConfig, s.cacheTTL); err != nil {
			s.logger.Warn("Failed to cache tenant config", map[string]interface{}{
				"tenant_id": tenantID,
				"error":     err.Error(),
			})
		}
	}

	return dbConfig, nil
}

// CreateConfig creates a new tenant configuration
func (s *tenantConfigService) CreateConfig(ctx context.Context, config *models.TenantConfig) error {
	ctx, span := observability.StartSpan(ctx, "service.tenant_config.CreateConfig")
	defer span.End()

	// Encrypt service tokens before saving
	if err := s.encryptServiceTokens(ctx, config); err != nil {
		return errors.Wrap(err, "failed to encrypt service tokens")
	}

	// Create in repository
	if err := s.repo.Create(ctx, config); err != nil {
		return errors.Wrap(err, "failed to create tenant config")
	}

	// Invalidate any existing cache
	if s.cache != nil {
		cacheKey := fmt.Sprintf("tenant:config:%s", config.TenantID)
		_ = s.cache.Delete(ctx, cacheKey)
	}

	s.logger.Info("Created tenant config", map[string]interface{}{
		"tenant_id": config.TenantID,
	})

	return nil
}

// UpdateConfig updates an existing tenant configuration
func (s *tenantConfigService) UpdateConfig(ctx context.Context, config *models.TenantConfig) error {
	ctx, span := observability.StartSpan(ctx, "service.tenant_config.UpdateConfig")
	defer span.End()

	// Encrypt service tokens before saving
	if err := s.encryptServiceTokens(ctx, config); err != nil {
		return errors.Wrap(err, "failed to encrypt service tokens")
	}

	// Update in repository
	if err := s.repo.Update(ctx, config); err != nil {
		return errors.Wrap(err, "failed to update tenant config")
	}

	// Invalidate cache
	if s.cache != nil {
		cacheKey := fmt.Sprintf("tenant:config:%s", config.TenantID)
		_ = s.cache.Delete(ctx, cacheKey)
	}

	s.logger.Info("Updated tenant config", map[string]interface{}{
		"tenant_id": config.TenantID,
	})

	return nil
}

// DeleteConfig deletes a tenant configuration
func (s *tenantConfigService) DeleteConfig(ctx context.Context, tenantID string) error {
	ctx, span := observability.StartSpan(ctx, "service.tenant_config.DeleteConfig")
	defer span.End()

	// Delete from repository
	if err := s.repo.Delete(ctx, tenantID); err != nil {
		return errors.Wrap(err, "failed to delete tenant config")
	}

	// Invalidate cache
	if s.cache != nil {
		cacheKey := fmt.Sprintf("tenant:config:%s", tenantID)
		_ = s.cache.Delete(ctx, cacheKey)
	}

	s.logger.Info("Deleted tenant config", map[string]interface{}{
		"tenant_id": tenantID,
	})

	return nil
}

// SetServiceToken sets an encrypted service token for a provider
func (s *tenantConfigService) SetServiceToken(ctx context.Context, tenantID, provider, token string) error {
	ctx, span := observability.StartSpan(ctx, "service.tenant_config.SetServiceToken")
	defer span.End()

	// Get current config
	config, err := s.GetConfig(ctx, tenantID)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant config")
	}

	// Initialize service tokens map if needed
	if config.ServiceTokens == nil {
		config.ServiceTokens = make(map[string]string)
	}

	// Set the token
	config.ServiceTokens[provider] = token

	// Update the config
	if err := s.UpdateConfig(ctx, config); err != nil {
		return errors.Wrap(err, "failed to update config with new token")
	}

	s.logger.Info("Set service token", map[string]interface{}{
		"tenant_id": tenantID,
		"provider":  provider,
	})

	return nil
}

// RemoveServiceToken removes a service token for a provider
func (s *tenantConfigService) RemoveServiceToken(ctx context.Context, tenantID, provider string) error {
	ctx, span := observability.StartSpan(ctx, "service.tenant_config.RemoveServiceToken")
	defer span.End()

	// Get current config
	config, err := s.GetConfig(ctx, tenantID)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant config")
	}

	// Remove the token
	delete(config.ServiceTokens, provider)

	// Update the config
	if err := s.UpdateConfig(ctx, config); err != nil {
		return errors.Wrap(err, "failed to update config")
	}

	s.logger.Info("Removed service token", map[string]interface{}{
		"tenant_id": tenantID,
		"provider":  provider,
	})

	return nil
}

// SetFeature sets a feature flag value
func (s *tenantConfigService) SetFeature(ctx context.Context, tenantID, feature string, value interface{}) error {
	ctx, span := observability.StartSpan(ctx, "service.tenant_config.SetFeature")
	defer span.End()

	// Get current config
	config, err := s.GetConfig(ctx, tenantID)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant config")
	}

	// Initialize features map if needed
	if config.Features == nil {
		config.Features = make(map[string]interface{})
	}

	// Set the feature
	config.Features[feature] = value

	// Update the config
	if err := s.UpdateConfig(ctx, config); err != nil {
		return errors.Wrap(err, "failed to update config")
	}

	s.logger.Info("Set feature flag", map[string]interface{}{
		"tenant_id": tenantID,
		"feature":   feature,
		"value":     value,
	})

	return nil
}

// IsFeatureEnabled checks if a feature is enabled
func (s *tenantConfigService) IsFeatureEnabled(ctx context.Context, tenantID, feature string) (bool, error) {
	ctx, span := observability.StartSpan(ctx, "service.tenant_config.IsFeatureEnabled")
	defer span.End()

	config, err := s.GetConfig(ctx, tenantID)
	if err != nil {
		return false, errors.Wrap(err, "failed to get tenant config")
	}

	return config.IsFeatureEnabled(feature), nil
}

// SetRateLimitForKeyType sets rate limits for a specific key type
func (s *tenantConfigService) SetRateLimitForKeyType(ctx context.Context, tenantID, keyType string, limit models.KeyTypeRateLimit) error {
	ctx, span := observability.StartSpan(ctx, "service.tenant_config.SetRateLimitForKeyType")
	defer span.End()

	// Get current config
	config, err := s.GetConfig(ctx, tenantID)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant config")
	}

	// Initialize overrides map if needed
	if config.RateLimitConfig.KeyTypeOverrides == nil {
		config.RateLimitConfig.KeyTypeOverrides = make(map[string]models.KeyTypeRateLimit)
	}

	// Set the rate limit
	config.RateLimitConfig.KeyTypeOverrides[keyType] = limit

	// Update the config
	if err := s.UpdateConfig(ctx, config); err != nil {
		return errors.Wrap(err, "failed to update config")
	}

	s.logger.Info("Set rate limit for key type", map[string]interface{}{
		"tenant_id": tenantID,
		"key_type":  keyType,
		"limit":     limit,
	})

	return nil
}

// SetRateLimitForEndpoint sets rate limits for a specific endpoint
func (s *tenantConfigService) SetRateLimitForEndpoint(ctx context.Context, tenantID, endpoint string, limit models.EndpointRateLimit) error {
	ctx, span := observability.StartSpan(ctx, "service.tenant_config.SetRateLimitForEndpoint")
	defer span.End()

	// Get current config
	config, err := s.GetConfig(ctx, tenantID)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant config")
	}

	// Initialize overrides map if needed
	if config.RateLimitConfig.EndpointOverrides == nil {
		config.RateLimitConfig.EndpointOverrides = make(map[string]models.EndpointRateLimit)
	}

	// Set the rate limit
	config.RateLimitConfig.EndpointOverrides[endpoint] = limit

	// Update the config
	if err := s.UpdateConfig(ctx, config); err != nil {
		return errors.Wrap(err, "failed to update config")
	}

	s.logger.Info("Set rate limit for endpoint", map[string]interface{}{
		"tenant_id": tenantID,
		"endpoint":  endpoint,
		"limit":     limit,
	})

	return nil
}

// encryptServiceTokens encrypts the service tokens before storage
func (s *tenantConfigService) encryptServiceTokens(ctx context.Context, config *models.TenantConfig) error {
	if len(config.ServiceTokens) == 0 {
		config.EncryptedTokens = json.RawMessage("{}")
		return nil
	}

	// Marshal tokens to JSON
	tokensJSON, err := json.Marshal(config.ServiceTokens)
	if err != nil {
		return errors.Wrap(err, "failed to marshal service tokens")
	}

	// Encrypt the JSON
	if s.encryption != nil {
		encrypted, err := s.encryption.Encrypt(ctx, tokensJSON)
		if err != nil {
			return errors.Wrap(err, "failed to encrypt service tokens")
		}
		config.EncryptedTokens = json.RawMessage(encrypted)
	} else {
		// No encryption service, store as-is (for development only)
		s.logger.Warn("No encryption service configured, storing tokens unencrypted", map[string]interface{}{})
		config.EncryptedTokens = json.RawMessage(tokensJSON)
	}

	return nil
}

// decryptServiceTokens decrypts the service tokens after retrieval
func (s *tenantConfigService) decryptServiceTokens(ctx context.Context, config *models.TenantConfig) error {
	config.ServiceTokens = make(map[string]string)

	if len(config.EncryptedTokens) == 0 || string(config.EncryptedTokens) == "{}" {
		return nil
	}

	var decryptedJSON []byte

	// Decrypt if encryption service is available
	if s.encryption != nil {
		decrypted, err := s.encryption.Decrypt(ctx, config.EncryptedTokens)
		if err != nil {
			return errors.Wrap(err, "failed to decrypt service tokens")
		}
		decryptedJSON = decrypted
	} else {
		// No encryption service, assume unencrypted (for development only)
		s.logger.Warn("No encryption service configured, assuming tokens are unencrypted", map[string]interface{}{})
		decryptedJSON = config.EncryptedTokens
	}

	// Unmarshal tokens
	if err := json.Unmarshal(decryptedJSON, &config.ServiceTokens); err != nil {
		return errors.Wrap(err, "failed to unmarshal service tokens")
	}

	return nil
}
