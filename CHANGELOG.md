# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.4] - 2025-04-17

### Added
- Enhanced Harness adapter with Cloud Cost Management (CCM) integration
- Added CCM model structures for costs, recommendations, budgets, and anomalies
- Implemented methods to retrieve CCM cost data using GraphQL API
- Added functionality to get and manage cost optimization recommendations
- Implemented budget information retrieval from CCM
- Added cost anomaly detection and management capabilities
- Created CCM-specific actions: apply/ignore recommendations, acknowledge/ignore anomalies
- Enhanced configuration with configurable CCM API URLs

### Changed
- Updated Harness adapter configuration to support more configurable URL options
- Made base URLs for all API endpoints configurable through the config file
- Improved testConnection method to properly test connectivity to the Harness API
- Enhanced API endpoint implementations to use correct REST and GraphQL endpoints
- Updated IsSafeOperation method to include CCM-related actions

### Fixed
- N/A

## [0.2.3] - 2025-04-17

### Added
- Implemented proper IAM authentication for AWS RDS using SDK
- Implemented proper IAM authentication for AWS ElastiCache using SDK
- Added unit tests for IAM authentication and IRSA detection
- Enhanced configuration management with better defaults for security
- Added missing fields for AWS service authentication in local configuration

### Changed
- Made IAM authentication the default and preferred method for all AWS services
- Improved error handling and fallback mechanisms for authentication failures
- Enhanced configuration to better separate production from local development settings
- Updated environment variable handling to be more consistent
- Modified database connection handling to prioritize IAM authentication
- Improved IAM role detection for Kubernetes deployments

### Fixed
- Fixed placeholder implementations of IAM authentication in RDS client
- Fixed placeholder implementations of IAM authentication in ElastiCache client
- Removed remaining hardcoded credentials from configuration
- Fixed issue with missing proper configuration for database and cache connections
- Implemented proper authentication token handling for AWS services

## [0.2.2] - 2025-04-17

### Added
- No new features added

### Changed
- Improved configuration management for all AWS services (RDS, ElastiCache, S3)
- Enhanced IAM authentication to be the default method for AWS services
- Updated ElastiCache client to better handle IAM authentication tokens
- Updated RDS client to prioritize IAM authentication and improved fallback mechanism
- Updated S3 client to consistently use IAM authentication when available
- Replaced hardcoded credentials with environment variable references
- Improved environment variable interpolation in configuration files
- Better handling of AWS region configuration through environment variables
- Enhanced error handling for authentication failures
- Improved test compatibility with updated authentication methods

### Fixed
- Removed hardcoded default passwords from configuration files
- Fixed potential issue where authentication might fail silently
- Fixed ElastiCache test to work with enhanced IAM authentication
- Removed unused imports in AWS service implementations
- Fixed potential security issues by removing hardcoded webhook secrets

## [0.2.1] - 2025-04-16

### Added
- Fixed dependency issues with AWS SDK by updating required package versions
- Fixed compatibility issues in ElastiCache and RDS client implementations
- Introduced temporary stubs for AWS service clients during testing
- Improved error handling in AWS service client initialization
- AWS service integrations using IAM Roles for Service Accounts (IRSA)
- Validated IRSA implementation for S3, RDS, and ElastiCache in EKS deployments
- RDS Aurora PostgreSQL integration with IAM-based authentication
- Redis ElastiCache integration with IAM-based authentication and cluster mode support
- Enhanced S3 integration with IAM-based authentication and comprehensive error handling
- Kubernetes manifest files with IRSA annotations for EKS deployment
- Port configuration to support both local development (8080) and EKS deployment (443)
- Detailed documentation for AWS service integrations and IRSA setup
- IAM policy templates for each AWS service
- Comprehensive guide for local development with AWS service integrations
- Fallback authentication methods for local development
- Connection pooling best practices for all AWS services
- Advanced example demonstrating combined S3 storage and vector search functionality
- Document outlining innovative ways to leverage the MCP server functionality
- Production deployment security guide with emphasis on using port 443 instead of 8080

### Changed
- Updated configuration management to support AWS IAM authentication
- Modified S3 client to use IRSA when available
- Enhanced Redis integration with cluster mode support
- Restructured database connection handling to support IAM authentication
- Improved security in configuration files by using environment variables for sensitive data
- Verified and tested IRSA configuration to ensure proper authentication with AWS services in EKS
- Added IRSA detection logic to automatically use IAM authentication when available
- Updated README to include AWS integration instructions and security notes for production deployments
- Enhanced system architecture documentation to include AWS service integrations
- Streamlined authentication flow with automatic detection of available auth methods

### Fixed
- Fixed unused imports in AWS package (elasticache.go and rds.go)
- Resolved method duplication in database package (CreateContextReferenceTable)
- Fixed type mismatch between aws.S3Client and storage.S3Client in server implementation
- Updated math/rand usage for better compatibility across Go versions
- Fixed compilation issues in cmd/server/main.go
- Fixed security vulnerability (CVE-2023-39325) in golang.org/x/net by upgrading to v0.35.0
- Fixed security vulnerabilities in github.com/gin-gonic/gin (CVE-2023-26125, CVE-2020-28483)
- Fixed security vulnerability (CVE-2024-24786) in google.golang.org/protobuf by upgrading to v1.36.3

## [0.2.0] - 2025-04-15

### Added
- S3 storage implementation for context data
- Context reference storage in database for indexing and querying
- S3 Context Manager for efficiently storing large context windows
- LocalStack integration for local S3 development
- Comprehensive test suite for all S3-related components
- Security enhancements for configuration management
- Vector search functionality using PostgreSQL pg_vector extension
- API endpoints for storing and retrieving vector embeddings
- Efficient similarity search for context items
- Hybrid approach where MCP manages vector storage while agents control embedding generation
- Example code demonstrating agent integration with vector search
- Detailed documentation for S3 storage and vector search functionality

### Changed
- Extended S3 client with additional methods (DeleteFile, ListFiles)
- Updated engine to work with the new storage backend
- Modified configuration to support S3 as context storage
- Enhanced docker-compose.yml to include LocalStack container
- Improved security in configuration files by using environment variables for sensitive data
- Added S3 server-side encryption configuration options
- Enhanced Prometheus configuration with security options
- Updated README.md with latest features and improved example code
- Enhanced system architecture documentation to include new components

### Fixed
- N/A

## [0.1.0] - 2025-04-14

### Added
- Initial implementation of MCP server
- Basic adapters for DevOps tools (GitHub, Artifactory, etc.)
- Core engine for event processing
- In-memory context management
- PostgreSQL database integration
- Redis cache implementation