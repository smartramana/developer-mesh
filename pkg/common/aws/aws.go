package aws

import (
	"fmt"
	
	"github.com/aws/aws-sdk-go-v2/aws"
)

// GetRegion gets the current AWS region from the AuthConfig
func GetRegion(config AuthConfig) string {
	return config.Region
}

// CreateConfig creates an AWS config with standard options
func CreateConfig(region string) aws.Config {
	// This is just a stub for now
	// In a real implementation, this would create an AWS SDK config
	return aws.Config{}
}

// GetAWSErrorCode extracts the error code from an AWS error
func GetAWSErrorCode(err error) string {
	// Just a stub - would parse AWS error code
	return fmt.Sprintf("AWS-Error-%v", err)
}
