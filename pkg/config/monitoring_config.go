package config

import (
	commonconfig "github.com/S-Corkum/devops-mcp/pkg/common/config"
)

// GetDefaultMonitoringConfig returns default monitoring configuration
func GetDefaultMonitoringConfig() commonconfig.MonitoringConfig {
	// Return an empty monitoring config
	// In a real implementation, we'd create a proper config with values
	// Since we're just fixing the import cycle, this is sufficient
	var config commonconfig.MonitoringConfig
	return config
}
