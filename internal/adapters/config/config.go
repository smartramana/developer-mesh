package config

import (
	"time"
	
	"github.com/S-Corkum/mcp-server/internal/adapters/resilience"
)

// AdapterConfig represents the configuration for an adapter
type AdapterConfig struct {
	// Basic settings
	Type          string                 `yaml:"type" json:"type"`
	Enabled       bool                   `yaml:"enabled" json:"enabled"`
	Settings      map[string]interface{} `yaml:"settings" json:"settings"`
	
	// Resilience settings
	Resilience    ResilienceConfig       `yaml:"resilience" json:"resilience"`
	
	// Security settings
	Security      SecurityConfig         `yaml:"security" json:"security"`
	
	// Observability settings
	Observability ObservabilityConfig    `yaml:"observability" json:"observability"`
}

// ResilienceConfig configures resilience patterns
type ResilienceConfig struct {
	Retry           RetryConfig           `yaml:"retry" json:"retry"`
	CircuitBreaker  CircuitBreakerConfig  `yaml:"circuit_breaker" json:"circuit_breaker"`
	RateLimiter     RateLimiterConfig     `yaml:"rate_limiter" json:"rate_limiter"`
	Timeout         TimeoutConfig         `yaml:"timeout" json:"timeout"`
	Bulkhead        BulkheadConfig        `yaml:"bulkhead" json:"bulkhead"`
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	MaxRetries       int           `yaml:"max_retries" json:"max_retries"`
	InitialInterval  time.Duration `yaml:"initial_interval" json:"initial_interval"`
	MaxInterval      time.Duration `yaml:"max_interval" json:"max_interval"`
	Multiplier       float64       `yaml:"multiplier" json:"multiplier"`
	MaxElapsedTime   time.Duration `yaml:"max_elapsed_time" json:"max_elapsed_time"`
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	MaxRequests      uint32        `yaml:"max_requests" json:"max_requests"`
	Interval         time.Duration `yaml:"interval" json:"interval"`
	Timeout          time.Duration `yaml:"timeout" json:"timeout"`
	FailureRatio     float64       `yaml:"failure_ratio" json:"failure_ratio"`
}

// RateLimiterConfig configures rate limiter behavior
type RateLimiterConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	Rate             float64       `yaml:"rate" json:"rate"`
	Burst            int           `yaml:"burst" json:"burst"`
	WaitLimit        time.Duration `yaml:"wait_limit" json:"wait_limit"`
}

// TimeoutConfig configures timeout behavior
type TimeoutConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	Timeout          time.Duration `yaml:"timeout" json:"timeout"`
	GracePeriod      time.Duration `yaml:"grace_period" json:"grace_period"`
}

// BulkheadConfig configures bulkhead behavior
type BulkheadConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	MaxConcurrent    int           `yaml:"max_concurrent" json:"max_concurrent"`
	MaxWaitingTime   time.Duration `yaml:"max_waiting_time" json:"max_waiting_time"`
}

// SecurityConfig configures security settings
type SecurityConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	Authentication   AuthConfig    `yaml:"authentication" json:"authentication"`
	TLS              TLSConfig     `yaml:"tls" json:"tls"`
}

// AuthConfig configures authentication settings
type AuthConfig struct {
	Type             string                 `yaml:"type" json:"type"`
	TokenRefresh     bool                   `yaml:"token_refresh" json:"token_refresh"`
	TokenTTL         time.Duration          `yaml:"token_ttl" json:"token_ttl"`
	Settings         map[string]interface{} `yaml:"settings" json:"settings"`
}

// TLSConfig configures TLS settings
type TLSConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	VerifyCert       bool          `yaml:"verify_cert" json:"verify_cert"`
	CertPath         string        `yaml:"cert_path" json:"cert_path"`
	KeyPath          string        `yaml:"key_path" json:"key_path"`
	CAPath           string        `yaml:"ca_path" json:"ca_path"`
}

// ObservabilityConfig configures observability settings
type ObservabilityConfig struct {
	Enabled          bool            `yaml:"enabled" json:"enabled"`
	Metrics          MetricsConfig   `yaml:"metrics" json:"metrics"`
	Tracing          TracingConfig   `yaml:"tracing" json:"tracing"`
	Logging          LoggingConfig   `yaml:"logging" json:"logging"`
}

