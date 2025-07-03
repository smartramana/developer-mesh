# DevOps MCP Continuous Deployment Pipeline Plan

## Executive Summary
This plan outlines a production-ready CD pipeline for DevOps MCP based on our deployment experience. The pipeline will automate deployments to our EC2 instance while maintaining zero-downtime and rollback capabilities.

## Key Learnings from Manual Deployment

### Infrastructure Requirements
- **Instance Size**: t3.small minimum (2GB RAM) for 3 containers
- **Services**: MCP Server (8080), REST API (8081), Worker (background)
- **Dependencies**: RDS PostgreSQL, ElastiCache Redis (TLS), S3, SQS
- **Networking**: Nginx reverse proxy with Let's Encrypt SSL

### Configuration Requirements
- Separate config files for services (REST API needs dedicated config)
- Environment-specific .env files
- Redis TLS configuration is critical
- Database migrations must run before service startup

### Deployment Challenges Solved
1. Redis TLS connection configuration
2. REST API port conflicts (needs 8081, not 8080)
3. Worker SQS timeout bug (fixed in code)
4. Memory constraints on t2.micro

## CD Pipeline Architecture

### 1. Trigger Strategy
```yaml
Triggers:
  - Push to main branch (after CI passes)
  - Manual dispatch for hotfixes
  - Tag creation (v* pattern) for releases
```

### 2. Environment Strategy
```
Development → Staging → Production
    ↓           ↓          ↓
  Auto      Manual     Manual
 Deploy     Approval   Approval
```

### 3. Deployment Stages

#### Stage 1: Pre-Deployment Validation
- Verify CI has passed
- Check image availability in GHCR
- Validate AWS credentials
- Check target instance health
- Backup current configuration

#### Stage 2: Database Migrations
- Create migration job container
- Run migrations with rollback capability
- Verify migration success
- Create backup point

#### Stage 3: Blue-Green Deployment
- Deploy new containers with -blue suffix
- Health check new containers
- Switch Nginx upstream to blue containers
- Monitor for errors (5 min)
- Remove old containers

#### Stage 4: Post-Deployment
- Run smoke tests
- Update Route 53 health checks
- Send deployment notifications
- Clean up old images

### 4. Secrets Management

#### GitHub Secrets Required:
```
AWS_ACCESS_KEY_ID          # For AWS services
AWS_SECRET_ACCESS_KEY      # For AWS services
EC2_SSH_PRIVATE_KEY        # For EC2 access
DATABASE_PASSWORD          # RDS password
GITHUB_TOKEN              # For GHCR access
ADMIN_API_KEY             # API authentication
```

#### Parameter Store Integration:
```
/devops-mcp/prod/database_password
/devops-mcp/prod/redis_endpoint
/devops-mcp/prod/sqs_queue_url
```

### 5. Zero-Downtime Strategy

1. **Health Check Endpoints**:
   - MCP: https://mcp.dev-mesh.io/health
   - API: https://api.dev-mesh.io/health

2. **Rolling Update Process**:
   ```
   Current State: [MCP-v1] [API-v1] [Worker-v1]
                      ↓       ↓         ↓
   Deploy Blue:  [MCP-v2] [API-v2] [Worker-v2]
                      ↓       ↓         ↓
   Health Check:     ✓       ✓         ✓
                      ↓       ↓         ↓
   Switch Traffic: [Active] [Active] [Active]
                      ↓       ↓         ↓
   Remove v1:    [Removed][Removed][Removed]
   ```

3. **Nginx Configuration Update**:
   ```nginx
   upstream mcp_backend {
       server localhost:8080;  # Switch to 8082 for blue
   }
   ```

### 6. Rollback Strategy

#### Automatic Rollback Triggers:
- Health check failures (3 consecutive)
- Error rate > 10% (5 min window)
- Database migration failure
- Container crash loops

#### Rollback Process:
1. Switch Nginx back to previous version
2. Stop new containers
3. Restore previous containers
4. Revert database migrations (if applicable)
5. Alert team

### 7. Monitoring & Observability

#### Deployment Metrics:
- Deployment duration
- Success/failure rate
- Rollback frequency
- Service availability during deployment

#### Health Monitoring:
- Container health status
- Response time monitoring
- Error rate tracking
- Resource utilization

## Implementation Files

