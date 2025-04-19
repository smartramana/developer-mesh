package github

import (
	"time"
)

// Config holds configuration for the GitHub adapter
type Config struct {
	// Authentication settings
	Token          string `mapstructure:"token"`
	AppID          string `mapstructure:"app_id"`
	AppPrivateKey  string `mapstructure:"app_private_key"`
	UseApp         bool   `mapstructure:"use_app"`
	
	// Connection settings
	BaseURL            string        `mapstructure:"base_url"`
	UploadURL          string        `mapstructure:"upload_url"`
	RequestTimeout     time.Duration `mapstructure:"request_timeout"`
	MaxIdleConns       int           `mapstructure:"max_idle_conns"`
	MaxConnsPerHost    int           `mapstructure:"max_conns_per_host"`
	MaxIdleConnsPerHost int          `mapstructure:"max_idle_conns_per_host"`
	IdleConnTimeout    time.Duration `mapstructure:"idle_conn_timeout"`
	
	// Rate limiting settings
	RateLimit        bool   `mapstructure:"rate_limit"`
	RateLimitPerHour int    `mapstructure:"rate_limit_per_hour"`
}

// DefaultConfig returns a default configuration for the GitHub adapter
func DefaultConfig() *Config {
	return &Config{
		// Default to GitHub.com
		BaseURL:         "https://api.github.com/",
		UploadURL:       "https://uploads.github.com/",
		
		// Default connection settings
		RequestTimeout:      30 * time.Second,
		MaxIdleConns:        10,
		MaxConnsPerHost:     10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		
		// Default rate limiting
		RateLimit:        true,
		RateLimitPerHour: 5000, // Default GitHub API rate limit
	}
}
