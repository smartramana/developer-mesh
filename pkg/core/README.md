# Core Package

## Overview

The `core` package implements the central engine for the Developer Mesh platform. It orchestrates context management, GitHub integration, and adapter coordination. The package provides the foundation for AI-powered DevOps automation with conversation context awareness.

**Implementation Status**: The core engine provides basic orchestration. Many features shown in the examples (like `ExecuteWithContext`, `ProcessGitHubContent`) are documented but not implemented in the current codebase.

## Architecture

```
core/
├── engine.go                      # Main orchestration engine
├── context_manager.go            # Context management wrapper
├── context/                      # Full context implementation
│   └── manager.go               # Context manager with S3 support
├── github_content.go            # GitHub content storage
├── github_relationship_manager.go # Entity relationship tracking
├── embedding_manager.go         # Embedding generation interface
├── adapter_context_bridge.go    # Tool-context integration
├── system.go                    # Event bus implementation
└── fallback_service.go          # Degraded mode operations
```

## Core Engine

The engine orchestrates all MCP functionality:

```go
// Initialize engine
engine := core.NewEngine(
    adapters,           // Tool adapters
    contextManager,     // Context management
    metricsClient,      // Metrics collection
    systemEventBus,     // Event system
    logger,            // Structured logging
)

// Execute adapter action (actual method signature)
result, err := engine.ExecuteAdapterAction(ctx, contextID, "github", "create_issue", 
    map[string]interface{}{
        "title": "Fix memory leak",
        "body":  "Description...",
    },
)

// Handle adapter webhook
err = engine.HandleAdapterWebhook(ctx, "github", "issue.created", payload)

// Note: ExecuteWithContext and ProcessGitHubContent methods shown in examples
// are not implemented in the current Engine struct
```

## Context Management

### Context Manager

Manages conversation contexts with persistence:

```go
// Get context manager from engine
contextManager := engine.GetContextManager()

// Create context (actual method signature from context/manager.go)
context, err := contextManager.CreateContext(ctx, &models.MCPContext{
    Name:    "deployment-planning",
    Content: "Planning production deployment...",
    Type:    "conversation",
    Metadata: map[string]interface{}{
        "project": "developer-mesh",
        "phase":   "planning",
    },
})

// Update context with new messages
updated, err := contextManager.UpdateContext(ctx, &UpdateContextRequest{
    ID: context.ID,
    Messages: []Message{
        {
            Role:    "user",
            Content: "What are the deployment steps?",
        },
        {
            Role:    "assistant",
            Content: "Here are the deployment steps...",
        },
    },
})

// Retrieve context
context, err := contextManager.GetContext(ctx, contextID)

// Search contexts
results, err := contextManager.SearchContexts(ctx, &SearchRequest{
    Query:  "deployment",
    Limit:  10,
    Offset: 0,
})
```

### S3 Persistence

Contexts are persisted to S3 for durability:

```go
// S3 storage structure
s3://bucket/contexts/{context_id}/
├── metadata.json    # Context metadata
├── messages.json    # Conversation messages
└── embeddings.json  # Generated embeddings

// Automatic S3 sync
// Contexts are automatically synced to S3 on updates
// Large contexts are offloaded to S3 to save database space
```

### Token Management

Automatic token counting and truncation:

```go
// Token limits
const (
    MaxTokensPerContext = 128000  // Claude's context window
    WarningThreshold    = 100000  // Warning at 78% capacity
)

// Truncation strategies
type TruncationStrategy string

const (
    // Remove oldest messages first
    TruncationOldestFirst TruncationStrategy = "oldest_first"
    
    // Keep user messages, truncate assistant
    TruncationPreserveUser TruncationStrategy = "preserve_user"
    
    // Keep most relevant messages
    TruncationRelevanceBased TruncationStrategy = "relevance_based"
)

// Configure truncation
manager := NewContextManager(
    db,
    WithTruncationStrategy(TruncationRelevanceBased),
    WithMaxTokens(100000),
)
```

