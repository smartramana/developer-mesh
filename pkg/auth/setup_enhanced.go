package auth

import (
    "fmt"
    "os"
    "strconv"
    "time"
    
    "github.com/S-Corkum/devops-mcp/pkg/common/cache"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
    "github.com/jmoiron/sqlx"
)

// SetupAuthenticationWithConfig provides full control over auth system initialization
func SetupAuthenticationWithConfig(
    config *AuthSystemConfig,
    db *sqlx.DB,
    cache cache.Cache,
    logger observability.Logger,
    metrics observability.MetricsClient,
) (*AuthMiddleware, error) {
    // Input validation
    if config == nil {
        return nil, fmt.Errorf("config cannot be nil")
    }
    if logger == nil {
        return nil, fmt.Errorf("logger cannot be nil")
    }
    if metrics == nil {
        return nil, fmt.Errorf("metrics client cannot be nil")
    }
    
    // Apply defaults
    if config.Service == nil {
        config.Service = DefaultConfig()
    }
    if config.RateLimiter == nil {
        config.RateLimiter = DefaultRateLimiterConfig()
    }
    
    // Create auth service
    authService := NewService(config.Service, db, cache, logger)
    
    // Load API keys into the service
    for key, settings := range config.APIKeys {
        if err := authService.AddAPIKey(key, settings); err != nil {
            // Log but don't fail - allows partial key loading
            logger.Warn("Failed to add API key", map[string]interface{}{
                "key_suffix": lastN(key, 4), // Security: only log last 4 chars
                "error":      err.Error(),
            })
        }
    }
    
    // Load keys from environment if no API keys were provided in config
    if len(config.APIKeys) == 0 {
        if err := authService.LoadAuthConfigBasedOnEnvironment(); err != nil {
            logger.Warn("Failed to load auth config from environment", map[string]interface{}{
                "error": err.Error(),
            })
        }
    }
    
    // Create components with injected config
    rateLimiter := NewRateLimiter(cache, logger, config.RateLimiter)
    metricsCollector := NewMetricsCollector(metrics)
    auditLogger := NewAuditLogger(logger)
    
    // Create middleware with all components
    middleware := NewAuthMiddleware(authService, rateLimiter, metricsCollector, auditLogger)
    
    logger.Info("Authentication system initialized", map[string]interface{}{
        "api_keys_loaded":   len(config.APIKeys),
        "rate_limit_max":    config.RateLimiter.MaxAttempts,
        "cache_enabled":     config.Service.CacheEnabled,
        "jwt_enabled":       config.Service.EnableJWT,
    })
    
    return middleware, nil
}

// SetupEnhancedAuthentication maintains backward compatibility for enhanced auth
func SetupEnhancedAuthentication(
    db *sqlx.DB,
    cache cache.Cache,
    logger observability.Logger,
    metrics observability.MetricsClient,
) (*AuthMiddleware, error) {
    // Build configuration from environment
    config := &AuthSystemConfig{
        Service:     buildServiceConfigFromEnv(),
        RateLimiter: buildRateLimiterConfigFromEnv(),
        APIKeys:     make(map[string]APIKeySettings),
    }
    
    return SetupAuthenticationWithConfig(config, db, cache, logger, metrics)
}

// Helper functions
func buildServiceConfigFromEnv() *ServiceConfig {
    config := DefaultConfig()
    
    // JWT configuration
    if secret := os.Getenv("JWT_SECRET"); secret != "" {
        config.JWTSecret = secret
    }
    if exp := os.Getenv("JWT_EXPIRATION"); exp != "" {
        if duration, err := time.ParseDuration(exp); err == nil {
            config.JWTExpiration = duration
        }
    }
    
    // Cache configuration
    if enabled := os.Getenv("AUTH_CACHE_ENABLED"); enabled == "false" {
        config.CacheEnabled = false
    }
    
    return config
}

func buildRateLimiterConfigFromEnv() *RateLimiterConfig {
    config := DefaultRateLimiterConfig()
    
    if attempts := os.Getenv("RATE_LIMIT_MAX_ATTEMPTS"); attempts != "" {
        if val, err := strconv.Atoi(attempts); err == nil && val > 0 {
            config.MaxAttempts = val
        }
    }
    
    if window := os.Getenv("RATE_LIMIT_WINDOW"); window != "" {
        if duration, err := time.ParseDuration(window); err == nil {
            config.WindowSize = duration
        }
    }
    
    return config
}

func lastN(s string, n int) string {
    if len(s) <= n {
        return s
    }
    return s[len(s)-n:]
}