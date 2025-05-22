// Package aws provides a compatibility layer for code that imports
// github.com/S-Corkum/devops-mcp/pkg/aws. This package re-exports all
// types and functions from github.com/S-Corkum/devops-mcp/pkg/common/aws.
//
// Deprecated: This package will be removed in a future version.
// Import github.com/S-Corkum/devops-mcp/pkg/common/aws directly instead.
// See the migration guide at docs/migration_guide.md for more information.
package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	
	commonaws "github.com/S-Corkum/devops-mcp/pkg/common/aws"
)

// Type aliases for compatibility
type (
	// AuthConfig wraps AWS authentication configuration
	AuthConfig = commonaws.AuthConfig
	
	// RDSConnectionConfig holds configuration for RDS
	RDSConnectionConfig = commonaws.RDSConnectionConfig
	
	// RDSConfig is an alias for RDSConnectionConfig for backward compatibility
	RDSConfig = commonaws.RDSConnectionConfig
	
	// RDSClient is a client for AWS RDS
	RDSClient = commonaws.ExtendedRDSClient
	
	// RDSClientInterface defines the interface for RDS operations
	RDSClientInterface = commonaws.RDSClientInterface
)

// IsIRSAEnabled returns true if IAM Roles for Service Accounts is enabled
func IsIRSAEnabled() bool {
	return commonaws.IsIRSAEnabled()
}

// GetAWSConfig gets AWS configuration
func GetAWSConfig(ctx context.Context, cfg AuthConfig) (aws.Config, error) {
	return commonaws.GetAWSConfig(ctx, cfg)
}

// GetSession returns an AWS session (stub for backward compatibility)
func GetSession(config AuthConfig) (interface{}, error) {
	// This is a stub implementation for backward compatibility
	// Modern code should use GetAWSConfig instead
	_, err := commonaws.GetAWSConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// NewRDSClient creates a new RDS client
func NewRDSClient(ctx context.Context, cfg RDSConnectionConfig) (*RDSClient, error) {
	return commonaws.NewExtendedRDSClient(ctx, cfg)
}

// Function to get AWS region from AuthConfig
func GetRegion(config AuthConfig) string {
	return commonaws.GetRegion(config)
}

// Function to create AWS config with standard options
func CreateConfig(region string) aws.Config {
	return commonaws.CreateConfig(region)
}

// Function to extract error code from AWS error
func GetAWSErrorCode(err error) string {
	return commonaws.GetAWSErrorCode(err)
}
