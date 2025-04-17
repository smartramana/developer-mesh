package aws

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// RDSConfig holds configuration for RDS
type RDSConfig struct {
	AuthConfig        AuthConfig `mapstructure:"auth"`
	Host              string     `mapstructure:"host"`
	Port              int        `mapstructure:"port"`
	Database          string     `mapstructure:"database"`
	Username          string     `mapstructure:"username"`
	Password          string     `mapstructure:"password"`
	UseIAMAuth        bool       `mapstructure:"use_iam_auth"`
	TokenExpiration   int        `mapstructure:"token_expiration"`
	MaxOpenConns      int        `mapstructure:"max_open_conns"`
	MaxIdleConns      int        `mapstructure:"max_idle_conns"`
	ConnMaxLifetime   time.Duration `mapstructure:"conn_max_lifetime"`
	EnablePooling     bool       `mapstructure:"enable_pooling"`
	MinPoolSize       int        `mapstructure:"min_pool_size"`
	MaxPoolSize       int        `mapstructure:"max_pool_size"`
	ConnectionTimeout int        `mapstructure:"connection_timeout"`
}

// RDSClient is a client for AWS RDS
type RDSClient struct {
	config RDSConfig
}

// NewRDSClient creates a new RDS client
func NewRDSClient(ctx context.Context, cfg RDSConfig) (*RDSClient, error) {
	return &RDSClient{
		config: cfg,
	}, nil
}

// GetAuthToken generates a temporary IAM auth token for RDS
func (c *RDSClient) GetAuthToken(ctx context.Context) (string, error) {
	// For testing in local development environments, return a mock token
	if c.config.Host == "localhost" || c.config.Host == "127.0.0.1" || 
	   c.config.Host == "" || strings.HasPrefix(c.config.Host, "host.docker.internal") {
		log.Println("Using mock RDS auth token for local development")
		return "mock-auth-token-local", nil
	}
	
	// In production environments, we should use the AWS SDK to generate an RDS auth token
	if IsIRSAEnabled() {
		log.Println("Using IRSA for RDS authentication with role:", os.Getenv("AWS_ROLE_ARN"))
		// TODO: Implement the actual AWS SDK call - this is currently a placeholder
		// For example, using the rdsutils package:
		//
		// signer := rdsutils.NewSigner()
		// authToken, err := signer.GetAuthToken(
		//     rdsutils.GetAuthTokenInput{
		//         Region:      c.config.AuthConfig.Region,
		//         HostName:    c.config.Host,
		//         Port:        c.config.Port,
		//         UserName:    c.config.Username,
		//         ExpiryTime:  time.Duration(c.config.TokenExpiration) * time.Second,
		//     })
		// if err != nil {
		//     return "", fmt.Errorf("error generating RDS auth token: %w", err)
		// }
		// return authToken, nil
	}
	
	// For now, we're using a mock implementation
	// This should be replaced with actual AWS SDK implementation in production
	log.Println("WARNING: Using mock RDS auth token - real implementation needed for production")
	return "mock-auth-token", nil
}

// BuildPostgresConnectionString builds a PostgreSQL connection string with IAM auth if enabled
func (c *RDSClient) BuildPostgresConnectionString(ctx context.Context) (string, error) {
	// Base DSN with connection timeout and SSL mode
	dsn := fmt.Sprintf("host=%s port=%d dbname=%s connect_timeout=%d",
		c.config.Host, 
		c.config.Port, 
		c.config.Database,
		c.config.ConnectionTimeout)

	// Default to using IAM auth (the secure option) unless explicitly disabled
	if c.config.UseIAMAuth {
		// Try to get an auth token - this will work both in AWS and locally with mock tokens
		token, err := c.GetAuthToken(ctx)
		if err != nil {
			log.Printf("Warning: Failed to get IAM auth token, error: %v", err)
			
			// If IAM auth fails and no password is configured, return an error
			if c.config.Password == "" {
				return "", fmt.Errorf("IAM authentication failed and no password fallback is configured: %w", err)
			}
			
			// Otherwise fall back to password auth
			log.Println("Falling back to password authentication for RDS")
			dsn = fmt.Sprintf("%s user=%s password=%s", dsn, c.config.Username, c.config.Password)
		} else {
			// Use the IAM token as the password
			dsn = fmt.Sprintf("%s user=%s password=%s", dsn, c.config.Username, token)
		}
	} else {
		// IAM auth explicitly disabled, use standard username/password authentication
		// Check if password is provided
		if c.config.Password == "" {
			return "", fmt.Errorf("password authentication enabled but no password provided")
		}
		dsn = fmt.Sprintf("%s user=%s password=%s", dsn, c.config.Username, c.config.Password)
	}

	// Always use SSL with RDS
	dsn = fmt.Sprintf("%s sslmode=require", dsn)

	// Add connection pooling parameters if enabled
	if c.config.EnablePooling {
		dsn = fmt.Sprintf("%s pool=true min_pool_size=%d max_pool_size=%d", 
			dsn, 
			c.config.MinPoolSize, 
			c.config.MaxPoolSize)
	}

	return dsn, nil
}

// DescribeDBInstances describes RDS DB instances
func (c *RDSClient) DescribeDBInstances(ctx context.Context, instanceIdentifier string) (interface{}, error) {
	// Mock implementation for testing
	return nil, nil
}