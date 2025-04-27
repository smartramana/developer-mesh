# Database Migration Guide

This document explains how to use the database migration system in the MCP Server.

## Table of Contents

1. [Overview](#overview)
2. [Automatic Migrations](#automatic-migrations)
3. [Creating New Migrations](#creating-new-migrations)
4. [Running Migrations Manually](#running-migrations-manually)
5. [Rollback Migrations](#rollback-migrations)
6. [Handling Migration Errors](#handling-migration-errors)
7. [Migration Best Practices](#migration-best-practices)

## Overview

The MCP Server uses a database migration system based on [golang-migrate](https://github.com/golang-migrate/migrate) to manage database schema changes. The migration system handles:

- Automatic migration during application startup
- Creating new migration files
- Applying migrations
- Rolling back migrations
- Fixing migration errors

Migrations are stored as SQL files in the `migrations/sql` directory, with pairs of `up` and `down` files for each migration:

- **up.sql** files contain the SQL to make the schema change
- **down.sql** files contain the SQL to undo the change

## Automatic Migrations

By default, the MCP Server will automatically run any pending migrations during startup. This behavior can be controlled through configuration settings:

```yaml
database:
  auto_migrate: true                  # Enable/disable automatic migrations
  migrations_path: "migrations/sql"   # Path to migration files
  migration_timeout: "1m"             # Timeout for migration operations
  fail_on_migration_error: true       # Whether to fail application startup on migration errors
```

When `auto_migrate` is enabled, the application will:

1. Check the current database version
2. Apply any pending migrations
3. Log the results of the migration process

If migrations fail and `fail_on_migration_error` is true, the application will fail to start, ensuring that you don't run with an incompatible database schema.

## Creating New Migrations

To create a new migration, use the migrate command line tool:

```bash
go run cmd/migrate/main.go -create -name "add_user_fields"
```

This will create two new files in the migrations directory:
- `NNN_add_user_fields.up.sql` - For implementing the changes
- `NNN_add_user_fields.down.sql` - For rolling back the changes

The `NNN` is a sequential version number that ensures migrations are applied in the correct order.

After creating the migration files, edit them to add the necessary SQL statements. For example:

**up.sql**:
```sql
-- Add user_id column to contexts table
ALTER TABLE mcp.contexts ADD COLUMN user_id VARCHAR(255);
CREATE INDEX idx_contexts_user_id ON mcp.contexts(user_id);
```

**down.sql**:
```sql
-- Remove user_id column from contexts table
DROP INDEX IF EXISTS idx_contexts_user_id;
ALTER TABLE mcp.contexts DROP COLUMN IF EXISTS user_id;
```

## Running Migrations Manually

During development or in CI/CD pipelines, you may want to run migrations manually without starting the full application. Use the migrate command line tool:

```bash
# Apply all pending migrations
go run cmd/migrate/main.go -up -dsn "postgres://username:password@localhost:5432/mcp_db?sslmode=disable"

# Apply only a specific number of migrations
go run cmd/migrate/main.go -up -steps 1 -dsn "postgres://username:password@localhost:5432/mcp_db?sslmode=disable"
```

For production environments, you can build the migration tool once and use it for all operations:

```bash
# Build the migration tool
go build -o migrate cmd/migrate/main.go

# Run migrations
./migrate -up -dsn "postgres://username:password@localhost:5432/mcp_db?sslmode=disable"
```

## Rollback Migrations

If you need to undo migrations, you can roll them back using the migrate tool:

```bash
# Roll back the most recent migration
go run cmd/migrate/main.go -down -dsn "postgres://username:password@localhost:5432/mcp_db?sslmode=disable"

# Roll back all migrations
go run cmd/migrate/main.go -reset -dsn "postgres://username:password@localhost:5432/mcp_db?sslmode=disable"
```

## Handling Migration Errors

If a migration fails, the database is marked as "dirty" and no further migrations will be applied until the issue is fixed. To check the status:

```bash
go run cmd/migrate/main.go -version -dsn "postgres://username:password@localhost:5432/mcp_db?sslmode=disable"
```

If you see that the database is in a dirty state, you need to:

1. Fix the issue that caused the migration to fail
2. Force the database version to the correct state:

```bash
# Force database to version N (use the correct version number)
go run cmd/migrate/main.go -force N -dsn "postgres://username:password@localhost:5432/mcp_db?sslmode=disable"
```

## Migration Best Practices

1. **Always test migrations**: Test every migration in a development environment before applying it to production.

2. **Write reversible migrations**: Ensure every `up` migration has a corresponding `down` migration that properly undoes the changes.

3. **Keep migrations idempotent**: When possible, write migrations that can be run multiple times without errors (e.g., use `CREATE TABLE IF NOT EXISTS` instead of just `CREATE TABLE`).

4. **Include transactions**: Wrap multiple SQL statements in a transaction to ensure they are applied atomically.

5. **Use descriptive names**: Give migrations clear, descriptive names that explain what they do.

6. **Document changes**: Include comments in the migration files explaining what they do and why.

7. **Keep migrations small**: Make each migration focus on a specific change rather than bundling many unrelated changes.

8. **Version control**: Always keep migrations in version control and never modify existing migration files once they've been applied to any environment.

9. **Back up data**: Before running migrations in production, always back up your database.

10. **Separate schema and data migrations**: When possible, separate structural schema changes from data migrations that modify existing records.
