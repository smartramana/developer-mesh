// Package common provides shared utilities and functionality
// for the MCP platform components.
package common

// Version is the semantic version of the common package
const Version = "0.1.0"

// IsProductionEnvironment returns whether the current environment is production
func IsProductionEnvironment(env string) bool {
	return env == "production"
}
