# Database Package

## Overview

The `database` package provides PostgreSQL database connectivity and management for the Developer Mesh platform, including connection pooling, migrations, and specialized repositories.

## Core Components

### Database Connection (`database.go`)
- PostgreSQL connection management with pgx/sqlx
- Connection pooling configuration
- Health checks and readiness probes
- Retry logic with exponential backoff
- Schema search path configuration (mcp, public)

### Configuration (`config.go`)
- Database configuration structures
- Environment variable support
- Connection string generation
- SSL mode configuration

### Context Management (`context.go`, `context_reference.go`)
- Context storage and retrieval
- Context reference tracking
- Session context management

### GitHub Content Storage (`github_content.go`)
- GitHub repository content caching
- File content storage
- Repository metadata tracking

### Migration Support (`migrations.go`)
- Database schema migrations
- Migration status tracking
- Rollback support

### Readiness Checks (`readiness.go`)
- Table existence verification
- Schema validation
- Database health monitoring
- Startup synchronization

## Usage

```go
// Create database connection
cfg := &Config{
    Host:       "localhost",
    Port:       5432,
    Name:       "devmesh_development",
    User:       "devmesh",
    Password:   "password",
    SSLMode:    "disable",
    SearchPath: "mcp,public",
}

db, err := NewDatabase(ctx, cfg)
if err != nil {
    return err
}
defer db.Close()

// Check database health
if err := db.Ping(); err != nil {
    return err
}
```

## Key Features

- **Connection Pooling**: Configurable pool size and lifetime
- **Health Checks**: Built-in health and readiness probes
- **Schema Support**: Multi-schema support with search path
- **Retry Logic**: Automatic retry with exponential backoff
- **Migration Management**: Database schema versioning
- **Context Storage**: Persistent context management

## Database Schema

All tables use the `mcp` schema prefix. Key tables include:
- `mcp.contexts` - Session contexts
- `mcp.context_references` - Context reference tracking
- `mcp.github_contents` - GitHub content cache
- `mcp.agents` - Agent configurations
- `mcp.tool_configurations` - Dynamic tool configs

## Testing

The package includes comprehensive test coverage with test helpers for:
- Database setup and teardown
- Transaction management
- Test data fixtures
- Mock database connections