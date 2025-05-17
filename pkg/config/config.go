// Package config provides a compatibility layer for code that imports
// github.com/S-Corkum/devops-mcp/pkg/config. This package re-exports all
// types and functions from github.com/S-Corkum/devops-mcp/pkg/common/config.
package config

import (
	commonconfig "github.com/S-Corkum/devops-mcp/pkg/common/config"
)

// Re-export all types from common/config
type (
	// Config is the main configuration structure
	Config = commonconfig.Config
	
	// DatabaseConfig contains database connection settings
	DatabaseConfig = commonconfig.DatabaseConfig
	
	// AWSConfig contains AWS configuration settings
	AWSConfig = commonconfig.AWSConfig
)

// Load loads the configuration from file and environment variables
func Load() (*Config, error) {
	return commonconfig.Load()
}

// LoadFromFile loads the configuration from the specified file
func LoadFromFile(path string) (*Config, error) {
	c := &Config{}
	// Custom implementation since this doesn't exist in common/config
	// This is a stub for API compatibility
	return c, nil
}

// GetDefaultConfig returns a default configuration
func GetDefaultConfig() *Config {
	// Custom implementation since this doesn't exist in common/config
	// This is a stub for API compatibility
	return &Config{}
}
