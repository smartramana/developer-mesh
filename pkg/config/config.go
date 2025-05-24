// Package config provides a compatibility layer for the pkg/common/config package.
// This package is part of the Go Workspace migration to ensure backward compatibility
// with code still importing the old pkg/config package path.
package config

// Type definitions for backward compatibility
// These match the struct definitions in pkg/common/config

// Config holds the complete application configuration
type Config struct {
	API        APIConfig              `mapstructure:"api"`
	Cache      interface{}            `mapstructure:"cache"`
	Database   DatabaseConfig         `mapstructure:"database"`
	Engine     CoreConfig             `mapstructure:"engine"`
	Metrics    interface{}            `mapstructure:"metrics"`
	AWS        interface{}            `mapstructure:"aws"`
	Environment string                `mapstructure:"environment"`
	Adapters   map[string]interface{} `mapstructure:"adapters"`
}

// APIConfig defines the API server configuration
type APIConfig struct {
	ListenAddress string                 `mapstructure:"listen_address"`
	BaseURL       string                 `mapstructure:"base_url"`
	TLSCertFile   string                 `mapstructure:"tls_cert_file"`
	TLSKeyFile    string                 `mapstructure:"tls_key_file"`
	CORSAllowed   string                 `mapstructure:"cors_allowed"`
	RateLimit     int                    `mapstructure:"rate_limit"`
	RequestTimeout int                   `mapstructure:"request_timeout"`
	ReadTimeout   interface{}            `mapstructure:"read_timeout"`
	WriteTimeout  interface{}            `mapstructure:"write_timeout"`
	IdleTimeout   interface{}            `mapstructure:"idle_timeout"`
	EnableCORS    bool                   `mapstructure:"enable_cors"`
	EnableSwagger bool                   `mapstructure:"enable_swagger"`
	Auth          map[string]interface{} `mapstructure:"auth"`
	Webhook       map[string]interface{} `mapstructure:"webhook"`
}

// CoreConfig defines the engine core configuration
type CoreConfig struct {
	EventBufferSize  int         `mapstructure:"event_buffer_size"`
	ConcurrencyLimit int         `mapstructure:"concurrency_limit"`
	EventTimeout     interface{} `mapstructure:"event_timeout"`
}

// DatabaseConfig holds configuration for the database
type DatabaseConfig struct {
	Driver        string      `mapstructure:"driver"`
	DSN           string      `mapstructure:"dsn"`
	Host          string      `mapstructure:"host"`
	Port          int         `mapstructure:"port"`
	Database      string      `mapstructure:"database"`
	Username      string      `mapstructure:"username"`
	Password      string      `mapstructure:"password"`
	MaxOpenConns  int         `mapstructure:"max_open_conns"`
	MaxIdleConns  int         `mapstructure:"max_idle_conns"`
	UseIAM        bool        `mapstructure:"use_iam"`
	SSLMode       string      `mapstructure:"ssl_mode"`
	SSLRootCert   string      `mapstructure:"ssl_root_cert"`
	SSLCert       string      `mapstructure:"ssl_cert"`
	SSLKey        string      `mapstructure:"ssl_key"`
	SearchPath    string      `mapstructure:"search_path"`
	MigrationsDir string      `mapstructure:"migrations_dir"`
	Vector        interface{} `mapstructure:"vector"`
}

// Function stubs for backward compatibility

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	// This is a stub that will be overridden by the real implementation
	return nil, nil
}

// IsProduction returns true if the environment is production
func (c *Config) IsProduction() bool {
	return c.Environment == "prod" || c.Environment == "production"
}

// IsDevelopment returns true if the environment is development
func (c *Config) IsDevelopment() bool {
	return c.Environment == "dev" || c.Environment == "development"
}

// IsStaging returns true if the environment is staging
func (c *Config) IsStaging() bool {
	return c.Environment == "staging" || c.Environment == "stage"
}

// GetListenPort returns the port number the API should listen on
func (c *Config) GetListenPort() int {
	// Default port
	return 8080
}
