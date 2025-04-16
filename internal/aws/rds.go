package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
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
	client *rds.Client
	config RDSConfig
}

// NewRDSClient creates a new RDS client
func NewRDSClient(ctx context.Context, cfg RDSConfig) (*RDSClient, error) {
	// Get AWS configuration
	awsCfg, err := GetAWSConfig(ctx, cfg.AuthConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create RDS client
	client := rds.NewFromConfig(awsCfg)

	return &RDSClient{
		client: client,
		config: cfg,
	}, nil
}

// GetAuthToken generates a temporary IAM auth token for RDS
func (c *RDSClient) GetAuthToken(ctx context.Context) (string, error) {
	input := &rds.GenerateAuthenticationTokenInput{
		Region:     aws.String(c.config.AuthConfig.Region),
		Hostname:   aws.String(c.config.Host),
		Port:       aws.Int32(int32(c.config.Port)),
		Username:   aws.String(c.config.Username),
	}

	if c.config.TokenExpiration > 0 {
		input.TokenExpirationInSeconds = aws.Int32(int32(c.config.TokenExpiration))
	}

	// In AWS SDK v2, GenerateAuthenticationToken is a direct function that returns a string
	token, err := c.client.GenerateAuthenticationToken(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to generate RDS auth token: %w", err)
	}

	return token, nil
}

// BuildPostgresConnectionString builds a PostgreSQL connection string with IAM auth if enabled
func (c *RDSClient) BuildPostgresConnectionString(ctx context.Context) (string, error) {
	// Base DSN with connection timeout and SSL mode
	dsn := fmt.Sprintf("host=%s port=%d dbname=%s connect_timeout=%d",
		c.config.Host, 
		c.config.Port, 
		c.config.Database,
		c.config.ConnectionTimeout)

	// Use IAM authentication if enabled and available
	if c.config.UseIAMAuth && IsIRSAEnabled() {
		token, err := c.GetAuthToken(ctx)
		if err != nil {
			return "", err
		}
		dsn = fmt.Sprintf("%s user=%s password=%s", dsn, c.config.Username, token)
	} else {
		// Fall back to standard username/password authentication
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
func (c *RDSClient) DescribeDBInstances(ctx context.Context, instanceIdentifier string) (*types.DBInstance, error) {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceIdentifier),
	}

	output, err := c.client.DescribeDBInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe DB instance: %w", err)
	}

	if len(output.DBInstances) == 0 {
		return nil, fmt.Errorf("no DB instance found with identifier: %s", instanceIdentifier)
	}

	return &output.DBInstances[0], nil
}
