# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Advanced example demonstrating combined S3 storage and vector search functionality
- Document outlining innovative ways to leverage the MCP server functionality
- Production deployment security guide with emphasis on using port 443 instead of 8080

### Changed
- Updated README to include security note for production deployments

### Fixed
- N/A

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
