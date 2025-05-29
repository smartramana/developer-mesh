// Package aws provides adapters for backward compatibility during migration
package aws

import (
	"context"
	"fmt"
)

// LegacyGetAWSConfig is kept for backward compatibility with the old implementation
// It converts between the new AWS SDK v2 config and the legacy interface return type
func LegacyGetAWSConfig(ctx context.Context, cfg AuthConfig) (any, error) {
	// Call the new implementation which returns the concrete AWS config
	awsConfig, err := GetAWSConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get AWS config: %w", err)
	}

	// Convert to any type
	return &awsConfig, nil
}

// The new implementation in auth.go should be the canonical one to use going forward.
// Deprecation notice: Please update your code to use the new package structure
// and interfaces from pkg/common/aws/auth.go.

// This package contains several functions and types that were redeclared
// during the migration. The migration strategy is:
// 1. Keep the better implementation (typically in auth.go)
// 2. Redirect legacy calls through adapters when needed
// 3. Schedule removal of deprecated functions in the future
