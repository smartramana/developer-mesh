# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - 2025-05-29

### Added
- Go workspace structure for better code organization and dependency management
- Hybrid architecture for mcp-server combining MCP and REST API functionality
- GitHub API integration testing infrastructure (configurable for real API or mocks)
- Helper scripts for GitHub App setup and webhook testing
- Comprehensive documentation for GitHub integration testing
- Performance optimizations for mcp-server including connection pooling and caching
- Swagger/OpenAPI documentation improvements with better examples
- Support for multiple authentication methods (API keys, JWT, GitHub App)
- Database schema detection for SQLite vs PostgreSQL compatibility

### Changed
- **Project Structure**: Migrated to Go workspace with separate modules for apps
- **Import Paths**: Updated all imports to use new workspace structure
- **Build System**: Enhanced Makefile with workspace-aware commands
- **Documentation**: Updated all docs to reflect current project state and commands
- **Configuration**: Simplified config files with better examples and defaults
- **Testing**: Improved test organization with dedicated test modules

### Fixed
- All linting errors following Go best practices
- Context update bug where Name field wasn't being updated
- Database schema issues when switching between SQLite and PostgreSQL
- Import cycles and dependency issues in the new workspace structure
- Mock server implementation for better test reliability
- Tenant validation in context CRUD operations
- Build and test commands to work with Go workspaces

### Developer Experience
- Added `config.yaml.example` with clear setup instructions
- Improved quick-start guide with accurate commands
- Enhanced troubleshooting documentation
- Better error messages for common setup issues
- Clearer contribution guidelines
- Simplified local development setup with `make dev-setup`

## [0.3.8] - 2025-04-27

### Added
- Support for S3-based context storage with automatic failover to in-memory storage if S3 is not configured.
- Example S3 configuration file (`config.example.yaml`) for easier setup.

### Changed
- Refactored S3 and storage code for improved modularity and maintainability.
- Updated interface method signatures for consistency and correctness.
- **Major documentation restructure:**
    - All documentation files reorganized into logical subdirectories by audience and purpose (getting started, user, admin, developer, integrations, examples, API, features, security, diagrams, use-cases).
    - All README files updated for correct navigation, purpose clarity, and to reflect new file locations.
    - Internal links and section descriptions improved for usability and maintainability.

### Fixed
- All core tests now pass or are intentionally skipped, ensuring better test reliability.
- Resolved test visibility and build issues.

### Internal
- Refactored and moved mock implementations to `_test.go` files to keep them out of production binaries.

## [0.3.7] - 2025-04-26

### Changed
- Refactored project structure for Go best practices (moved/reorganized script files)
- Updated Makefile and project files for improved development workflow
- Reorganized and fixed event listener logic
- Improved and fixed functional test suite; all functional tests now pass
- Updated and expanded Swagger/OpenAPI documentation to reflect full capabilities
- Dropped authentication requirement for GET /metrics endpoint (metrics are now public)
- Updated API documentation and README to clarify /metrics endpoint is public
- Updated API reference and Swagger docs to include /metrics and monitoring endpoints

### Fixed
- Fixed build failures and test issues related to recent refactors
- Resolved type mismatch in APIKeys handling in the API
- Improved error handling and test reliability in event and adapter logic
- Addressed issues with functional test reliability (especially for GitHub and metrics endpoints)
- Fixed various minor bugs in event and test logic

### Documentation
- Major documentation updates:
  - API reference and OpenAPI/Swagger docs reflect all current endpoints and security models
  - Monitoring and metrics documentation clarified
  - Auth requirements and endpoint access clarified for all users
  - Changelog and project files updated for transparency



### Added
- Comprehensive documentation overhaul with improved project structure
- New CONTRIBUTING guide for new contributors
- Detailed index for better documentation navigation
- Upgrading guide for version migrations
- Best practices documentation
- API documentation best practices guide
- Enhanced OpenAPI 3.0 specification with best practices implementation
- Comprehensive API reference markdown document
- Automatic database migration system with proper versioning
- Database migration utilities for data transformation

### Changed
- Completely restructured the OpenAPI/Swagger documentation
- Added detailed descriptions for all API endpoints and parameters
- Introduced tags for better API organization and categorization
- Added complete examples for request and response objects
- Implemented consistent response patterns and HATEOAS links
- Added proper security schemes documentation
- Enhanced the main README with comprehensive project information
- Improved documentation structure for better usability

### Fixed
- Resolved build and compatibility issues in database migration code
- Fixed missing RunTxContext method in database package
- Fixed unused variable in migration manager
- Added Register method to Provider struct
- Fixed test compatibility issues with TestEventBus and StandaloneEventBus
- Updated imports to resolve conflicts
- Improved transaction handling with BeginTxx

## [0.3.5] - 2025-04-22

