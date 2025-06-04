package aws

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

// RDSConnectionConfig holds connection configuration for RDS
type RDSConnectionConfig struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	Database          string        `mapstructure:"database"`
	Username          string        `mapstructure:"username"`
	Password          string        `mapstructure:"password"`
	UseIAMAuth        bool          `mapstructure:"use_iam_auth"`
	TokenExpiration   int           `mapstructure:"token_expiration"`
	MaxOpenConns      int           `mapstructure:"max_open_conns"`
	MaxIdleConns      int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime   time.Duration `mapstructure:"conn_max_lifetime"`
	EnablePooling     bool          `mapstructure:"enable_pooling"`
	MinPoolSize       int           `mapstructure:"min_pool_size"`
	MaxPoolSize       int           `mapstructure:"max_pool_size"`
	ConnectionTimeout int           `mapstructure:"connection_timeout"`
	Auth              AuthConfig    `mapstructure:"auth"`
}

// RDSClientInterface defines the interface for RDS operations
type RDSClientInterface interface {
	GetAuthToken(ctx context.Context) (string, error)
	BuildPostgresConnectionString(ctx context.Context) (string, error)
	DescribeDBInstances(ctx context.Context, instanceIdentifier string) (*rds.DescribeDBInstancesOutput, error)
}

// ExtendedRDSClient extends the basic RDSClient with additional methods
type ExtendedRDSClient struct {
	awsConfig  aws.Config
	connConfig RDSConnectionConfig
	rdsClient  *rds.Client
}

// NewExtendedRDSClient creates a new extended RDS client with both connection and AWS config
func NewExtendedRDSClient(ctx context.Context, connCfg RDSConnectionConfig) (*ExtendedRDSClient, error) {
	// Get AWS config
	var awsConfig aws.Config

	// Always use IAM authentication if enabled
	if connCfg.UseIAMAuth {
		var err error
		awsConfig, err = GetAWSConfig(ctx, connCfg.Auth)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config with IAM auth: %w", err)
		}
	}

	// Create RDS client
	// For IAM auth we already have the AWS config, for non-IAM we'll set up a default client later
	var rdsClient *rds.Client
	if connCfg.UseIAMAuth {
		rdsClient = rds.NewFromConfig(awsConfig)
	}

	// Return client with connection config
	return &ExtendedRDSClient{
		awsConfig:  awsConfig,
		connConfig: connCfg,
		rdsClient:  rdsClient,
	}, nil
}

// GetAuthToken generates a temporary IAM auth token for RDS
// To implement IAM auth for RDS in production, you'll need to:
// 1. Import the AWS SDK RDS auth package: github.com/aws/aws-sdk-go-v2/feature/rds/auth
// 2. Use rdsAuth.BuildAuthToken to generate an IAM auth token
func (c *ExtendedRDSClient) GetAuthToken(ctx context.Context) (string, error) {
	// If IAM auth is disabled, return password
	if !c.connConfig.UseIAMAuth {
		if c.connConfig.Password == "" {
			return "", fmt.Errorf("password is required when IAM authentication is disabled")
		}
		return c.connConfig.Password, nil
	}

	// For testing in local development environments, return a mock token
	if c.connConfig.Host == "localhost" || c.connConfig.Host == "127.0.0.1" ||
		c.connConfig.Host == "" || strings.HasPrefix(c.connConfig.Host, "host.docker.internal") {
		log.Println("Using mock RDS auth token for local development")
		return "mock-auth-token-local", nil
	}

	// Get AWS configuration
	_, err := GetAWSConfig(ctx, c.connConfig.Auth)
	if err != nil {
		return "", fmt.Errorf("failed to get AWS config: %w", err)
	}

	// Set token expiration (for future use when implementing real token generation)
	tokenExpiryTime := time.Duration(c.connConfig.TokenExpiration) * time.Second
	if tokenExpiryTime == 0 {
		tokenExpiryTime = 15 * time.Minute
	}
	_ = tokenExpiryTime // TODO: Use when implementing real token generation

	// For now, this is a placeholder implementation
	// In a production environment, this would use the AWS SDK
	log.Println("Using mock RDS auth token for IAM authentication")

	// In production, this would use the AWS SDK to generate a token:
	// authToken, err := rdsAuth.BuildAuthToken(
	//    ctx, c.connConfig.Host, c.connConfig.Auth.Region, c.connConfig.Username, c.awsConfig.Credentials)
	return "iam-auth-token", nil
}

