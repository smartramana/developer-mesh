# Developer Mesh Configuration

This directory contains the configuration files for the Developer Mesh platform following industry best practices.

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
   # Or directly: ./developer-mesh server
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
./developer-mesh config show

# Validate configuration
./developer-mesh config validate
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
- Use real AWS endpoints for production services

## Migration from Old Config

If you have old configuration files:

1. Move values from `config.yaml` to appropriate environment configs
2. Extract secrets to environment variables
3. Delete old config files
4. Update deployment scripts to set `ENVIRONMENT`

## Performance Optimization Quick Reference

### Key Performance Settings

```yaml
# config.production.yaml - Performance optimized
performance:
  go:
    max_procs: 0         # Use all CPUs
    gc_percent: 200      # Less frequent GC for better throughput
    
  http:
    max_connections: 50000
    enable_http2: true
    enable_compression: true
    compression_level: 6
    
  pools:
    database:
      max_open: 500      # Match your RDS max_connections
      max_idle: 100
      lifetime: 30m
    redis:
      pool_size: 1000
      min_idle: 50
      
  workers:
    pool_size: 100       # Adjust based on workload
    queue_size: 10000
    prefetch_count: 50
```

### Performance Environment Variables

```bash
# Quick performance tuning via environment
export GOMAXPROCS=16                    # Override CPU usage
export GOGC=200                         # GC tuning
export HTTP_MAX_CONNECTIONS=100000      # Connection limit
export DB_POOL_MAX_OPEN=1000           # Database connections
export REDIS_POOL_SIZE=2000            # Redis connections
export WORKER_POOL_SIZE=200            # Worker threads
```

### Caching Configuration

```yaml
# Enable multi-level caching for performance
caching:
  memory:
    enabled: true
    size: "4GB"          # In-memory cache size
    ttl: 5m
  redis:
    enabled: true
    ttl: 1h
  s3:
    enabled: true        # For large objects
    bucket: "${CACHE_BUCKET}"
    ttl: 24h
```

## Cost Optimization Quick Reference

### Cost Control Settings

```yaml
# config.production.yaml - Cost controls
cost_management:
  limits:
    daily_limit: 100.00
    session_limit: 1.00
    
  services:
    bedrock:
      session_limit: 0.10
      models:
        - name: "claude-3-opus"
          max_cost_per_day: 20.00
          
  alerts:
    enabled: true
    thresholds:
      - level: "warning"
        percent: 80
      - level: "critical"
        percent: 95
```

### Cost-Saving Environment Variables

```bash
# Quick cost controls via environment
export BEDROCK_SESSION_LIMIT=0.05       # $0.05 per session
export GLOBAL_COST_LIMIT=50.00          # $50 daily limit
export MODEL_SELECTION_STRATEGY=cheapest # Always use cheapest model
export ENABLE_SPOT_INSTANCES=true       # Use spot for workers
export AUTO_SCALE_DOWN_AFTER=300        # Scale down after 5 min
```

### Resource Optimization

```yaml
# Scheduled scaling for cost savings
resource_optimization:
  schedules:
    - name: "business_hours"
      schedule: "0 8 * * MON-FRI"
      min_instances: 3
      max_instances: 20
      
    - name: "off_hours"
      schedule: "0 18 * * MON-FRI"
      min_instances: 1      # Scale down at night
      max_instances: 5
      
    - name: "weekend"
      schedule: "0 0 * * SAT,SUN"
      min_instances: 0      # Shut down on weekends
```

## Quick Reference by Scenario

### Development (Low Cost, Easy Debugging)
```bash
export ENVIRONMENT=development
export LOG_LEVEL=debug
export BEDROCK_SESSION_LIMIT=0.01
export DB_POOL_SIZE=5
export WORKER_POOL_SIZE=2
```

### Staging (Balanced Performance/Cost)
```bash
export ENVIRONMENT=staging
export LOG_LEVEL=info
export BEDROCK_SESSION_LIMIT=0.10
export DB_POOL_SIZE=50
export WORKER_POOL_SIZE=10
export ENABLE_SPOT_INSTANCES=true
```

### Production (High Performance)
```bash
export ENVIRONMENT=production
export LOG_LEVEL=warn
export GOGC=200
export DB_POOL_SIZE=500
export REDIS_POOL_SIZE=1000
export WORKER_POOL_SIZE=100
export MODEL_SELECTION_STRATEGY=performance
```

### Production (Cost Optimized)
```bash
export ENVIRONMENT=production
export LOG_LEVEL=error
export BEDROCK_SESSION_LIMIT=0.05
export MODEL_SELECTION_STRATEGY=cheapest
export ENABLE_SPOT_INSTANCES=true
export AUTO_SCALING_ENABLED=true
export MIN_INSTANCES=1
```

## Monitoring Your Configuration

### Performance Metrics to Watch

```yaml
monitoring:
  metrics:
    - name: "response_time_p99"
      target: "< 100ms"
    - name: "throughput_rps"
      target: "> 1000"
    - name: "error_rate"
      target: "< 0.01"
    - name: "cost_per_request"
      target: "< $0.001"
```

### Cost Metrics to Track

```yaml
cost_monitoring:
  track:
    - "daily_spend"
    - "cost_per_user"
    - "model_usage_by_cost"
    - "infrastructure_utilization"
```

## Common Configuration Patterns

### 1. Start Small, Scale Up
```yaml
# Start with minimal resources
initial:
  instances: 1
  db_connections: 10
  cache_size: "100MB"

# Scale based on metrics
scaling:
  trigger: "cpu > 70%"
  action: "add instance"
```

### 2. Cache Everything Expensive
```yaml
caching:
  embeddings: true      # $0.0001 per call saved
  model_responses: true # $0.003 per call saved
  api_responses: true   # Reduce latency
```

### 3. Use Cheaper Models for Simple Tasks
```yaml
model_routing:
  simple_queries: "titan-embed-v1"     # $0.0001
  complex_queries: "claude-3-sonnet"   # $0.003
  critical_queries: "claude-3-opus"    # $0.015
```

## Configuration Best Practices

1. **Start with `config.base.yaml`** - Put common settings here
2. **Override per environment** - Only change what's needed
3. **Use environment variables for secrets** - Never commit secrets
4. **Monitor configuration impact** - Track metrics after changes
5. **Document why** - Explain non-obvious settings
6. **Test configuration changes** - Especially performance settings
7. **Set resource limits** - Prevent runaway costs
8. **Enable monitoring** - You can't optimize what you don't measure
9. **Review regularly** - Configuration drift happens
10. **Automate validation** - Catch errors before deployment

## Related Documentation

- [Configuration Guide](../docs/operations/configuration-guide.md) - Detailed configuration documentation
- [Configuration Scenarios](../docs/guides/configuration-scenarios.md) - Real-world configuration examples
- [Performance Tuning Guide](../docs/guides/performance-tuning-guide.md) - Deep dive into performance
- [Cost Optimization Guide](../docs/guides/cost-optimization-guide.md) - Comprehensive cost reduction
- [Authentication Guide](../docs/operations/authentication-operations-guide.md) - Auth-specific configuration
- [Docker Compose Guide](../docker-compose.local.yml) - Docker-specific setup