## GitHub Integration

### Content Storage

Store and index GitHub content:

```go
// Get GitHub content manager from engine
githubManager := engine.GetGitHubContentManager()

// Store content using the storage manager
// Note: The specific StoreGitHubContent, StoreGitHubPR methods shown in examples
// are not implemented. The actual implementation uses a different API.

// The GitHubContentManager provides:
// - Storage of GitHub content to S3
// - Database persistence
// - Relationship management
```
```

### Relationship Management

Track relationships between GitHub entities:

```go
// Relationship types
const (
    RelationshipReferences = "references"    // Issue references another
    RelationshipMentions   = "mentions"      // User mentions
    RelationshipLinked     = "linked"        // Linked issues/PRs
    RelationshipDuplicate  = "duplicate"     // Duplicate issues
    RelationshipBlocks     = "blocks"        // Blocking issues
    RelationshipImplements = "implements"    // PR implements issue
)

// Process relationships
relationships := engine.ProcessRelationships(ctx, &GitHubContent{
    Type:    "pull_request",
    Content: prData,
})

// Query relationships
related, err := engine.GetRelatedEntities(ctx, &EntityQuery{
    EntityType: "issue",
    EntityID:   "123",
    RelationType: RelationshipLinked,
})
```

### Content Processing Pipeline

```go
// Full processing pipeline
err = engine.ProcessGitHubContent(ctx, content)
// 1. Store in S3
// 2. Index in database
// 3. Extract relationships
// 4. Generate embeddings
// 5. Update search index
// 6. Publish events
```

## Embedding Management

**Note**: The embedding management interface shown is not implemented in the current core package. Embedding functionality is handled by the embedding package and REST API.

### Planned Interface

```go
// This interface is documented but not yet implemented
type EmbeddingManager interface {
    GenerateEmbedding(ctx context.Context, text string, model string) ([]float32, error)
    BatchGenerateEmbeddings(ctx context.Context, texts []string, model string) ([][]float32, error)
    StoreEmbedding(ctx context.Context, embedding *Embedding) error
    SearchSimilar(ctx context.Context, query []float32, limit int) ([]*Embedding, error)
}
```

### Content Processing

Process different content types:

```go
// Process code chunks
embeddings, err := embeddingManager.ProcessCodeChunks(ctx, &CodeChunkRequest{
    FilePath: "pkg/core/engine.go",
    Language: "go",
    ChunkSize: 500,
    Overlap: 50,
})

// Process GitHub issues
embeddings, err := embeddingManager.ProcessIssue(ctx, &IssueRequest{
    Owner:  "S-Corkum",
    Repo:   "developer-mesh",
    Number: 123,
})

// Process pull request discussions
embeddings, err := embeddingManager.ProcessDiscussion(ctx, &DiscussionRequest{
    PRID:     456,
    Comments: prComments,
})
```

## Adapter Context Bridge

The adapter context bridge is implemented in `adapter_context_bridge.go` but with a different API:

```go
// The bridge wraps adapter execution with context recording
// Actual implementation differs from the documented example

// The engine provides webhook recording for context:
err = engine.RecordWebhookInContext(ctx, agentID, "github", "issue.created", payload)
```

## Event System

The package uses the events system from `pkg/events`:

```go
// The engine has an internal eventBus (*events.EventBusImpl)
// Event publishing happens internally during operations

// System events are defined in pkg/events/system/
// The actual event bus implementation differs from the examples shown
```

## Fallback Service

Ensures basic functionality during failures:

```go
// Fallback service provides
fallback := NewFallbackService()

// Emergency health check
health := fallback.HealthCheck()

// Emergency ID generation
id := fallback.GenerateID()

// Emergency event logging
fallback.LogEvent("Service degraded", map[string]interface{}{
    "reason": "Database unavailable",
    "time":   time.Now(),
})