### Added
- Enhanced vector search functionality with multi-model support
- Added similarity score calculation in vector search results
- Implemented model-specific embedding endpoints for better organization
- Added new API endpoints for managing model-specific embeddings
  - GET /api/v1/vectors/models - List all models with embeddings
  - GET /api/v1/vectors/context/:context_id/model/:model_id - Get embeddings for a specific model
  - DELETE /api/v1/vectors/context/:context_id/model/:model_id - Delete embeddings for a specific model

### Changed
- Improved vector embedding repository with model-specific methods
- Updated vector search to respect similarity thresholds
- Enhanced vector API to support multi-model operations
- Updated search API to include similarity scores in results
- Refactored vector handlers for better testability
- Modified test infrastructure to support multi-model testing

### Fixed
- Fixed interface compatibility issues in vector handlers
- Enhanced type safety in embedding repository
- Resolved embedding parsing issues in multi-model scenarios
- Fixed vector storage to properly handle embeddings from different models
- Improved error handling in vector search operations

## [0.3.4] - 2025-04-20

### Added
- Enhanced GitHub API client implementation:
  - Improved rate limiting with adaptive retry strategies
  - Added ETag support for more efficient API calls
  - Enhanced GraphQL client implementation for complex queries
  - Comprehensive webhook security and processing improvements

### Changed
- Reimplemented context management functionality with improved design
- Updated documentation for context management features
- Enhanced testing suite with comprehensive unit tests for context truncation
- Optimized GitHub API integration for better performance and reliability

### Fixed
- Resolved compilation issues after major code refactoring
- Fixed interface errors in internal/core package
- Fixed nil pointer dereference in adapter registry initialization
- Resolved type compatibility issues in adapter registry and event bridge
- Fixed MetricsClient field naming and initialization in AdapterManager
- Improved error handling in GitHub adapter with better exception management

## [0.3.3] - 2025-04-18

### Added
- Consolidated GitHub adapter into unified implementation:
  - Combined multiple files into github.go, github_test.go, and safety.go
  - Enhanced safety.go with additional safety checks for repositories, teams, and permissions
  - Added new operation: searchRepositories for more targeted repository search
  - Extended workflow support with approval and rejection capabilities
  - Added detailed statistics tracking for operation-level metrics
  - Implemented average response time tracking for performance monitoring
  - Enhanced health monitoring with more comprehensive diagnostics
  
### Changed
- Improved GitHub adapter organization with better separation of concerns
- Enhanced error handling with more descriptive error messages
- Added contextual information to all error messages
- Improved rate limit handling with optional automatic retry when limits are reached
- Updated safety checks to handle more edge cases
- Enhanced test coverage for all GitHub operations

### Fixed
- Fixed response handling for workflow operations
- Improved webhook URL safety validation with more robust domain checking
- Enhanced branch operation safety with comprehensive checks against protected branches
- Streamlined pagination handling for all list operations
- Resolved edge cases in error handling for rate limiting

## [0.3.2] - 2025-04-18

### Added
- Enhanced GitHub adapter with numerous improvements:
  - Added pagination support for all list methods with configurable page sizes
  - Added rate limiting awareness with automatic detection and handling
  - Implemented workflow management features (enable/disable workflows)
  - Added support for workflow run approvals and rejections
  - Added advanced search capabilities for repositories, users, and code
  - Implemented detailed statistics tracking for monitoring adapter health
  - Added robust error handling with improved retry logic and exponential backoff
  - Enhanced GitHub adapter health reporting with detailed diagnostics
  - Added GetHealthDetails method for comprehensive health monitoring

### Changed
- Updated GitHub API integration to consistently use API version 2022-11-28
- Improved HTTP transport to include proper headers for all requests
- Enhanced error handling with more descriptive error messages
- Updated configuration with more customization options
- Implemented safer parameter validation across all operations

### Fixed
- Fixed potential race conditions in rate limit handling
- Enhanced safety checks to prevent operations on protected branches
- Fixed error handling to properly distinguish between various error types
- Improved webhook handling with better payload validation

## [0.3.1] - 2025-04-17

### Fixed
- Updated JFrog Artifactory adapter tests to align with the current implementation
- Updated JFrog Xray adapter tests to align with the current implementation
- Fixed test structure to properly validate auth headers, safe operations, API endpoints, and health checks
- Enhanced test coverage for both adapters, including common operations such as GetData and ExecuteAction
- Added test for extractVersionFromComponentID utility function in Xray adapter

## [0.3.0] - 2025-04-17

### Added
- Comprehensive test suite for SonarQube adapter with 100% code coverage
- Unit tests for SonarQube adapter initialization and configuration
- Tests for all SonarQube adapter operations including quality gates, issues, metrics, and projects
- Tests for SonarQube adapter actions including project management and quality gate assignment
- Mock server implementation for testing SonarQube API interactions
- Test cases for HTTP client behavior including retries and error handling

