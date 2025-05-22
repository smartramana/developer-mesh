// Package aws provides compatibility for the auth-related functionality
// This file is primarily a bridge to pkg/common/aws
package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"

	commonaws "github.com/S-Corkum/devops-mcp/pkg/common/aws"
)

// AssumeRoleProvider uses STS to assume the specified IAM role and returns a credentials provider
// This is kept for backward compatibility
func AssumeRoleProvider(ctx context.Context, awsCfg aws.Config, roleArn string) aws.CredentialsProvider {
	return commonaws.AssumeRoleProvider(ctx, awsCfg, roleArn)
}

// The following functions have been moved to aws.go as part of the centralized compatibility layer:
// - IsIRSAEnabled() bool
// - GetAWSConfig(ctx context.Context, cfg AuthConfig) (aws.Config, error)
// - AuthConfig type
//
// If you need to use these functions, import this package and use them directly.

