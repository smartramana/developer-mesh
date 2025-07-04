#!/bin/bash
# Fix missing environment variables in production

set -e

# Generate a secure JWT secret if not provided
if [ -z "$JWT_SECRET" ]; then
    JWT_SECRET=$(openssl rand -base64 32)
fi

# Add missing critical environment variables
cat << EOF >> .env

# Missing critical env vars - added $(date)
JWT_SECRET=${JWT_SECRET}
CORS_ALLOWED_ORIGINS=https://mcp.dev-mesh.io,https://api.dev-mesh.io
TLS_ENABLED=false
USE_IAM_AUTH=false
TRACING_ENABLED=false
HA_ENABLED=false
DATA_RESIDENCY_ENABLED=false
REQUEST_SIGNING_ENABLED=false
SECRETS_PROVIDER=env
IP_WHITELIST=
REQUIRE_MFA=false
USE_READ_REPLICAS=false
READ_REPLICA_HOSTS=
METRICS_AUTH_TOKEN=$(openssl rand -hex 16)

# ElastiCache configuration
ELASTICACHE_ENDPOINTS=${REDIS_ADDR}
ELASTICACHE_AUTH_TOKEN=

# Optional providers (disabled)
OPENAI_ENABLED=false
OPENAI_API_KEY=
GOOGLE_AI_ENABLED=false
GOOGLE_AI_API_KEY=
BEDROCK_ENABLED=true

# WebSocket origins
WS_ALLOWED_ORIGINS=["https://mcp.dev-mesh.io"]

# API configuration
API_LISTEN_ADDRESS=:8081
PORT=8080

# SonarQube (optional)
SONARQUBE_URL=
SONARQUBE_TOKEN=

# GitHub webhook secret
GITHUB_WEBHOOK_SECRET=dev-webhook-secret

# S3 KMS (optional)
S3_KMS_KEY_ID=

# Worker configuration
WORKER_QUEUE_TYPE=sqs
WORKER_CONCURRENCY=10

# Monitoring
LOG_LEVEL=info
ALLOWED_REGIONS=us-east-1

# Cost limits
BEDROCK_SESSION_LIMIT=0.10
GLOBAL_COST_LIMIT=10.0
EOF

echo "Environment variables fixed!"