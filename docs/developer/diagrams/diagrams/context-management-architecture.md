# Context Management Architecture

## Overview

The Context Management architecture enables AI agents to maintain conversation histories and relevant state across interactions. This document explains the architecture, storage patterns, and integration points of the context management system.

## Architecture Diagram

```mermaid
graph TD
    subgraph "API Layer"
        ContextAPI[Context API] --> ContextManager
    end
    
    subgraph "Core Layer"
        ContextManager[Context Manager]
        ContextManager -- Events --> EventBus[Event Bus]
        ContextManager --> CacheLayer[Cache Layer]
        ContextManager --> DatabaseLayer[Database Layer]
        ContextManager --> StorageLayer[Storage Layer]
    end
    
    subgraph "Persistence Layer"
        CacheLayer --> Redis[(Redis)]
        DatabaseLayer --> PostgreSQL[(PostgreSQL + pgvector)]
        StorageLayer --> S3[(AWS S3)]
    end
    
    subgraph "Adapters"
        AdapterManager[Adapter Manager] --> ContextManager
        AdapterBridge[Adapter Bridge] --> ContextManager
    end
    
    subgraph "External Tools"
        GitHub[GitHub]
        AWS[AWS Services]
        Other[Other Tools]
    end
    
    AdapterManager --> GitHub
    AdapterManager --> AWS
    AdapterManager --> Other
```

## Components

### Context Manager

The Context Manager is the central component responsible for:

- Creating, retrieving, updating, and deleting contexts
- Managing context content (messages, events)
- Implementing token counting and context window management
- Handling context truncation strategies
- Publishing events for context changes

### Storage Strategy

The context management system uses a tiered storage approach:

1. **Redis Cache**: For fast access to frequently used contexts
   - TTL-based caching to optimize memory usage
   - Used for context metadata and recent content

2. **PostgreSQL Database**: For structured storage of context metadata
   - Stores context metadata and references
   - Uses pgvector extension for semantic search capabilities
   - Enables efficient querying by agent ID, session ID, etc.

3. **S3 Storage**: For large context data
   - Stores complete conversation histories
   - Used for contexts that exceed a certain size threshold
   - Provides durability for long-term storage

### Context Structure

Each context consists of:

- Metadata (agent ID, model ID, session ID, token counts, etc.)
- A sequence of context items (messages, events, tool operations)
- Optional vector embeddings for semantic search

### Integration with Adapters

Adapters interact with the Context Manager to:

- Record tool operations in contexts
- Store webhook events
- Retrieve contextual information for tool operations

### Authentication and Authorization

- Uses AWS IAM Roles for Service Accounts (IRSA) for AWS service access
- Implements role-based access control for context operations
- Ensures isolation between contexts of different agents

## Sequence Diagrams

### Creating a Context

```mermaid
sequenceDiagram
    participant Agent as AI Agent
    participant API as Context API
    participant Manager as Context Manager
    participant DB as Database
    participant Cache as Redis Cache
    participant Storage as S3 Storage
    participant Bus as Event Bus
    
    Agent->>API: Create Context Request
    API->>Manager: CreateContext()
    Manager->>Manager: Validate & Initialize
    Manager->>DB: Store Context Metadata
    Manager->>Storage: Store Context Content
    Manager->>Cache: Cache Context
    Manager->>Bus: Publish ContextCreated Event
    Manager->>API: Return Context
    API->>Agent: Context Created Response
```

### Updating a Context

```mermaid
sequenceDiagram
    participant Agent as AI Agent
    participant API as Context API
    participant Manager as Context Manager
    participant DB as Database
    participant Cache as Redis Cache
    participant Storage as S3 Storage
    participant Bus as Event Bus
    
    Agent->>API: Update Context Request
    API->>Manager: UpdateContext()
    Manager->>Cache: Get Existing Context
    alt Context in Cache
        Cache->>Manager: Return Context
    else Context not in Cache
        Manager->>DB: Get Context Metadata
        Manager->>Storage: Get Context Content
        Manager->>Cache: Cache Context
    end
    Manager->>Manager: Apply Updates
    Manager->>Manager: Check Token Limit
    opt Context Exceeds Token Limit
        Manager->>Manager: Apply Truncation Strategy
    end
    Manager->>DB: Update Context Metadata
    Manager->>Storage: Update Context Content
    Manager->>Cache: Update Cache
    Manager->>Bus: Publish ContextUpdated Event
    Manager->>API: Return Updated Context
    API->>Agent: Context Updated Response
```

## Truncation Strategies

The Context Manager implements multiple truncation strategies:

1. **Oldest First**: Removes the oldest context items first
2. **Preserving User**: Prioritizes removing assistant responses while preserving user messages
3. **Relevance Based**: Uses vector embeddings to remove less relevant context items (planned for future)

## Authentication with AWS Services

The MCP Server uses IAM Roles for Service Accounts (IRSA) to authenticate with AWS services:

1. **S3 Access**: For storing and retrieving context data
2. **RDS Access**: For connecting to Aurora PostgreSQL with IAM authentication
3. **Secrets Manager**: For managing database credentials and other secrets

## Observability

The context management system includes:

- Structured logging for all operations
- Metrics for context operations (creation, update, deletion, etc.)
- Tracing for tracking context operations across components
