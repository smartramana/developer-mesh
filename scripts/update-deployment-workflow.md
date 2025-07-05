# Deployment Workflow Updates

## Critical Changes Needed in `.github/workflows/deploy-production-v2.yml`

### 1. Fix Docker Image Tags
Add IMAGE_TAG export before docker-compose commands:

```yaml
- name: Deploy Services with Blue-Green Strategy
  run: |
    COMMAND_ID=$(aws ssm send-command \
      --instance-ids "${{ steps.get-instance.outputs.instance_id }}" \
      --document-name "AWS-RunShellScript" \
      --parameters 'commands=[
        "cd /home/ec2-user/devops-mcp",
        "export IMAGE_TAG=main-${{ steps.short-sha.outputs.short_sha }}",
        "docker-compose -f docker-compose.production.yml pull",
        "docker-compose -f docker-compose.production.yml up -d"
      ]'
```

### 2. Add Missing Environment Variables
Update the "Create/Update Environment File" step to include:

```bash
# JWT and Security
JWT_SECRET=$(openssl rand -base64 64 | tr -d "\n")
CORS_ALLOWED_ORIGINS=https://mcp.dev-mesh.io,https://api.dev-mesh.io
TLS_ENABLED=false
USE_IAM_AUTH=false
REQUIRE_AUTH=true
REQUIRE_MFA=false
REQUEST_SIGNING_ENABLED=false
IP_WHITELIST=

# Cache Configuration
CACHE_TYPE=redis
CACHE_TLS_ENABLED=true
CACHE_DIAL_TIMEOUT=10s
CACHE_READ_TIMEOUT=5s
CACHE_WRITE_TIMEOUT=5s
ELASTICACHE_ENDPOINTS=${{ secrets.REDIS_ENDPOINT }}
ELASTICACHE_AUTH_TOKEN=

# Database Pool Settings
DB_MAX_OPEN_CONNS=100
DB_MAX_IDLE_CONNS=25

# Redis Pool Settings
REDIS_POOL_SIZE=100
REDIS_MIN_IDLE=20

# Monitoring
TRACING_ENABLED=false
HA_ENABLED=false
DATA_RESIDENCY_ENABLED=false
METRICS_AUTH_TOKEN=$(openssl rand -hex 16)

# Feature Flags
OPENAI_ENABLED=false
GOOGLE_AI_ENABLED=false
USE_READ_REPLICAS=false

# WebSocket Configuration
WS_ALLOWED_ORIGINS=["https://mcp.dev-mesh.io"]
WS_MAX_CONNECTIONS=50000

# Additional Settings
SECRETS_PROVIDER=env
ALLOWED_REGIONS=us-east-1
GITHUB_WEBHOOK_SECRET=${{ secrets.GITHUB_WEBHOOK_SECRET }}
GITHUB_API_URL=https://api.github.com
```

### 3. Deploy All Required Config Files
Add a step to deploy all config files:

```yaml
- name: Deploy Configuration Files
  run: |
    COMMAND_ID=$(aws ssm send-command \
      --instance-ids "${{ steps.get-instance.outputs.instance_id }}" \
      --document-name "AWS-RunShellScript" \
      --parameters 'commands=[
        "cd /home/ec2-user/devops-mcp",
        "mkdir -p configs",
        "curl -s https://raw.githubusercontent.com/S-Corkum/devops-mcp/${{ github.sha }}/configs/config.base.yaml > configs/config.base.yaml",
        "curl -s https://raw.githubusercontent.com/S-Corkum/devops-mcp/${{ github.sha }}/configs/config.production.yaml > configs/config.production.yaml",
        "curl -s https://raw.githubusercontent.com/S-Corkum/devops-mcp/${{ github.sha }}/configs/auth.production.yaml > configs/auth.production.yaml",
        "curl -s https://raw.githubusercontent.com/S-Corkum/devops-mcp/${{ github.sha }}/docker-compose.production.yml > docker-compose.production.yml",
        "chown -R ec2-user:ec2-user configs",
        "echo \"Configuration files deployed successfully\""
      ]'
```

### 4. Fix Database Password Generation
Ensure passwords don't contain special characters:

```yaml
DATABASE_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
```

### 5. Add Deployment Verification
Add a final step to verify deployment:

```yaml
- name: Verify Deployment
  run: |
    COMMAND_ID=$(aws ssm send-command \
      --instance-ids "${{ steps.get-instance.outputs.instance_id }}" \
      --document-name "AWS-RunShellScript" \
      --parameters 'commands=[
        "cd /home/ec2-user/devops-mcp",
        "echo \"Checking service health...\"",
        "sleep 30",
        "curl -f http://localhost:8080/health || exit 1",
        "curl -f http://localhost:8081/health || exit 1",
        "docker-compose -f docker-compose.production.yml ps",
        "echo \"Deployment verification successful!\""
      ]'
```

### 6. Remove Git Operations
Replace all git clone/checkout operations with direct downloads from GitHub.

### 7. Fix Health Check Commands
Update docker-compose.production.yml healthcheck to use wget or curl instead of binary:

```yaml
healthcheck:
  test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
```

## GitHub Secrets Required
Ensure these secrets are set:
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `DATABASE_PASSWORD` (or generate dynamically)
- `ADMIN_API_KEY`
- `REDIS_ENDPOINT`
- `S3_BUCKET`
- `SQS_QUEUE_URL`
- `GITHUB_TOKEN`
- `GITHUB_WEBHOOK_SECRET`