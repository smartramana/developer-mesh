# REST API Database Migrations

The REST API now supports automatic database migrations on startup.

## Configuration

### Environment Variables

- `SKIP_MIGRATIONS` - Set to "true" to skip migrations on startup
- `MIGRATIONS_PATH` - Override the default migration directory path
- `MIGRATIONS_FAIL_FAST` - Set to "true" to exit on migration failure (default: log and continue)

### Command Line Flags

- `--skip-migration` - Skip database migrations on startup
- `--migrate` - Run migrations and exit (useful for init containers)

## Default Behavior

By default, the REST API will:
1. Run all pending migrations on startup
2. Continue running even if migrations fail (unless MIGRATIONS_FAIL_FAST=true)
3. Look for migrations in standard locations:
   - `/app/migrations/sql` (Docker production)
   - `apps/rest-api/migrations/sql` (development)

## Usage Examples

```bash
# Run normally with migrations
./rest-api

# Skip migrations
./rest-api --skip-migration

# Run migrations only and exit
./rest-api --migrate

# Skip via environment
SKIP_MIGRATIONS=true ./rest-api

# Fail on migration errors
MIGRATIONS_FAIL_FAST=true ./rest-api
```

## Docker Deployment

The Dockerfile automatically copies migrations to `/app/migrations/sql`, so they're available in the container.

For Kubernetes/Docker deployments, you can:
1. Let the app run migrations on startup (default)
2. Use an init container with `--migrate` flag
3. Run migrations separately and use `--skip-migration`