// Use in engine
engine := NewEngine(
    adapters,
    contextManager,
    WithFallbackService(fallback),
)
```

## Database Adapter

Compatibility layer for migration:

```go
// Wrap pkg/database with internal/database interface
adapter := NewDatabaseAdapter(pkgDB)

// Use adapter with legacy code
legacyService := NewLegacyService(adapter)

// Gradual migration path:
// 1. Wrap new implementation
// 2. Update services one by one
// 3. Remove adapter when complete
```

## Error Handling

Comprehensive error types:

```go
// Context errors
var (
    ErrContextNotFound = errors.New("context not found")
    ErrContextTooLarge = errors.New("context exceeds token limit")
    ErrInvalidContext  = errors.New("invalid context format")
)

// GitHub errors
var (
    ErrGitHubRateLimit = errors.New("GitHub rate limit exceeded")
    ErrGitHubNotFound  = errors.New("GitHub resource not found")
    ErrGitHubAuth      = errors.New("GitHub authentication failed")
)

// Engine errors
var (
    ErrAdapterNotFound = errors.New("adapter not found")
    ErrActionFailed    = errors.New("action execution failed")
    ErrInvalidParams   = errors.New("invalid parameters")
)
```

## Metrics and Monitoring

Built-in metrics collection:

```go
// Automatic metrics
- engine_operations_total
- engine_operation_duration_seconds
- context_tokens_total
- github_content_processed_total
- embedding_generations_total
- event_published_total
- adapter_executions_total
```

## Testing

### Unit Tests

```go
// Mock dependencies
mockDB := NewMockDatabase()
mockS3 := NewMockS3Client()
mockAdapters := NewMockAdapterRegistry()

engine := NewEngine(
    mockAdapters,
    NewContextManager(mockDB, mockS3),
    NewMockMetricsClient(),
    NewMockEventBus(),
)

// Test operations
result, err := engine.ExecuteWithContext(ctx, request)
assert.NoError(t, err)
```

### Integration Tests

```go
// Test with real services
func TestEngineIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Setup real dependencies
    db := setupTestDB(t)
    s3 := setupTestS3(t)
    
    // Test full workflow
    engine := NewEngine(...)
    
    // Create context
    // Execute tools
    // Verify results
}
```

## Best Practices

1. **Always use context**: Pass context for cancellation and tracing
2. **Handle token limits**: Monitor context size and truncate appropriately
3. **Cache embeddings**: Reuse embeddings to reduce costs
4. **Batch operations**: Process multiple items together
5. **Monitor events**: Subscribe to relevant events
6. **Test fallbacks**: Ensure degraded mode works
7. **Version content**: Track content versions in S3

## Performance Optimization

- **S3 Caching**: Cache frequently accessed contexts
- **Embedding Cache**: Cache generated embeddings
- **Batch Processing**: Process GitHub content in batches
- **Async Events**: Handle events asynchronously
- **Connection Pooling**: Reuse database connections

## Migration Guide

When migrating from internal to pkg:

1. Use database adapter for compatibility
2. Update imports gradually
3. Test thoroughly at each step
4. Remove adapters when complete

## Implementation Gaps

The following features are documented but not yet implemented:

- ExecuteWithContext method on Engine
- ProcessGitHubContent method on Engine  
- StoreGitHubContent/StoreGitHubPR methods
- EmbeddingManager interface and implementation
- Full event system as documented
- Many of the content processing pipeline features

## Future Enhancements

- [ ] Implement missing Engine methods
- [ ] Add embedding management
- [ ] GraphQL support for GitHub
- [ ] Real-time context sync
- [ ] Advanced relationship analysis
- [ ] Context versioning
- [ ] Distributed context management
- [ ] ML-powered truncation

## References

- [MCP Documentation](https://modelcontextprotocol.io/)
- [GitHub API](https://docs.github.com/en/rest)
- [AWS S3 Best Practices](https://docs.aws.amazon.com/AmazonS3/latest/userguide/best-practices.html)
- [Embedding Models](https://platform.openai.com/docs/guides/embeddings)