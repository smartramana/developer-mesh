// Package aws provides a compatibility layer for the pkg/common/aws package.
// This package is part of the Go Workspace migration to ensure backward compatibility
// with code still importing the old pkg/aws package path.
package aws

// No imports needed for type aliases
// The actual implementations will be loaded from the common/aws package
// through the replace directive in go.mod

// Type definitions for backward compatibility
// These match the struct definitions in pkg/common/aws

// AWSClient provides a standard interface for AWS clients
type AWSClient interface {
	GetSession() interface{}
	GetCredentials() interface{}
	GetRegion() string
	CreateS3Client() interface{}
	CreateSQSClient() interface{}
}

// AuthConfig wraps AWS authentication configuration
type AuthConfig struct {
	Region     string
	Endpoint   string
	AssumeRole string
}

// LegacyAuthConfig wraps AWS authentication configuration (legacy version)
type LegacyAuthConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Profile         string
	Endpoint        string
}

// RDSConfig holds configuration for RDS
type RDSConfig struct {
	Region     string
	SecretName string
}

// RDSClient is a client for AWS RDS
type RDSClient struct {
	Config interface{}
}

// S3Config holds configuration for S3
type S3Config struct {
	Auth              AuthConfig
	Bucket            string
	UploadPartSize    int64
	DownloadPartSize  int64
	Concurrency       int
	RequestTimeout    int
	ServerSideEncrypt string
}

// ElastiCacheConfig holds configuration for ElastiCache
type ElastiCacheConfig struct {
	Auth               AuthConfig
	ClusterAddress     string
	Port               int
	ClusterMode        bool
	ClusterDiscovery   bool
	UseTLS             bool
	InsecureSkipVerify bool
	MaxRetries         int
	MinIdleConnections int
	PoolSize           int
	DialTimeout        int
	ReadTimeout        int
	WriteTimeout       int
	PoolTimeout        int
}

// StandardAWSClient implements the AWSClient interface
type StandardAWSClient struct {
	// Contains unexported fields
}

// Function declarations for backward compatibility
// These are stubs that will be implemented by the real implementations in pkg/common/aws

// NewAWSClient creates a new AWS client with the provided config
func NewAWSClient(ctx interface{}, cfg interface{}) AWSClient {
	// This is a stub that will be overridden by the real implementation
	return nil
}

// GetAWSConfig gets an AWS config
func GetAWSConfig(ctx interface{}, cfg AuthConfig) (interface{}, error) {
	// This is a stub that will be overridden by the real implementation
	return nil, nil
}

// LegacyGetAWSConfig gets an AWS config using legacy format
func LegacyGetAWSConfig(ctx interface{}, cfg AuthConfig) (interface{}, error) {
	// This is a stub that will be overridden by the real implementation
	return nil, nil
}

// IsIRSAEnabled returns whether IRSA is enabled
func IsIRSAEnabled() bool {
	// This is a stub that will be overridden by the real implementation
	return false
}

// GetAWSConfigLegacy gets an AWS config using legacy format
func GetAWSConfigLegacy(ctx interface{}, cfg LegacyAuthConfig) (interface{}, error) {
	// This is a stub that will be overridden by the real implementation
	return nil, nil
}

// NewRDSClient creates a new RDS client
func NewRDSClient(ctx interface{}, cfg RDSConfig) (*RDSClient, error) {
	// This is a stub that will be overridden by the real implementation
	return nil, nil
}

// IsIRSAEnabledLegacy returns whether IRSA is enabled (legacy version)
func IsIRSAEnabledLegacy() bool {
	// This is a stub that will be overridden by the real implementation
	return false
}
