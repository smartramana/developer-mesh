# DevOps MCP Configuration

This directory contains the configuration files for the DevOps MCP platform following industry best practices.

## Configuration Structure

```
configs/
├── README.md                # This file
├── config.base.yaml        # Base configuration with defaults
├── config.development.yaml # Development environment
├── config.staging.yaml     # Staging environment  
├── config.production.yaml  # Production environment
├── config.test.yaml       # Test environment
├── config.docker.yaml     # Docker Compose environment
├── grafana/               # Grafana dashboard configs
├── prometheus.yml         # Prometheus monitoring config
└── redis-cluster/         # Redis cluster configurations
```

## Environment Configuration

### Local Development

1. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` with your settings (GitHub token, etc.)

3. Choose your development method:

   **Option A: With Docker (Recommended)**
   ```bash
   make local-dev
   # Or directly: docker-compose -f docker-compose.local.yml up
   ```
   Docker Compose will use its built-in service names (database, redis, etc.)

   **Option B: Without Docker**
   ```bash
   # Ensure PostgreSQL and Redis are running locally
   make local-native
   # Or directly: ./devops-mcp server
   ```
   This uses localhost connections from your .env file

### Production Deployment

1. Set environment variables from your secret management system:
   ```bash
   export ENVIRONMENT=production
   export JWT_SECRET=$(aws secretsmanager get-secret-value --secret-id jwt-secret --query SecretString --output text)
   export DATABASE_HOST=your-rds-instance.region.rds.amazonaws.com
   # ... other production values
   ```

2. Deploy with your orchestration tool (Kubernetes, ECS, etc.)

## Configuration Hierarchy

Configurations are loaded and merged in this order:

1. `config.base.yaml` - Base defaults for all environments
2. `config.{environment}.yaml` - Environment-specific overrides
3. `config.{environment}.local.yaml` - Local overrides (git-ignored)
4. Environment variables - Highest priority

### Example

```yaml
# config.base.yaml
api:
  timeout: 30s
  
# config.production.yaml  
api:
  timeout: 60s  # Override for production
  
# Environment variable
export API_TIMEOUT=120s  # Final value used
```

## Key Configuration Sections

### API Configuration
- Server settings (port, timeouts, TLS)
- CORS configuration
- Rate limiting
- Authentication requirements

### Authentication
- JWT settings
- API key management
- OAuth2 providers
- Security policies

### Database
- Connection settings
- Pool configuration
- Migration settings
- Read replica configuration

### Cache
- Local cache settings
- Redis/ElastiCache configuration
- Cache key patterns

### Storage
- Context storage provider (S3, filesystem, database)
- Embedding storage settings
- Encryption configuration

### Monitoring
- Logging configuration
- Metrics collection
- Distributed tracing
- Health check endpoints

## Environment Variables

All configuration values can be overridden via environment variables:

- Dots become underscores: `api.timeout` → `API_TIMEOUT`
- Arrays use comma separation: `CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:8080`
- Nested values: `auth.jwt.secret` → `AUTH_JWT_SECRET`

## Security Best Practices

1. **Never commit secrets** - Use environment variables or secret management
2. **Use strong JWT secrets** - Minimum 32 characters
3. **Rotate API keys regularly** - 90-day rotation recommended
4. **Enable TLS in production** - Required for secure communication
5. **Restrict CORS origins** - Only allow specific domains in production

## Adding New Configuration

1. Add defaults to `config.base.yaml`
2. Add environment-specific overrides as needed
3. Document the new configuration in this README
4. Update the configuration loader validation if required

## Troubleshooting

### Configuration Not Loading

```bash
# Check which config file is being used
./devops-mcp config show

# Validate configuration
./devops-mcp config validate
```

### Environment Variables Not Working

```bash
# Check if variable is set
echo $DATABASE_HOST

# Export the variable
export DATABASE_HOST=localhost

# Or use .env file
echo "DATABASE_HOST=localhost" >> .env
```

### Docker Services Can't Connect

Ensure you're using Docker service names, not localhost:
- `database` not `localhost` for PostgreSQL
- `redis` not `localhost` for Redis
- `localstack` not `localhost` for AWS services

## Migration from Old Config

If you have old configuration files:

1. Move values from `config.yaml` to appropriate environment configs
2. Extract secrets to environment variables
3. Delete old config files
4. Update deployment scripts to set `ENVIRONMENT`

## Related Documentation

- [Configuration Guide](../docs/operations/configuration-guide.md) - Detailed configuration documentation
- [Authentication Guide](../docs/operations/authentication-operations-guide.md) - Auth-specific configuration
- [Docker Compose Guide](../docker-compose.local.yml) - Docker-specific setup