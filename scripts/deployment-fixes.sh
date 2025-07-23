#!/bin/bash
# Deployment fixes for consistent production deployments

set -e

echo "==================================="
echo "Developer Mesh Deployment Fixes"
echo "==================================="

# Function to generate secure password without special characters
generate_secure_password() {
    openssl rand -base64 32 | tr -d "=+/" | cut -c1-32
}

# Function to generate JWT secret
generate_jwt_secret() {
    openssl rand -base64 64 | tr -d "\n"
}

# Function to create complete .env file
create_production_env() {
    local db_password="${1:-$(generate_secure_password)}"
    local admin_api_key="${2:-$(generate_secure_password)}"
    local jwt_secret="${3:-$(generate_jwt_secret)}"
    
    cat << EOF
# Generated on $(date)
# Environment
ENVIRONMENT=production

# Database Configuration
DATABASE_HOST=developer-mesh-postgres.cshaq28kmnw8.us-east-1.rds.amazonaws.com
DATABASE_PORT=5432
DATABASE_NAME=devops_mcp
DATABASE_USER=dbadmin
DATABASE_PASSWORD=${db_password}
DATABASE_SSL_MODE=require

# Cache/Redis Configuration
REDIS_ADDR=master.developer-mesh-redis-encrypted.qem3fz.use1.cache.amazonaws.com:6379
REDIS_TLS_ENABLED=true
CACHE_TYPE=redis
CACHE_TLS_ENABLED=true
CACHE_DIAL_TIMEOUT=10s
CACHE_READ_TIMEOUT=5s
CACHE_WRITE_TIMEOUT=5s
ELASTICACHE_ENDPOINTS=\${REDIS_ADDR}
ELASTICACHE_AUTH_TOKEN=

# API Configuration
API_LISTEN_ADDRESS=:8081
PORT=8080
MCP_SERVER_URL=http://mcp-server:8080
ADMIN_API_KEY=${admin_api_key}
JWT_SECRET=${jwt_secret}

# CORS and Security
CORS_ALLOWED_ORIGINS=https://mcp.dev-mesh.io,https://api.dev-mesh.io
TLS_ENABLED=false
USE_IAM_AUTH=false
REQUIRE_AUTH=true
REQUIRE_MFA=false
REQUEST_SIGNING_ENABLED=false
IP_WHITELIST=

# AWS Configuration
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=\${AWS_ACCESS_KEY_ID}
AWS_SECRET_ACCESS_KEY=\${AWS_SECRET_ACCESS_KEY}
S3_BUCKET=sean-mcp-dev-contexts
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test

# Monitoring and Tracing
LOG_LEVEL=info
TRACING_ENABLED=false
HA_ENABLED=false
DATA_RESIDENCY_ENABLED=false

# Feature Flags
OPENAI_ENABLED=false
GOOGLE_AI_ENABLED=false
BEDROCK_ENABLED=true
USE_READ_REPLICAS=false

# Worker Configuration
WORKER_QUEUE_TYPE=sqs
WORKER_CONCURRENCY=10

# Cost Limits
BEDROCK_SESSION_LIMIT=0.10
GLOBAL_COST_LIMIT=10.0

# Secrets Provider
SECRETS_PROVIDER=env

# WebSocket Configuration
WS_ALLOWED_ORIGINS=["https://mcp.dev-mesh.io"]
WS_MAX_CONNECTIONS=50000

# Database Pool Settings
DB_MAX_OPEN_CONNS=100
DB_MAX_IDLE_CONNS=25

# Redis Pool Settings
REDIS_POOL_SIZE=100
REDIS_MIN_IDLE=20

# GitHub Configuration
GITHUB_TOKEN=\${GITHUB_TOKEN}
GITHUB_WEBHOOK_SECRET=dev-webhook-secret
GITHUB_API_URL=https://api.github.com

# Additional Services (Optional)
SONARQUBE_URL=
SONARQUBE_TOKEN=
S3_KMS_KEY_ID=
METRICS_AUTH_TOKEN=$(openssl rand -hex 16)
ALLOWED_REGIONS=us-east-1
READ_REPLICA_HOSTS=
VPC_ENDPOINT_IDS=
BEDROCK_VPC_ENDPOINT=
BEDROCK_ROLE_ARN=
OPENAI_API_KEY=
GOOGLE_AI_API_KEY=
EOF
}

# Function to validate required files
validate_deployment_files() {
    local errors=0
    
    echo "Validating deployment files..."
    
    # Check required config files
    for file in "configs/config.base.yaml" "configs/config.production.yaml" "configs/auth.production.yaml"; do
        if [ ! -f "$file" ]; then
            echo "❌ Missing required file: $file"
            ((errors++))
        else
            echo "✓ Found: $file"
        fi
    done
    
    # Check docker-compose
    if [ ! -f "docker-compose.production.yml" ]; then
        echo "❌ Missing docker-compose.production.yml"
        ((errors++))
    else
        echo "✓ Found: docker-compose.production.yml"
    fi
    
    return $errors
}

# Function to update deployment workflow
update_deployment_workflow() {
    echo "Key changes needed in .github/workflows/deploy-production-v2.yml:"
    echo ""
    echo "1. Add IMAGE_TAG environment variable when running docker-compose:"
    echo "   export IMAGE_TAG=main-\${SHORT_SHA}"
    echo ""
    echo "2. Deploy all config files:"
    echo "   - configs/config.base.yaml"
    echo "   - configs/config.production.yaml" 
    echo "   - configs/auth.production.yaml"
    echo ""
    echo "3. Generate complete .env file with all required variables"
    echo ""
    echo "4. Remove git operations, use direct downloads from GitHub"
    echo ""
    echo "5. Add deployment verification step"
}

# Main execution
case "${1:-help}" in
    generate-env)
        echo "Generating production .env file..."
        create_production_env "$2" "$3" "$4"
        ;;
    
    validate)
        validate_deployment_files
        ;;
    
    fix-workflow)
        update_deployment_workflow
        ;;
    
    *)
        echo "Usage: $0 {generate-env|validate|fix-workflow}"
        echo ""
        echo "Commands:"
        echo "  generate-env [db_pass] [api_key] [jwt_secret]  Generate complete .env file"
        echo "  validate                                        Validate deployment files"
        echo "  fix-workflow                                    Show workflow fixes needed"
        ;;
esac