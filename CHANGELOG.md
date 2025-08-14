# Changelog

All notable changes to Developer Mesh will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.1] - 2025-01-14

### Added

#### Core Platform
- **MCP Protocol Implementation**: Full Model Context Protocol (MCP) 2025-06-18 support over WebSocket with JSON-RPC 2.0
  - Standard MCP methods: initialize, tools/list, tools/call, resources/list, resources/read
  - DevMesh tools exposed as MCP tools (devmesh.* namespace)
  - Resource subscriptions with devmesh:// URI scheme
  - Connection mode detection for Claude Code, IDE, and Agent clients
  
- **Three-Tier Agent Architecture**:
  - Agent Manifests for defining agent types and capabilities
  - Agent Configurations for tenant-specific settings
  - Agent Registrations for tracking running instances
  - Support for multiple instances of the same agent type
  - Workload management and assignment strategies

- **Dynamic Tools API**:
  - Zero-code tool integration via OpenAPI discovery
  - Automatic discovery of tool capabilities from OpenAPI specs
  - Universal authentication support (OAuth2, API key, bearer token, basic auth)
  - Per-tool health monitoring with configurable intervals
  - Tool execution with caching and circuit breakers
  - User token passthrough for personalized authentication
  - Learning system that improves discovery over time

- **Multi-Tenant Embedding System**:
  - Global embedding model catalog with 15+ models
  - Per-tenant model configuration and access control
  - Support for OpenAI, AWS Bedrock, Google, and Anthropic providers
  - Cost tracking and usage quotas (monthly/daily limits)
  - Agent-level model preferences
  - Automatic quota enforcement and failover

#### Authentication & Security
- **Organization Registration System**:
  - Complete organization signup flow
  - User registration with email verification
  - Password management with reset tokens
  - JWT authentication with refresh tokens
  - Session management and tracking
  - User invitation system with role-based access
  - Auth audit logging for security events

- **Security Features**:
  - Per-tenant credential encryption (AES-256-GCM)
  - API key validation with regex patterns
  - SQL injection prevention via parameterized queries
  - Rate limiting per tenant and API endpoint
  - Webhook signature validation

#### Event Processing
- **Redis Streams Integration**:
  - Webhook event processing with consumer groups
  - Dead letter queue (DLQ) for failed messages
  - Idempotency support with deduplication
  - At-least-once delivery guarantee
  - Automatic retry with exponential backoff

- **Webhook Processing**:
  - Dynamic webhook handler for all tool types
  - GitHub webhook support
  - Generic webhook ingestion
  - Event persistence and replay capability

#### Infrastructure
- **Database Schema**:
  - PostgreSQL with pgvector extension for semantic search
  - MCP schema namespace for all tables
  - 27 migration scripts for complete database setup
  - Support for vector embeddings and similarity search

- **Service Architecture**:
  - MCP Server (WebSocket, port 8080)
  - REST API Server (HTTP, port 8081)
  - Worker Service (Redis Streams consumer)
  - Go workspace support with shared packages

- **Observability**:
  - Structured logging with context
  - Prometheus metrics endpoints
  - Health check endpoints for all services
  - Request tracing and correlation IDs

#### Developer Experience
- **Local Development**:
  - Docker Compose setup for all services
  - Make targets for common tasks
  - Environment-based configuration system
  - Development auth keys for testing

- **API Documentation**:
  - Swagger/OpenAPI documentation
  - MCP protocol reference implementation
  - REST API endpoints documentation

### Configuration
- Hierarchical configuration system (base + environment overrides)
- Environment variable support for all settings
- Separate configurations for development, staging, and production
- Redis configuration with Sentinel and Cluster support
- AWS service configuration (S3, Bedrock)

### Testing
- Unit tests for core components
- Integration tests for API endpoints
- MCP protocol test scripts
- Mock implementations for development

### Known Limitations
- Email service not yet implemented (placeholders in code)
- Some monitoring components are optional
- Embedding API conditionally registered based on provider availability

## Notes

This is the initial release of Developer Mesh, providing a production-ready platform for orchestrating AI agents in DevOps workflows. The platform implements the industry-standard Model Context Protocol (MCP) and provides comprehensive multi-tenant support with enterprise-grade security features.