### 1. Main Deployment Workflow
`.github/workflows/deploy-production.yml`:
- Triggered on main branch push
- Calls reusable deployment workflow
- Production-specific validations

### 2. Reusable Deployment Workflow
`.github/workflows/deployment.yml`:
- Environment-agnostic deployment logic
- Parameterized for different environments
- Includes all deployment stages

### 3. Supporting Scripts
```
scripts/deployment/
├── 01-pre-deploy-checks.sh     # Validate environment
├── 02-backup-current.sh         # Backup current state
├── 03-run-migrations.sh         # Database migrations
├── 04-deploy-blue-green.sh      # Deploy new version
├── 05-health-checks.sh          # Verify deployment
├── 06-switch-traffic.sh         # Update Nginx
├── 07-cleanup.sh                # Remove old containers
└── 99-rollback.sh               # Emergency rollback
```

### 4. Configuration Templates
```
configs/
├── config.yaml                  # MCP server config
├── config.rest-api.yaml         # REST API config (port 8081)
├── config.worker.yaml           # Worker config
└── nginx/
    ├── mcp.conf.template        # Nginx template
    └── ssl-renew.sh            # Certificate renewal
```

## Security Considerations

### 1. SSH Key Management
- Use GitHub Secrets for private keys
- Rotate keys quarterly
- Restrict SSH to GitHub Actions IPs

### 2. Secret Rotation
- Database passwords via AWS Secrets Manager
- API keys with expiration
- Audit trail for secret access

### 3. Network Security
- Deployment only from GitHub Actions
- VPN for manual interventions
- Security group restrictions

## Testing Strategy

### 1. Pre-Deployment Tests
- Configuration validation
- Migration dry-run
- Security scanning

### 2. Post-Deployment Tests
```bash
# Smoke tests
curl https://mcp.dev-mesh.io/health
curl https://api.dev-mesh.io/health

# Integration tests
- Database connectivity
- Redis operations
- SQS message processing
- S3 operations
```

### 3. Canary Deployment
- 10% traffic to new version
- Monitor for 10 minutes
- Gradual rollout: 10% → 50% → 100%

## Failure Scenarios

### Scenario 1: Database Migration Failure
- Action: Automatic rollback
- Notification: Slack/Email alert
- Recovery: Manual intervention required

### Scenario 2: Container Crash
- Action: Restart with backoff
- Fallback: Previous version
- Alert: After 3 failures

### Scenario 3: Health Check Timeout
- Action: Extended health check (60s)
- Fallback: Keep current version
- Investigation: Check logs

## Success Criteria

### Deployment Success Metrics:
- All health checks passing
- Zero customer-facing errors
- Response time < 200ms
- All containers stable for 5 minutes

### Pipeline Success Metrics:
- < 10 minute deployment time
- > 99% success rate
- < 1% rollback rate
- Zero security incidents

## Next Steps

1. **Phase 1**: Implement basic deployment workflow
2. **Phase 2**: Add blue-green deployment
3. **Phase 3**: Implement canary deployments
4. **Phase 4**: Add advanced monitoring
5. **Phase 5**: Multi-region deployment

## Appendix: Environment Variables

### Required for Deployment:
```bash
# AWS Configuration
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=***
AWS_SECRET_ACCESS_KEY=***

# Database
DATABASE_HOST=devops-mcp-postgres.cshaq28kmnw8.us-east-1.rds.amazonaws.com
DATABASE_PORT=5432
DATABASE_USER=dbadmin
DATABASE_PASSWORD=***
DATABASE_NAME=devops_mcp
DATABASE_SSL_MODE=require

# Redis
REDIS_ADDR=master.devops-mcp-redis-encrypted.qem3fz.use1.cache.amazonaws.com:6379
REDIS_TLS_ENABLED=true

# Application
ENVIRONMENT=production
MCP_SERVER_URL=http://mcp-server:8080
PORT=8081  # For REST API
API_LISTEN_ADDRESS=:8081

# EC2
EC2_INSTANCE_IP=44.211.47.174
EC2_INSTANCE_ID=i-08a9a59532aad9879
```

## Conclusion

This CD pipeline plan incorporates all lessons learned from our manual deployment experience. It prioritizes reliability, security, and zero-downtime deployments while maintaining the flexibility to handle various failure scenarios.