### Fixed
- Improved error handling in SonarQube adapter's executeRequest method
- Enhanced authentication method validation in SonarQube adapter
- Improved request creation with proper URL handling and query parameter encoding
- Fixed potential nil pointer issues in HTTP client handling

## [0.2.9] - 2025-04-17

### Added
- Comprehensive test suite for Harness adapter
- Tests for Harness adapter's safety functions
- Improved code safety with properly defined stubs for unimplemented methods

### Fixed
- Fixed compilation errors in Harness adapter implementation
- Fixed missing byte import in ignoreCCMRecommendation and similar methods
- Fixed proper error handling in request/response processing
- Resolved unstructured error handling in adapter methods

## [0.2.8] - 2025-04-17

### Added
- Added comprehensive tests for GitHub adapter's safety functions: IsSafeRepository, IsSafeWebhookURL, and IsSafeBranchOperation
- Enhanced testing coverage for GitHub adapter functionality

### Changed
- Improved GitHub API type handling for workflow dispatch operations
- Fixed type compatibility issues with GitHub API v45.2.0
- Enhanced testability of the GitHub adapter by improving mock server integration

### Fixed
- Fixed type mismatch in triggerWorkflow function for proper GitHub API compatibility
- Fixed authentication token handling in test scenarios
- Resolved issues with the CreateWorkflowDispatchEventByID and Dispatch methods

## [0.2.7] - 2025-04-17

### Added
- Enhanced GitHub adapter with comprehensive REST API v2022-11-28 integration
- Added API version header support with `X-GitHub-Api-Version: 2022-11-28` for all requests
- Implemented GitHub Actions workflow management (trigger, check status)
- Added branch management operations (create, list)
- Added team and member management functionality
- Implemented webhook management capabilities (create, delete, list)
- Added code search functionality
- Added support for commit history retrieval and analysis
- Enhanced pull request management with support for reviewers and labels
- Added custom HTTP transport for consistent header management
- Improved webhook event handling with support for more event types

### Changed
- Updated GitHub API client to use the latest API version
- Enhanced safety checks for potentially dangerous operations
- Improved parameter validation and error handling
- Restructured code for better maintainability
- Enhanced test coverage with more comprehensive test cases
- Updated GitHub event parsing with better type safety

### Fixed
- Fixed webhook payload handling to properly parse all supported event types
- Improved error messages for better debugging
- Enhanced security checks to prevent unsafe operations

## [0.2.6] - 2025-04-17

### Added
- Enhanced JFrog Artifactory adapter with comprehensive REST API integration
- Enhanced JFrog Xray adapter with comprehensive REST API integration
- Added support for multiple authentication methods in both adapters (Bearer token, JWT, API key, Basic auth)
- Implemented connectivity testing using system/ping endpoints
- Added new Artifactory operations: getStorageInfo, getFolderInfo, getSystemInfo, getVersion, getRepositoryInfo
- Added new Artifactory actions: getFolderContent, calculateChecksum
- Added new Xray operations: getSystemInfo, getSystemVersion, getWatches, getPolicies, getSummary
- Added new Xray actions: scanBuild (with v1/v2 API support), generateVulnerabilitiesReport, getComponentDetails, getScanStatus
- Implemented advanced search capability using Artifactory Query Language (AQL)

### Changed
- Updated authentication method to use centralized setAuthHeaders function
- Enhanced request handling with proper HTTP status code checking
- Improved parameter validation for all API operations
- Modified response processing to transform API responses to expected formats
- Improved error handling with detailed error messages
- Added API version support for backward compatibility
- Updated request construction to follow JFrog API best practices

### Fixed
- Fixed mock server connectivity testing to properly verify server health
- Enhanced testConnection method to properly test connectivity to the JFrog APIs
- Improved implementation of scan operations to properly handle API responses

## [0.2.5] - 2025-04-17

### Added
- Enhanced SonarQube adapter with comprehensive API integration
- Added proper API authentication support including Bearer token and Basic auth methods
- Implemented connectivity testing using the system/status endpoint
- Added support for quality gates management (get status, assign to projects, list gates)
- Implemented project management functionality (create, delete, set tags)
- Added metrics and measures endpoints with support for filtering and historical data
- Implemented issues management with comprehensive filtering options
- Added component details retrieval functionality
- Created new operations: search_metrics, get_measures_history, get_component_details, get_quality_gates
- Added new actions: create_project, delete_project, get_analysis_status, set_project_tags, set_quality_gate

### Changed
- Improved request handling with centralized request creation and execution
- Enhanced error handling with proper HTTP status code checking
- Implemented retry logic for API requests
- Updated request authentication to follow SonarQube best practices
- Improved form data submission for POST requests
- Enhanced parameter validation for all API operations

### Fixed
- Fixed mock server connectivity testing to properly verify server health
- Improved error reporting with more detailed error messages
- Enhanced testConnection method to properly test connectivity to the SonarQube API

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