// MetricsConfig configures metrics settings
type MetricsConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	IncludeOperations bool         `yaml:"include_operations" json:"include_operations"`
	IncludeResults   bool          `yaml:"include_results" json:"include_results"`
	SampleRate       float64       `yaml:"sample_rate" json:"sample_rate"`
}

// TracingConfig configures tracing settings
type TracingConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	SampleRate       float64       `yaml:"sample_rate" json:"sample_rate"`
	IncludePayloads  bool          `yaml:"include_payloads" json:"include_payloads"`
}

// LoggingConfig configures logging settings
type LoggingConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	Level            string        `yaml:"level" json:"level"`
	IncludeOperations bool         `yaml:"include_operations" json:"include_operations"`
	IncludeResults   bool          `yaml:"include_results" json:"include_results"`
	SampleRate       float64       `yaml:"sample_rate" json:"sample_rate"`
}

// GetRetryConfig converts the configuration to a resilience.RetryConfig
func (c *RetryConfig) GetRetryConfig() resilience.RetryConfig {
	if !c.Enabled {
		return resilience.RetryConfig{MaxRetries: 0}
	}
	
	return resilience.RetryConfig{
		MaxRetries:      c.MaxRetries,
		InitialInterval: c.InitialInterval,
		MaxInterval:     c.MaxInterval,
		Multiplier:      c.Multiplier,
		MaxElapsedTime:  c.MaxElapsedTime,
	}
}

// GetCircuitBreakerConfig converts the configuration to a resilience.CircuitBreakerConfig
func (c *CircuitBreakerConfig) GetCircuitBreakerConfig(name string) resilience.CircuitBreakerConfig {
	if !c.Enabled {
		return resilience.CircuitBreakerConfig{Name: name, MaxRequests: 0}
	}
	
	return resilience.CircuitBreakerConfig{
		Name:         name,
		MaxRequests:  c.MaxRequests,
		Interval:     c.Interval,
		Timeout:      c.Timeout,
		ReadyToTrip: func(counts resilience.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= c.FailureRatio
		},
	}
}

// GetRateLimiterConfig converts the configuration to a resilience.RateLimiterConfig
func (c *RateLimiterConfig) GetRateLimiterConfig(name string) resilience.RateLimiterConfig {
	if !c.Enabled {
		return resilience.RateLimiterConfig{Name: name, Rate: 0}
	}
	
	return resilience.RateLimiterConfig{
		Name:      name,
		Rate:      c.Rate,
		Burst:     c.Burst,
		WaitLimit: c.WaitLimit,
	}
}

// GetTimeoutConfig converts the configuration to a resilience.TimeoutConfig
func (c *TimeoutConfig) GetTimeoutConfig() resilience.TimeoutConfig {
	if !c.Enabled {
		return resilience.TimeoutConfig{Timeout: 0}
	}
	
	return resilience.TimeoutConfig{
		Timeout:     c.Timeout,
		GracePeriod: c.GracePeriod,
	}
}

// GetBulkheadConfig converts the configuration to a resilience.BulkheadConfig
func (c *BulkheadConfig) GetBulkheadConfig(name string) resilience.BulkheadConfig {
	if !c.Enabled {
		return resilience.BulkheadConfig{Name: name, MaxConcurrent: 0}
	}
	
	return resilience.BulkheadConfig{
		Name:           name,
		MaxConcurrent:  c.MaxConcurrent,
		MaxWaitingTime: c.MaxWaitingTime,
	}
}

