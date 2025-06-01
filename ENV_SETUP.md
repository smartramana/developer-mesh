# Environment Setup Guide

This project follows a simple, industry-standard pattern for environment configuration.

## Environment Files

We use a **single .env pattern**:

- **`.env.example`** - The template with ALL variables documented (committed to git)
- **`.env`** - Your local configuration (gitignored)

That's it! No confusion with multiple .env files.

## Quick Start

```bash
# 1. Copy the template
cp .env.example .env

# 2. Edit with your values
nano .env  # At minimum, add your GitHub token

# 3. Run locally
make local-native  # Uses localhost (from your .env)

# 4. Or run with Docker
make local-dev     # Uses Docker service names (hardcoded in docker-compose.yml)
```

## How It Works

### Running Locally (make local-native)
- Uses values from your `.env` file
- Expects PostgreSQL and Redis on localhost
- Example: `DATABASE_HOST=localhost`

### Running with Docker (make local-dev)
- Docker Compose has hardcoded service names
- Overrides database/redis hosts to use container names
- Example: `DATABASE_HOST=database` (set in docker-compose.yml)
- Still uses your `.env` for GitHub tokens, API keys, etc.

### Production
- Don't use .env files at all
- Set environment variables in your deployment platform:
  - Kubernetes: ConfigMaps and Secrets
  - AWS: Systems Manager Parameter Store
  - Heroku: Config Vars
  - Docker: --env-file or secrets

## Key Variables to Set

For local development, you typically only need to set:
```bash
# Required for GitHub integration
GITHUB_TOKEN=your-personal-access-token

# Optional - defaults work for local dev
DATABASE_HOST=localhost  # or your database host
REDIS_HOST=localhost     # or your redis host
```

## Best Practices

1. **Never commit** your `.env` file
2. **Keep .env.example updated** when adding new variables
3. **Use descriptive names** for environment variables
4. **Document each variable** in .env.example
5. **Use sensible defaults** where possible

## Troubleshooting

If Docker services can't connect:
- Check `docker-compose.local.yml` for the correct service names
- Docker services use internal names: `database`, `redis`, `localstack`

If local services can't connect:
- Ensure PostgreSQL and Redis are running
- Check your `.env` has `localhost` for DATABASE_HOST and REDIS_HOST
- Verify ports match (5432 for PostgreSQL, 6379 for Redis)