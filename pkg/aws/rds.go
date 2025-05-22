// Package aws provides compatibility for the RDS-related functionality
// This file is primarily a bridge to pkg/common/aws
package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/rds"

	commonaws "github.com/S-Corkum/devops-mcp/pkg/common/aws"
)

// The following types and functions have been moved to aws.go as part of the centralized compatibility layer:
// - RDSConfig type (now aliased to commonaws.RDSConnectionConfig)
// - RDSClient type (now aliased to commonaws.ExtendedRDSClient)
// - NewRDSClient function (now a wrapper for commonaws.NewExtendedRDSClient)
//
// The RDSClient methods below are kept for backward compatibility:

// LegacyRDSClientAdapter adapts old code that expects the legacy RDS client
// This is a temporary adapter for backward compatibility.
type LegacyRDSClientAdapter struct {
	extendedClient *commonaws.ExtendedRDSClient
}

// CreateLegacyRDSClientAdapter creates a compatibility adapter
func CreateLegacyRDSClientAdapter(ctx context.Context, cfg RDSConnectionConfig) (*LegacyRDSClientAdapter, error) {
	client, err := NewRDSClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &LegacyRDSClientAdapter{extendedClient: client}, nil
}

// GetAuthToken generates a temporary IAM auth token for RDS
func (a *LegacyRDSClientAdapter) GetAuthToken(ctx context.Context) (string, error) {
	return a.extendedClient.GetAuthToken(ctx)
}

// BuildPostgresConnectionString builds a PostgreSQL connection string with IAM auth
func (a *LegacyRDSClientAdapter) BuildPostgresConnectionString(ctx context.Context) (string, error) {
	return a.extendedClient.BuildPostgresConnectionString(ctx)
}

// DescribeDBInstances describes RDS DB instances
func (a *LegacyRDSClientAdapter) DescribeDBInstances(ctx context.Context, instanceIdentifier string) (*rds.DescribeDBInstancesOutput, error) {
	return a.extendedClient.DescribeDBInstances(ctx, instanceIdentifier)
}

