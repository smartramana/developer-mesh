package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// ConfigLoader handles loading and merging configuration files
type ConfigLoader struct {
	configPath string
	envFile    string
	viper      *viper.Viper
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader(configPath string) *ConfigLoader {
	return &ConfigLoader{
		configPath: configPath,
		viper:      viper.New(),
	}
}

// LoadEnvironment loads environment-specific configuration
func (cl *ConfigLoader) LoadEnvironment(environment string) error {
	// Load .env file if it exists
	envFile := fmt.Sprintf(".env.%s", environment)
	if environment == "development" || environment == "" {
		envFile = ".env"
	}
	
	if _, err := os.Stat(envFile); err == nil {
		if err := cl.loadEnvFile(envFile); err != nil {
			return fmt.Errorf("error loading env file %s: %w", envFile, err)
		}
	}

	// Set up viper
	cl.viper.SetConfigType("yaml")
	cl.viper.AutomaticEnv()
	cl.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	
	// Load base configuration first
	baseConfig := filepath.Join(cl.configPath, "config.base.yaml")
	if err := cl.loadConfigFile(baseConfig); err != nil {
		return fmt.Errorf("failed to load base config: %w", err)
	}

	// Load environment-specific configuration
	envConfig := filepath.Join(cl.configPath, fmt.Sprintf("config.%s.yaml", environment))
	if _, err := os.Stat(envConfig); err == nil {
		if err := cl.mergeConfigFile(envConfig); err != nil {
			return fmt.Errorf("failed to load environment config: %w", err)
		}
	}

	// Load local overrides if they exist
	localConfig := filepath.Join(cl.configPath, fmt.Sprintf("config.%s.local.yaml", environment))
	if _, err := os.Stat(localConfig); err == nil {
		if err := cl.mergeConfigFile(localConfig); err != nil {
			return fmt.Errorf("failed to load local config: %w", err)
		}
	}

	return nil
}

// loadEnvFile loads environment variables from a file
func (cl *ConfigLoader) loadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Parse key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Remove quotes if present
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
			   (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		
		// Set environment variable
		os.Setenv(key, value)
	}
	
	return scanner.Err()
}

// loadConfigFile loads a configuration file
func (cl *ConfigLoader) loadConfigFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))
	
	// Parse YAML to handle _base directive
	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal([]byte(expanded), &rawConfig); err != nil {
		return err
	}

	// Check for base configuration
	if base, ok := rawConfig["_base"].(string); ok {
		basePath := filepath.Join(cl.configPath, base)
		if err := cl.loadConfigFile(basePath); err != nil {
			return fmt.Errorf("failed to load base config %s: %w", base, err)
		}
		delete(rawConfig, "_base")
	}

	// Merge configuration
	return cl.viper.MergeConfigMap(rawConfig)
}

// mergeConfigFile merges a configuration file with existing config
func (cl *ConfigLoader) mergeConfigFile(filename string) error {
	return cl.loadConfigFile(filename)
}

// Get returns a configuration value
func (cl *ConfigLoader) Get(key string) interface{} {
	return cl.viper.Get(key)
}

// GetString returns a string configuration value
func (cl *ConfigLoader) GetString(key string) string {
	return cl.viper.GetString(key)
}

// GetInt returns an integer configuration value
func (cl *ConfigLoader) GetInt(key string) int {
	return cl.viper.GetInt(key)
}

// GetBool returns a boolean configuration value
func (cl *ConfigLoader) GetBool(key string) bool {
	return cl.viper.GetBool(key)
}

// GetStringSlice returns a string slice configuration value
func (cl *ConfigLoader) GetStringSlice(key string) []string {
	return cl.viper.GetStringSlice(key)
}

// GetStringMap returns a string map configuration value
func (cl *ConfigLoader) GetStringMap(key string) map[string]interface{} {
	return cl.viper.GetStringMap(key)
}

// UnmarshalKey unmarshals a specific configuration section
func (cl *ConfigLoader) UnmarshalKey(key string, rawVal interface{}) error {
	return cl.viper.UnmarshalKey(key, rawVal)
}

// Unmarshal unmarshals the entire configuration
func (cl *ConfigLoader) Unmarshal(rawVal interface{}) error {
	return cl.viper.Unmarshal(rawVal)
}

// IsSet checks if a configuration key is set
func (cl *ConfigLoader) IsSet(key string) bool {
	return cl.viper.IsSet(key)
}

// SetDefault sets a default value for a configuration key
func (cl *ConfigLoader) SetDefault(key string, value interface{}) {
	cl.viper.SetDefault(key, value)
}

// LoadConfig is a convenience function to load configuration for an environment
func LoadConfig(configPath, environment string) (*ConfigLoader, error) {
	if environment == "" {
		environment = os.Getenv("ENVIRONMENT")
		if environment == "" {
			environment = "development"
		}
	}

	loader := NewConfigLoader(configPath)
	if err := loader.LoadEnvironment(environment); err != nil {
		return nil, err
	}

	return loader, nil
}

// ValidateConfig validates required configuration values
func ValidateConfig(loader *ConfigLoader, environment string) error {
	// Common required fields
	required := []string{
		"environment",
		"api.listen_address",
		"database.driver",
		"cache.distributed.type",
	}

	// Environment-specific required fields
	switch environment {
	case "production", "staging":
		required = append(required, 
			"auth.jwt.secret",
			"database.host",
			"cache.distributed.address",
		)
	}

	// Check required fields
	var missing []string
	for _, field := range required {
		if !loader.IsSet(field) || loader.GetString(field) == "" {
			missing = append(missing, field)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration fields: %v", missing)
	}

	// Validate specific values
	if loader.GetBool("auth.jwt.enabled") {
		secret := loader.GetString("auth.jwt.secret")
		if len(secret) < 32 {
			return fmt.Errorf("JWT secret must be at least 32 characters long")
		}
	}

	return nil
}