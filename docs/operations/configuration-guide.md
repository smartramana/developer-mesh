# Configuration Guide

This guide explains the DevOps MCP configuration system and best practices for managing configurations across different environments.

## Table of Contents

1. [Configuration Overview](#configuration-overview)
2. [Environment-Specific Configurations](#environment-specific-configurations)
3. [Configuration Loading](#configuration-loading)
4. [Environment Variables](#environment-variables)
5. [Security Best Practices](#security-best-practices)
6. [Configuration Reference](#configuration-reference)
7. [Migration Guide](#migration-guide)

## Configuration Overview

DevOps MCP uses a hierarchical configuration system that supports:

- **Base configuration** with sensible defaults
- **Environment-specific overrides** (development, staging, production)
- **Local overrides** for developer customization
- **Environment variable substitution**
- **Secret management integration**

### Configuration Files Structure

```
configs/
├── config.base.yaml           # Base configuration (defaults)
├── config.development.yaml    # Development environment
├── config.staging.yaml        # Staging environment
├── config.production.yaml     # Production environment
├── config.test.yaml          # Test environment
└── config.*.local.yaml       # Local overrides (git-ignored)
```

### Configuration Hierarchy

Configurations are loaded and merged in the following order:

1. `config.base.yaml` - Base defaults
2. `config.{environment}.yaml` - Environment-specific settings
3. `config.{environment}.local.yaml` - Local overrides (optional)
4. Environment variables - Highest priority

## Environment-Specific Configurations

### Development Environment

**File**: `configs/config.development.yaml`

Optimized for local development:
- Authentication can be disabled
- Static API keys for testing
- Local PostgreSQL and Redis
- Debug logging enabled
- Mock services available
- Hot reload enabled

```bash
# Start with development config
export ENVIRONMENT=development
./devops-mcp server
```

### Staging Environment

**File**: `configs/config.staging.yaml`

Similar to production with relaxations:
- Test API keys allowed
- Higher rate limits
- Full request logging
- Tracing enabled at 100%
- Chaos testing available

```bash
# Start with staging config
export ENVIRONMENT=staging
./devops-mcp server
```

### Production Environment

**File**: `configs/config.production.yaml`

Hardened for production use:
- All secrets from environment/vault
- TLS required
- Strict rate limiting
- Minimal logging
- High availability features
- Compliance features enabled

```bash
# Start with production config
export ENVIRONMENT=production
./devops-mcp server
```

### Test Environment

**File**: `configs/config.test.yaml`

Optimized for automated testing:
- In-memory databases
- No external dependencies
- Deterministic behavior
- Fast timeouts
- All mocks enabled

## Configuration Loading

### Using the Configuration Loader

```go
import "github.com/S-Corkum/devops-mcp/pkg/config"

// Load configuration
loader, err := config.LoadConfig("./configs", "production")
if err != nil {
    log.Fatal(err)
}

// Validate configuration
if err := config.ValidateConfig(loader, "production"); err != nil {
    log.Fatal(err)
}

// Access configuration values
port := loader.GetString("api.listen_address")
dbHost := loader.GetString("database.host")
```

### Configuration Inheritance

Use the `_base` directive to inherit from another configuration:

```yaml
# config.staging.yaml
_base: config.base.yaml  # Inherit from base

# Override specific values
api:
  enable_swagger: true
```

### Local Overrides

Create `config.{environment}.local.yaml` for local customization:

```yaml
# config.development.local.yaml
database:
  host: my-local-postgres
  
adapters:
  github:
    token: my-personal-token
```

## Environment Variables

### Variable Substitution

Configuration files support environment variable substitution:

```yaml
database:
  host: "${DB_HOST}"
  password: "${DB_PASSWORD}"
  port: ${DB_PORT:-5432}  # With default value
```

### Override via Environment

Any configuration value can be overridden via environment variables:

```bash
# Override api.listen_address
export API_LISTEN_ADDRESS=:8081

# Override database.host
export DATABASE_HOST=my-database.example.com

# Override nested values (dots become underscores)
export API_AUTH_JWT_SECRET=my-secret
```

### Environment Variable Files

#### Development (.env)

```bash
cp .env.example .env
# Edit .env with your values
```

#### Production (.env.production.example)

Production environments should use proper secret management:

- AWS Secrets Manager
- HashiCorp Vault
- Kubernetes Secrets
- Azure Key Vault

## Security Best Practices

### 1. Never Commit Secrets

```yaml
# BAD - Never do this
auth:
  jwt:
    secret: "actual-secret-value"

# GOOD - Use environment variables
auth:
  jwt:
    secret: "${JWT_SECRET}"
```

### 2. Use Secret Management

Production configuration for AWS Secrets Manager:

```yaml
security:
  secrets:
    provider: "aws-secrets-manager"
    aws_region: "${AWS_REGION}"
    key_prefix: "devops-mcp/"
```

### 3. Rotate Secrets Regularly

```bash
# Rotate JWT secret
aws secretsmanager put-secret-value \
  --secret-id devops-mcp/jwt-secret \
  --secret-string "$(openssl rand -base64 32)"
```

### 4. Restrict Configuration Access

```bash
# Set proper file permissions
chmod 600 configs/config.production.local.yaml

# Use RBAC for Kubernetes ConfigMaps
kubectl create role config-reader \
  --verb=get,list \
  --resource=configmaps
```

### 5. Audit Configuration Changes

```yaml
# Enable configuration audit
audit:
  config_changes:
    enabled: true
    storage: "database"
    retention_days: 90
```

## Configuration Reference

### Core Configuration Sections

#### API Configuration

```yaml
api:
  listen_address: ":8080"      # Server listen address
  base_path: "/api/v1"         # API base path
  enable_swagger: true         # Enable Swagger UI
  enable_pprof: false          # Enable profiling
  
  # Timeouts
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
  
  # TLS
  tls:
    enabled: true
    cert_file: "/path/to/cert"
    key_file: "/path/to/key"
```

#### Authentication Configuration

```yaml
auth:
  # JWT settings
  jwt:
    secret: "${JWT_SECRET}"
    expiration: 24h
    algorithm: "HS256"
    
  # API key settings
  api_keys:
    enable_database: true
    min_length: 16
    rotation_period: 90d
    
  # Rate limiting
  rate_limiting:
    enabled: true
    default:
      max_attempts: 100
      window: 1m
```

#### Database Configuration

```yaml
database:
  driver: "postgres"
  host: "${DB_HOST}"
  port: 5432
  username: "${DB_USER}"
  password: "${DB_PASSWORD}"
  database: "${DB_NAME}"
  
  # Connection pool
  max_open_conns: 100
  max_idle_conns: 25
  conn_max_lifetime: 30m
  
  # SSL/TLS
  ssl_mode: "require"
  
  # Read replicas
  read_replicas:
    enabled: true
    hosts: ["replica1.example.com", "replica2.example.com"]
```

#### Cache Configuration

```yaml
cache:
  # Local cache
  local:
    enabled: true
    size: 10000
    ttl: 60s
    
  # Distributed cache
  distributed:
    type: "redis-cluster"
    cluster_endpoints: ["redis1:6379", "redis2:6379"]
    password: "${REDIS_PASSWORD}"
    
    # TLS
    tls:
      enabled: true
      ca_cert: "/path/to/ca.crt"
```

## Migration Guide

### From Old Configuration Format

If migrating from the old configuration format:

1. **Identify environment-specific values**
   ```yaml
   # Old: Single config with conditionals
   database:
     host: ${DB_HOST:-localhost}
   
   # New: Separate configs
   # config.development.yaml
   database:
     host: localhost
   
   # config.production.yaml
   database:
     host: "${DB_HOST}"
   ```

2. **Extract common values to base**
   ```yaml
   # config.base.yaml
   api:
     base_path: "/api/v1"  # Same across environments
   ```

3. **Move secrets to environment variables**
   ```yaml
   # Remove from config
   auth:
     jwt:
       secret: "hardcoded-secret"
   
   # Use environment variable
   auth:
     jwt:
       secret: "${JWT_SECRET}"
   ```

4. **Update deployment scripts**
   ```bash
   # Old
   ./devops-mcp --config config.yaml
   
   # New
   export ENVIRONMENT=production
   ./devops-mcp server
   ```

### Configuration Validation

Use the built-in validation:

```go
// Validate before starting
if err := config.ValidateConfig(loader, environment); err != nil {
    log.Fatalf("Invalid configuration: %v", err)
}
```

### Health Checks

Configure health checks to verify configuration:

```yaml
monitoring:
  health:
    checks:
      - name: "config"
        critical: true
        check: "validate_required_vars"
```

## Troubleshooting

### Common Issues

1. **Missing environment variables**
   ```
   Error: missing required configuration fields: [auth.jwt.secret]
   Solution: Ensure JWT_SECRET is set in environment
   ```

2. **Configuration not loading**
   ```
   Error: failed to load environment config
   Solution: Check file exists and has correct syntax
   ```

3. **Variable substitution not working**
   ```
   Symptom: Seeing ${VAR} in config values
   Solution: Ensure variable is exported in environment
   ```

### Debug Configuration Loading

```bash
# Enable debug logging
export LOG_LEVEL=debug

# Show loaded configuration
./devops-mcp config show

# Validate configuration
./devops-mcp config validate
```

## Best Practices Summary

1. **Use hierarchical configuration** - Base → Environment → Local
2. **Never commit secrets** - Use environment variables or secret managers
3. **Validate configuration** - Fail fast on missing required values
4. **Document all settings** - Include descriptions and examples
5. **Use consistent naming** - Follow the established patterns
6. **Implement health checks** - Verify configuration at runtime
7. **Audit configuration changes** - Track who changed what and when
8. **Test configuration changes** - Especially for production
9. **Use feature flags** - For gradual rollouts
10. **Monitor configuration drift** - Ensure deployed config matches expected