// DefaultAdapterConfig returns a default adapter configuration
func DefaultAdapterConfig() AdapterConfig {
	return AdapterConfig{
		Enabled: true,
		Settings: make(map[string]interface{}),
		Resilience: ResilienceConfig{
			Retry: RetryConfig{
				Enabled:         true,
				MaxRetries:      3,
				InitialInterval: 100 * time.Millisecond,
				MaxInterval:     10 * time.Second,
				Multiplier:      2.0,
				MaxElapsedTime:  30 * time.Second,
			},
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:      true,
				MaxRequests:  1,
				Interval:     30 * time.Second,
				Timeout:      60 * time.Second,
				FailureRatio: 0.5,
			},
			RateLimiter: RateLimiterConfig{
				Enabled:   true,
				Rate:      100,
				Burst:     10,
				WaitLimit: 5 * time.Second,
			},
			Timeout: TimeoutConfig{
				Enabled:     true,
				Timeout:     10 * time.Second,
				GracePeriod: 2 * time.Second,
			},
			Bulkhead: BulkheadConfig{
				Enabled:        true,
				MaxConcurrent:  10,
				MaxWaitingTime: 5 * time.Second,
			},
		},
		Security: SecurityConfig{
			Enabled: true,
			Authentication: AuthConfig{
				Type:         "api_key",
				TokenRefresh: false,
				TokenTTL:     24 * time.Hour,
				Settings:     make(map[string]interface{}),
			},
			TLS: TLSConfig{
				Enabled:    false,
				VerifyCert: true,
			},
		},
		Observability: ObservabilityConfig{
			Enabled: true,
			Metrics: MetricsConfig{
				Enabled:          true,
				IncludeOperations: true,
				IncludeResults:   false,
				SampleRate:       1.0,
			},
			Tracing: TracingConfig{
				Enabled:         true,
				SampleRate:      0.1,
				IncludePayloads: false,
			},
			Logging: LoggingConfig{
				Enabled:          true,
				Level:            "info",
				IncludeOperations: false,
				IncludeResults:   false,
				SampleRate:       0.1,
			},
		},
	}
}

// ConfigValidator validates adapter configurations
type ConfigValidator interface {
	// Validate validates a configuration
	Validate(config AdapterConfig) (bool, []string)
}

// DefaultConfigValidator is the default implementation of ConfigValidator
type DefaultConfigValidator struct{}

// Validate validates a configuration
func (v *DefaultConfigValidator) Validate(config AdapterConfig) (bool, []string) {
	var errors []string
	
	// Validate basic settings
	if config.Type == "" {
		errors = append(errors, "adapter type is required")
	}
	
	// Validate retry settings
	if config.Resilience.Retry.Enabled {
		if config.Resilience.Retry.MaxRetries < 0 {
			errors = append(errors, "max retries cannot be negative")
		}
		if config.Resilience.Retry.InitialInterval <= 0 {
			errors = append(errors, "initial interval must be positive")
		}
		if config.Resilience.Retry.MaxInterval <= 0 {
			errors = append(errors, "max interval must be positive")
		}
		if config.Resilience.Retry.Multiplier <= 0 {
			errors = append(errors, "multiplier must be positive")
		}
	}
	
	// Validate circuit breaker settings
	if config.Resilience.CircuitBreaker.Enabled {
		if config.Resilience.CircuitBreaker.MaxRequests <= 0 {
			errors = append(errors, "max requests must be positive")
		}
		if config.Resilience.CircuitBreaker.Timeout <= 0 {
			errors = append(errors, "timeout must be positive")
		}
		if config.Resilience.CircuitBreaker.FailureRatio <= 0 || config.Resilience.CircuitBreaker.FailureRatio > 1 {
			errors = append(errors, "failure ratio must be between 0 and 1")
		}
	}
	
	// Validate rate limiter settings
	if config.Resilience.RateLimiter.Enabled {
		if config.Resilience.RateLimiter.Rate <= 0 {
			errors = append(errors, "rate must be positive")
		}
		if config.Resilience.RateLimiter.Burst <= 0 {
			errors = append(errors, "burst must be positive")
		}
	}
	
	// Validate timeout settings
	if config.Resilience.Timeout.Enabled {
		if config.Resilience.Timeout.Timeout <= 0 {
			errors = append(errors, "timeout must be positive")
		}
	}
	
	// Validate bulkhead settings
	if config.Resilience.Bulkhead.Enabled {
		if config.Resilience.Bulkhead.MaxConcurrent <= 0 {
			errors = append(errors, "max concurrent must be positive")
		}
	}
	
	return len(errors) == 0, errors
}
