# Core Package

## Overview

The `core` package implements the central engine for the DevOps MCP platform. It orchestrates context management, GitHub integration, embedding processing, and adapter coordination. The package provides the foundation for AI-powered DevOps automation with conversation context awareness and comprehensive GitHub content analysis.

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

// Execute tool action with context
result, err := engine.ExecuteWithContext(ctx, &ExecuteRequest{
    Tool:   "github",
    Action: "create_issue",
    Params: map[string]interface{}{
        "title": "Fix memory leak",
        "body":  "Description...",
    },
    ContextID: contextID,
})

// Process GitHub content
err = engine.ProcessGitHubContent(ctx, &GitHubContent{
    Type:       "issue",
    Owner:      "S-Corkum",
    Repo:       "devops-mcp",
    Number:     123,
    Content:    issueData,
})
```

## Context Management

### Context Manager

Manages conversation contexts with persistence:

```go
// Create context
context, err := contextManager.CreateContext(ctx, &CreateContextRequest{
    Name:    "deployment-planning",
    Content: "Planning production deployment...",
    Type:    "conversation",
    Metadata: map[string]interface{}{
        "project": "devops-mcp",
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
// Store GitHub issue
err = engine.StoreGitHubContent(ctx, &GitHubIssue{
    Owner:     "S-Corkum",
    Repo:      "devops-mcp",
    Number:    123,
    Title:     "Memory leak in worker",
    Body:      "Description...",
    State:     "open",
    Labels:    []string{"bug", "performance"},
    CreatedAt: time.Now(),
})

// Store pull request
err = engine.StoreGitHubPR(ctx, &GitHubPullRequest{
    Owner:      "S-Corkum",
    Repo:       "devops-mcp",
    Number:     456,
    Title:      "Fix memory leak",
    Body:       "This PR fixes...",
    State:      "open",
    BaseRef:    "main",
    HeadRef:    "fix/memory-leak",
    Commits:    3,
    Additions:  45,
    Deletions:  12,
})

// Query stored content
issues, err := engine.GetGitHubIssues(ctx, &IssueQuery{
    Owner:  "S-Corkum",
    Repo:   "devops-mcp",
    State:  "open",
    Labels: []string{"bug"},
})
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

### Embedding Interface

```go
type EmbeddingManager interface {
    // Generate embedding for text
    GenerateEmbedding(ctx context.Context, text string, model string) ([]float32, error)
    
    // Batch generate embeddings
    BatchGenerateEmbeddings(ctx context.Context, texts []string, model string) ([][]float32, error)
    
    // Store embedding
    StoreEmbedding(ctx context.Context, embedding *Embedding) error
    
    // Search similar embeddings
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
    Repo:   "devops-mcp",
    Number: 123,
})

// Process pull request discussions
embeddings, err := embeddingManager.ProcessDiscussion(ctx, &DiscussionRequest{
    PRID:     456,
    Comments: prComments,
})
```

## Adapter Context Bridge

Integrates tools with context awareness:

```go
// Execute tool with context recording
bridge := NewAdapterContextBridge(adapters, contextManager)

result, err := bridge.ExecuteWithContext(ctx, &ExecuteRequest{
    Tool:      "github",
    Action:    "create_issue",
    Params:    params,
    ContextID: contextID,
})

// Automatic context updates:
// 1. Records tool request in context
// 2. Executes tool action
// 3. Records tool response in context
// 4. Updates token count
// 5. Applies truncation if needed
```

## Event System

Publish/subscribe for system events:

```go
// Initialize event bus
eventBus := NewSystemEventBus()

// Subscribe to events
sub := eventBus.Subscribe("github.issue.created", func(event Event) error {
    issue := event.Data.(*GitHubIssue)
    // Process new issue
    return nil
})
defer sub.Unsubscribe()

// Publish events
err = eventBus.Publish(&Event{
    Type: "github.issue.created",
    Data: issue,
    Metadata: map[string]interface{}{
        "source": "webhook",
    },
})

// Event types
const (
    EventContextCreated    = "context.created"
    EventContextUpdated    = "context.updated"
    EventGitHubIssue       = "github.issue.*"
    EventGitHubPR          = "github.pr.*"
    EventEmbeddingCreated  = "embedding.created"
    EventToolExecuted      = "tool.executed"
)
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

## Future Enhancements

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