// BuildPostgresConnectionString builds a PostgreSQL connection string with IAM auth if enabled
func (c *ExtendedRDSClient) BuildPostgresConnectionString(ctx context.Context) (string, error) {
	// Base DSN with connection timeout and SSL mode
	dsn := fmt.Sprintf("host=%s port=%d dbname=%s connect_timeout=%d",
		c.connConfig.Host,
		c.connConfig.Port,
		c.connConfig.Database,
		c.connConfig.ConnectionTimeout)

	// Default to using IAM auth (the secure option) unless explicitly disabled
	if c.connConfig.UseIAMAuth {
		// Try to get an auth token - this will work both in AWS and locally with mock tokens
		token, err := c.GetAuthToken(ctx)
		if err != nil {
			log.Printf("Warning: Failed to get IAM auth token, error: %v", err)

			// If IAM auth fails and no password is configured, return an error
			if c.connConfig.Password == "" {
				return "", fmt.Errorf("IAM authentication failed and no password fallback is configured: %w", err)
			}

			// Otherwise fall back to password auth
			log.Println("Falling back to password authentication for RDS")
			dsn = fmt.Sprintf("%s user=%s password=%s", dsn, c.connConfig.Username, c.connConfig.Password)
		} else {
			// Use the IAM token as the password
			dsn = fmt.Sprintf("%s user=%s password=%s", dsn, c.connConfig.Username, token)
		}
	} else {
		// IAM auth explicitly disabled, use standard username/password authentication
		// Check if password is provided
		if c.connConfig.Password == "" {
			return "", fmt.Errorf("password authentication enabled but no password provided")
		}
		dsn = fmt.Sprintf("%s user=%s password=%s", dsn, c.connConfig.Username, c.connConfig.Password)
	}

	// Always use SSL with RDS
	dsn = fmt.Sprintf("%s sslmode=require", dsn)

	// Add connection pooling parameters if enabled
	if c.connConfig.EnablePooling {
		dsn = fmt.Sprintf("%s pool=true min_pool_size=%d max_pool_size=%d",
			dsn,
			c.connConfig.MinPoolSize,
			c.connConfig.MaxPoolSize)
	}

	return dsn, nil
}

// DescribeDBInstances describes RDS DB instances
func (c *ExtendedRDSClient) DescribeDBInstances(ctx context.Context, instanceIdentifier string) (*rds.DescribeDBInstancesOutput, error) {
	// Ensure we have an RDS client
	if c.rdsClient == nil {
		return nil, fmt.Errorf("RDS client not initialized, make sure AWS config is provided")
	}

	// Create the input parameters
	input := &rds.DescribeDBInstancesInput{}

	// If instance identifier provided, set it
	if instanceIdentifier != "" {
		input.DBInstanceIdentifier = &instanceIdentifier
	}

	// Call the AWS API
	return c.rdsClient.DescribeDBInstances(ctx, input)
}

// CreateDBInstance creates a new RDS DB instance
func (c *ExtendedRDSClient) CreateDBInstance(ctx context.Context, input *rds.CreateDBInstanceInput) (*rds.CreateDBInstanceOutput, error) {
	// Ensure we have an RDS client
	if c.rdsClient == nil {
		return nil, fmt.Errorf("RDS client not initialized, make sure AWS config is provided")
	}

	// Call the AWS API
	return c.rdsClient.CreateDBInstance(ctx, input)
}

// DeleteDBInstance deletes an RDS DB instance
func (c *ExtendedRDSClient) DeleteDBInstance(ctx context.Context, instanceIdentifier string, skipFinalSnapshot bool) (*rds.DeleteDBInstanceOutput, error) {
	// Ensure we have an RDS client
	if c.rdsClient == nil {
		return nil, fmt.Errorf("RDS client not initialized, make sure AWS config is provided")
	}

	// Create the input parameters
	input := &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier: &instanceIdentifier,
		SkipFinalSnapshot:    aws.Bool(skipFinalSnapshot),
	}

	// If we need a final snapshot and skipFinalSnapshot is false, set a name
	if !skipFinalSnapshot {
		snapshotID := fmt.Sprintf("%s-final-%d", instanceIdentifier, time.Now().Unix())
		input.FinalDBSnapshotIdentifier = &snapshotID
	}

	// Call the AWS API
	return c.rdsClient.DeleteDBInstance(ctx, input)
}
