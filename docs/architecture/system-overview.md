# DevOps MCP System Architecture Overview

## Introduction

DevOps MCP (Model Context Protocol) is designed as a modular system using Go workspace architecture with multiple modules. This document provides a high-level overview of the system components and how they interact.

## System Components

The DevOps MCP system consists of several main components:

![System Architecture](../assets/images/system-architecture.png)

### MCP Server

The MCP Server is the central component that:
- Processes requests from AI agents
- Manages authentication and authorization
- Routes requests to appropriate handlers
- Coordinates between different subsystems

**Location**: `apps/mcp-server`

### REST API Service

The REST API service:
- Provides RESTful endpoints for tool integrations
- Handles CRUD operations for resources
- Manages vector embeddings for search
- Implements the adapter pattern to connect with repositories

**Location**: `apps/rest-api`

### Worker Service

The Worker service:
- Processes asynchronous tasks from a queue
- Handles long-running operations
- Ensures idempotent processing
- Implements retry logic for failed operations

**Location**: `apps/worker`

### Shared Libraries

The shared packages in `pkg/` provide:
- Common interfaces and models
- Database access logic
- Configuration management
- Utility functions used across services

**Location**: `pkg/`

## Data Flow

1. **Request Processing**:
   - Client sends request to MCP Server or REST API
   - Request is authenticated and validated
   - Handler dispatches to appropriate service

2. **Vector Search**:
   - Embedding vectors are stored in PostgreSQL with pgvector
   - Search queries compute similarity using vector operations
   - Results include similarity scores in metadata

3. **Asynchronous Processing**:
   - Events are written to SQS queue
   - Worker service consumes events
   - Idempotency is maintained using Redis

## Repository Layer

The repository layer follows a clean architecture approach:

1. **Core Interfaces** (`pkg/repository/interfaces.go`)
   - Define contracts for repository implementations

2. **Implementation-Specific Repositories**
   - SQL-based implementations
   - In-memory implementations for testing

3. **Adapters**
   - Bridge between API expectations and repository implementations
   - Handle type conversion and field mapping
   
For more details on the adapter pattern implementation, see [Adapter Pattern](adapter-pattern.md).

## Storage System

DevOps MCP uses multiple storage systems:

- **PostgreSQL with pgvector**: For persistent data and vector operations
- **Redis**: For caching and idempotency tracking
- **LocalStack (SQS)**: For queue management in development

## Cross-Cutting Concerns

Several aspects apply across the system:

- **Configuration**: Environment-based configuration with defaults
- **Logging**: Structured logging with context-based fields
- **Error Handling**: Standardized error types and wrapping
- **Metrics**: Prometheus metrics for monitoring
- **Health Checks**: Component-level health reporting
