package auth

import (
	"os"

	"github.com/pkg/errors"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// AuthMode represents the authentication mode
type AuthMode string

const (
	AuthModeProduction AuthMode = "production"
	AuthModeTest       AuthMode = "test"
)

// FactoryConfig contains configuration for the auth factory
type FactoryConfig struct {
	Mode             AuthMode
	ProductionConfig *AuthConfig
	Logger           observability.Logger
	Tracer           observability.StartSpanFunc
}

// NewAuthorizer creates the appropriate authorizer based on configuration
func NewAuthorizer(config FactoryConfig) (Authorizer, error) {
	// Validate configuration
	if config.Logger == nil {
		return nil, errors.New("logger is required")
	}
	
	if config.Tracer == nil {
		return nil, errors.New("tracer is required")
	}
	
	// Determine auth mode from environment if not explicitly set
	if config.Mode == "" {
		config.Mode = determineAuthMode()
	}
	
	// Log the auth mode with security implications
	config.Logger.Info("Initializing authorization system", map[string]interface{}{
		"mode":                    config.Mode,
		"mcp_test_mode":          os.Getenv("MCP_TEST_MODE"),
		"test_auth_enabled":      os.Getenv("TEST_AUTH_ENABLED"),
		"environment":            os.Getenv("ENVIRONMENT"),
		"security_implications":  getSecurityImplications(config.Mode),
	})
	
	switch config.Mode {
	case AuthModeProduction:
		return createProductionAuthorizer(config)
	case AuthModeTest:
		return createTestAuthorizer(config)
	default:
		return nil, errors.Errorf("unknown auth mode: %s", config.Mode)
	}
}

// determineAuthMode determines the auth mode from environment variables
func determineAuthMode() AuthMode {
	// Check if we're in test mode
	if os.Getenv("MCP_TEST_MODE") == "true" && os.Getenv("TEST_AUTH_ENABLED") == "true" {
		return AuthModeTest
	}
	
	// Default to production
	return AuthModeProduction
}

// createProductionAuthorizer creates a production authorizer
func createProductionAuthorizer(config FactoryConfig) (Authorizer, error) {
	// Ensure we're not in test mode
	if os.Getenv("MCP_TEST_MODE") == "true" {
		return nil, errors.New("production authorizer cannot be used in test mode")
	}
	
	// Validate production config
	if config.ProductionConfig == nil {
		return nil, errors.New("production config is required for production mode")
	}
	
	// Create production authorizer
	authorizer, err := NewProductionAuthorizer(*config.ProductionConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create production authorizer")
	}
	
	config.Logger.Info("Production authorizer created successfully", map[string]interface{}{
		"cache_enabled":   config.ProductionConfig.CacheEnabled,
		"cache_duration":  config.ProductionConfig.CacheDuration,
		"model_path":      config.ProductionConfig.ModelPath,
		"policy_path":     config.ProductionConfig.PolicyPath,
	})
	
	return authorizer, nil
}

// createTestAuthorizer creates a test authorizer
func createTestAuthorizer(config FactoryConfig) (Authorizer, error) {
	// Validate test mode environment
	if os.Getenv("MCP_TEST_MODE") != "true" {
		return nil, errors.New("test mode must be explicitly enabled with MCP_TEST_MODE=true")
	}
	
	if os.Getenv("TEST_AUTH_ENABLED") != "true" {
		return nil, errors.New("test auth must be explicitly enabled with TEST_AUTH_ENABLED=true")
	}
	
	// Warn if in production environment
	if os.Getenv("ENVIRONMENT") == "production" {
		config.Logger.Warn("WARNING: Test authorizer being used in production environment!", map[string]interface{}{
			"security_risk": "HIGH",
			"action":        "Review configuration immediately",
		})
	}
	
	// Create test provider
	testProvider, err := NewTestProvider(config.Logger, config.Tracer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test provider")
	}
	
	config.Logger.Info("Test authorizer created successfully", map[string]interface{}{
		"rate_limit":     "100/minute",
		"token_expiry":   "1h",
		"security_mode":  "test",
	})
	
	return testProvider, nil
}

// getSecurityImplications returns security implications for the auth mode
func getSecurityImplications(mode AuthMode) string {
	switch mode {
	case AuthModeProduction:
		return "Full security enforced with RBAC policies"
	case AuthModeTest:
		return "Reduced security for testing - DO NOT USE IN PRODUCTION"
	default:
		return "Unknown security implications"
	}
}

// ValidateAuthConfiguration validates that auth configuration is consistent across services
func ValidateAuthConfiguration(logger observability.Logger) error {
	mode := determineAuthMode()
	
	// Check for conflicting configuration
	if os.Getenv("ENVIRONMENT") == "production" && mode == AuthModeTest {
		return errors.New("test auth mode cannot be used in production environment")
	}
	
	// Check for required environment variables in production
	if mode == AuthModeProduction {
		requiredVars := []string{
			"JWT_SECRET",
			"DATABASE_URL", // For policy storage
		}
		
		for _, v := range requiredVars {
			if os.Getenv(v) == "" {
				return errors.Errorf("required environment variable %s not set", v)
			}
		}
	}
	
	// Log configuration for debugging
	logger.Info("Auth configuration validated", map[string]interface{}{
		"mode":              mode,
		"environment":       os.Getenv("ENVIRONMENT"),
		"mcp_test_mode":     os.Getenv("MCP_TEST_MODE"),
		"test_auth_enabled": os.Getenv("TEST_AUTH_ENABLED"),
	})
	
	return nil
}