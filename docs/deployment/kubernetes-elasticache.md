# Deploying Developer Mesh with AWS ElastiCache

This guide explains how to deploy Developer Mesh services (Edge MCP and Worker) in Kubernetes with AWS ElastiCache for Redis instead of the bundled Redis container.

## Prerequisites

- Kubernetes cluster (EKS recommended for AWS)
- AWS ElastiCache Redis cluster
- RDS PostgreSQL instance
- kubectl and Helm installed
- Proper network connectivity between K8s cluster and ElastiCache

## Architecture Overview

When using ElastiCache, the architecture changes from:
- **Default**: Services → Local Redis Pod → Storage
- **ElastiCache**: Services → AWS ElastiCache → Managed Redis

## ElastiCache Setup

### 1. Create ElastiCache Cluster

Create an ElastiCache Redis cluster with:
- **Engine**: Redis 7.x
- **Node Type**: cache.t3.medium or larger for production
- **Number of Replicas**: 2+ for high availability
- **Encryption**: In-transit encryption enabled
- **Auth**: AUTH token enabled (optional but recommended)

### 2. Security Group Configuration

Ensure the ElastiCache security group allows inbound traffic on port 6379 from your Kubernetes cluster nodes.

### 3. Get Connection Details

After creation, note:
- **Primary Endpoint**: `your-cluster.abc123.cache.amazonaws.com:6379`
- **AUTH Token**: (if configured)

## Helm Deployment

### Edge MCP Deployment

1. **Create values file** (`values.elasticache.yaml`):

```yaml
# Disable internal Redis
redis:
  enabled: false

# Configure external Redis (ElastiCache)
externalRedis:
  host: "your-cluster.abc123.cache.amazonaws.com"
  port: 6379
  password: ""  # Set if using AUTH
  database: 0
  tls:
    enabled: true
    skipVerify: false

# Edge MCP configuration
edgeMcp:
  cache:
    redis:
      enabled: true
  # ... other configuration
```

2. **Deploy with Helm**:

```bash
helm install edge-mcp ./deployments/k8s/helm/edge-mcp \
  -f ./deployments/k8s/helm/edge-mcp/values.elasticache.yaml \
  --namespace edge-mcp \
  --create-namespace
```

### Worker Deployment

1. **Create values file** (`worker-values.elasticache.yaml`):

```yaml
# Disable internal Redis
redis:
  enabled: false

# Configure external Redis (ElastiCache)
externalRedis:
  host: "your-cluster.abc123.cache.amazonaws.com"
  port: 6379
  password: ""  # Set if using AUTH
  tls:
    enabled: true
    skipVerify: false

# Worker configuration
worker:
  replicaCount: 3
  database:
    host: "your-rds.region.rds.amazonaws.com"
    # ... database configuration
```

2. **Deploy with Helm**:

```bash
helm install worker ./deployments/k8s/helm/worker \
  -f ./deployments/k8s/helm/worker/values.elasticache.yaml \
  --namespace edge-mcp
```

## Environment Variable Resolution

The Helm charts now configure Redis environment variables consistently across all services:

| Service | Environment Variable | Format | Purpose |
|---------|---------------------|--------|---------|
| All Services | `REDIS_ADDR` | `host:port` | Redis address for all connections |
| Edge MCP | `EDGE_MCP_REDIS_DB` | Number | Database number (default: 0) |
| Edge MCP | `EDGE_MCP_REDIS_ENABLED` | Boolean | Enable Redis cache layer |

**Important**: All services now use the same `REDIS_ADDR` format:
- Always just `host:port` (e.g., `elasticache.amazonaws.com:6379`)
- Never includes protocol (`redis://` or `rediss://`)
- Never includes authentication (`user:pass@`)
- Never includes database number (`/0`)

Authentication and TLS are configured via separate environment variables:
- `REDIS_PASSWORD`: For AUTH token
- `REDIS_USERNAME`: For Redis 6.0+ ACL users
- `REDIS_TLS_ENABLED`: To enable TLS
- `REDIS_TLS_SKIP_VERIFY`: For development only

