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
	"os"

	commonaws "github.com/S-Corkum/devops-mcp/pkg/common/aws"
)

// IsIRSAEnabled returns true if IAM Roles for Service Accounts is enabled
func IsIRSAEnabled() bool {
	// Check if both required env vars for IRSA exist
	return os.Getenv("AWS_ROLE_ARN") != "" && os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") != ""
}

// GetSession returns an AWS session
func GetSession(config AuthConfig) (interface{}, error) {
	// This is a stub implementation until the common package has this function
	return nil, nil
}

// Re-export types from common/aws
type (
	// AuthConfig wraps AWS authentication configuration
	AuthConfig = commonaws.AuthConfig
	
	// RDSConfig holds configuration for RDS
	RDSConfig = commonaws.RDSConnectionConfig
	
	// RDSClient is a client for AWS RDS
	RDSClient = commonaws.ExtendedRDSClient
)

// NewRDSClient creates a new RDS client
func NewRDSClient(ctx context.Context, cfg RDSConfig) (*RDSClient, error) {
	return commonaws.NewExtendedRDSClient(ctx, cfg)
}

// GetAWSConfig gets AWS configuration
func GetAWSConfig(ctx context.Context, cfg AuthConfig) (interface{}, error) {
	return commonaws.GetAWSConfig(ctx, cfg)
}