## TLS Configuration

For production ElastiCache with TLS:

```yaml
externalRedis:
  host: "your-cluster.cache.amazonaws.com"
  port: 6379
  tls:
    enabled: true
    skipVerify: false  # Always false in production
```

The services will automatically use `rediss://` protocol when TLS is enabled.

## Authentication

If your ElastiCache cluster uses AUTH:

1. **Store password in Kubernetes Secret**:

```bash
kubectl create secret generic elasticache-auth \
  --from-literal=password=your-auth-token \
  --namespace edge-mcp
```

2. **Reference in values**:

```yaml
externalRedis:
  password: "your-auth-token"  # Will be stored in Secret by Helm
```

## IAM Roles (IRSA)

For AWS IAM Roles for Service Accounts:

```yaml
serviceAccount:
  create: true
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/YourRole
```

## Monitoring

Monitor ElastiCache metrics in CloudWatch:
- **EngineCPUUtilization**: Should stay below 80%
- **DatabaseMemoryUsagePercentage**: Monitor for memory pressure
- **CurrConnections**: Track connection count
- **Evictions**: Should be zero in properly sized clusters

## Troubleshooting

### Connection Refused to [::]:6379

**Problem**: Service trying to connect to localhost IPv6.

**Solution**: Ensure `REDIS_ADDR` environment variable is properly set to ElastiCache endpoint in all services.

### TLS Handshake Failed

**Problem**: TLS certificate verification failing.

**Solution**:
1. Ensure CA certificates are present in Docker image (fixed in v0.0.10)
2. Set `tls.enabled: true` in values
3. For dev only: Set `tls.skipVerify: true`

### Authentication Failed

**Problem**: AUTH required but not configured.

**Solution**: Set `externalRedis.password` in values file with ElastiCache AUTH token.

### Network Timeout

**Problem**: Cannot reach ElastiCache from pods.

**Solution**:
1. Check security groups allow traffic from K8s nodes
2. Verify subnet routing if using private subnets
3. Check Network ACLs if configured

## Best Practices

1. **Use ElastiCache Replication**: Deploy multi-AZ with automatic failover
2. **Enable Encryption**: Use in-transit and at-rest encryption
3. **Set Resource Limits**: Configure proper memory limits to avoid evictions
4. **Monitor Metrics**: Set up CloudWatch alarms for key metrics
5. **Backup Strategy**: Enable automatic backups for ElastiCache
6. **Connection Pooling**: Services use connection pooling by default
7. **Separate Clusters**: Consider separate ElastiCache clusters for cache vs queue

## Migration from Local Redis

To migrate from local Redis to ElastiCache:

1. **Export data** (if needed):
```bash
kubectl exec -n edge-mcp redis-master-0 -- redis-cli BGSAVE
kubectl cp edge-mcp/redis-master-0:/data/dump.rdb ./redis-backup.rdb
```

2. **Update Helm values** to use ElastiCache
3. **Rolling update** services:
```bash
helm upgrade edge-mcp ./edge-mcp -f values.elasticache.yaml
helm upgrade worker ./worker -f values.elasticache.yaml
```

4. **Verify** connections and functionality

## Cost Optimization

- Use Reserved Instances for predictable workloads
- Right-size based on actual usage patterns
- Consider using cache.t4g (Graviton) instances for better price/performance
- Monitor and set appropriate eviction policies

## Security Considerations

- Always use TLS in production
- Enable AUTH tokens
- Use AWS PrivateLink for network isolation
- Implement least-privilege IAM policies
- Regular security group audits
- Enable ElastiCache encryption at rest

## Related Documentation

- [Kubernetes Deployment Guide](./kubernetes-deployment.md)
- [AWS Infrastructure Setup](./aws-setup.md)
- [Production Best Practices](./production-best-practices.md)