#!/bin/bash
set -e

# Script to fix config file loading issues in production
# The main issue is that services expect configs/config.yaml but we're providing config.production.yaml

echo "=== Config File Loading Fix ==="
echo

# Check AWS CLI
if ! aws sts get-caller-identity &>/dev/null; then
    echo "ERROR: AWS CLI is not configured"
    exit 1
fi

# Configuration
AWS_REGION="${AWS_REGION:-us-east-1}"
EC2_INSTANCE_IP="${EC2_INSTANCE_IP}"

if [ -z "$EC2_INSTANCE_IP" ]; then
    echo "ERROR: EC2_INSTANCE_IP is not set"
    exit 1
fi

# Get instance ID
INSTANCE_ID=$(aws ec2 describe-instances \
    --filters "Name=ip-address,Values=$EC2_INSTANCE_IP" \
    --query "Reservations[0].Instances[0].InstanceId" \
    --output text \
    --region "$AWS_REGION")

echo "Instance ID: $INSTANCE_ID"

# Fix 1: Update docker-compose.yml to remove the symlink command and set proper env vars
echo "=== Fixing docker-compose configuration ==="

# Create an updated docker-compose file that properly sets MCP_CONFIG_FILE
cat > /tmp/docker-compose-fix.yml << 'EOF'
version: '3.8'

services:
  mcp-server:
    image: ghcr.io/s-corkum/devops-mcp-mcp-server:${IMAGE_TAG:-latest}
    container_name: mcp-server
    restart: unless-stopped
    mem_limit: 200m
    mem_reservation: 150m
    ports:
      - "8080:8080"
    env_file:
      - .env
    environment:
      - PORT=8080
      - MCP_CONFIG_FILE=/app/configs/config.production.yaml
      - CONFIG_PATH=/app/configs/config.production.yaml
    volumes:
      - ./configs/config.base.yaml:/app/configs/config.base.yaml:ro
      - ./configs/config.production.yaml:/app/configs/config.production.yaml:ro
      - ./configs/auth.production.yaml:/app/configs/auth.production.yaml:ro
      - ./logs:/app/logs
    healthcheck:
      test: ["CMD", "/app/mcp-server", "-health-check"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    networks:
      - mcp-network

  rest-api:
    image: ghcr.io/s-corkum/devops-mcp-rest-api:${IMAGE_TAG:-latest}
    container_name: rest-api
    restart: unless-stopped
    mem_limit: 200m
    mem_reservation: 150m
    ports:
      - "8081:8081"
    env_file:
      - .env
    environment:
      - MCP_SERVER_URL=http://mcp-server:8080
      - PORT=8081
      - API_LISTEN_ADDRESS=:8081
      - MCP_CONFIG_FILE=/app/configs/config.production.yaml
      - CONFIG_PATH=/app/configs/config.production.yaml
    volumes:
      - ./configs/config.base.yaml:/app/configs/config.base.yaml:ro
      - ./configs/config.production.yaml:/app/configs/config.production.yaml:ro
      - ./configs/config.rest-api.yaml:/app/configs/config.rest-api.yaml:ro
      - ./configs/auth.production.yaml:/app/configs/auth.production.yaml:ro
      - ./logs:/app/logs
    depends_on:
      mcp-server:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8081/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    networks:
      - mcp-network

  worker:
    image: ghcr.io/s-corkum/devops-mcp-worker:${IMAGE_TAG:-latest}
    container_name: worker
    restart: unless-stopped
    mem_limit: 200m
    mem_reservation: 150m
    env_file:
      - .env
    environment:
      - MCP_CONFIG_FILE=/app/configs/config.production.yaml
      - CONFIG_PATH=/app/configs/config.production.yaml
    volumes:
      - ./configs/config.base.yaml:/app/configs/config.base.yaml:ro
      - ./configs/config.production.yaml:/app/configs/config.production.yaml:ro
      - ./configs/auth.production.yaml:/app/configs/auth.production.yaml:ro
      - ./logs:/app/logs
    depends_on:
      mcp-server:
        condition: service_healthy
    networks:
      - mcp-network

networks:
  mcp-network:
    driver: bridge
EOF

# Upload the fixed docker-compose file
echo "Uploading fixed docker-compose.yml to EC2..."
aws ssm send-command \
    --instance-ids "$INSTANCE_ID" \
    --document-name "AWS-RunShellScript" \
    --parameters "commands=[
        'cd /home/ec2-user/devops-mcp',
        'cp docker-compose.yml docker-compose.yml.backup',
        'cat > docker-compose.yml << '\''COMPOSE_END'\''',
        '$(cat /tmp/docker-compose-fix.yml)',
        'COMPOSE_END'
    ]" \
    --query "Command.CommandId" \
    --output text \
    --region "$AWS_REGION" > /tmp/command-id.txt

COMMAND_ID=$(cat /tmp/command-id.txt)
aws ssm wait command-executed \
    --command-id "$COMMAND_ID" \
    --instance-id "$INSTANCE_ID" \
    --region "$AWS_REGION" 2>/dev/null || true

# Fix 2: Create a fallback config.yaml that imports config.production.yaml
echo "=== Creating fallback config.yaml ==="

aws ssm send-command \
    --instance-ids "$INSTANCE_ID" \
    --document-name "AWS-RunShellScript" \
    --parameters 'commands=[
        "cd /home/ec2-user/devops-mcp",
        "echo \"Creating fallback config.yaml that imports production config...\"",
        "cat > configs/config.yaml << '\''EOF'\''",
        "# Fallback config that imports production config",
        "# This ensures services can find config even if MCP_CONFIG_FILE is not set",
        "",
        "# Import all settings from production config",
        "import:",
        "  - config.production.yaml",
        "EOF",
        "echo \"Fallback config created\""
    ]' \
    --query "Command.CommandId" \
    --output text \
    --region "$AWS_REGION" > /tmp/command-id.txt

COMMAND_ID=$(cat /tmp/command-id.txt)
aws ssm wait command-executed \
    --command-id "$COMMAND_ID" \
    --instance-id "$INSTANCE_ID" \
    --region "$AWS_REGION" 2>/dev/null || true

# Fix 3: Ensure .env has correct config paths
echo "=== Updating .env file with correct paths ==="

aws ssm send-command \
    --instance-ids "$INSTANCE_ID" \
    --document-name "AWS-RunShellScript" \
    --parameters 'commands=[
        "cd /home/ec2-user/devops-mcp",
        "echo \"Updating .env file...\"",
        "# Remove old config path settings",
        "grep -v \"^MCP_CONFIG_FILE=\" .env > .env.tmp || true",
        "grep -v \"^CONFIG_PATH=\" .env.tmp > .env.new || true",
        "# Add correct config paths",
        "echo \"MCP_CONFIG_FILE=/app/configs/config.production.yaml\" >> .env.new",
        "echo \"CONFIG_PATH=/app/configs/config.production.yaml\" >> .env.new",
        "# Replace old .env",
        "mv .env .env.backup",
        "mv .env.new .env",
        "chmod 600 .env",
        "echo \"Environment file updated\""
    ]' \
    --query "Command.CommandId" \
    --output text \
    --region "$AWS_REGION" > /tmp/command-id.txt

COMMAND_ID=$(cat /tmp/command-id.txt)
aws ssm wait command-executed \
    --command-id "$COMMAND_ID" \
    --instance-id "$INSTANCE_ID" \
    --region "$AWS_REGION" 2>/dev/null || true

# Fix 4: Restart services with new configuration
echo "=== Restarting services ==="

aws ssm send-command \
    --instance-ids "$INSTANCE_ID" \
    --document-name "AWS-RunShellScript" \
    --parameters 'commands=[
        "cd /home/ec2-user/devops-mcp",
        "echo \"Stopping services...\"",
        "docker-compose down",
        "echo \"Starting services with fixed configuration...\"",
        "docker-compose up -d",
        "echo \"Waiting for services to start...\"",
        "sleep 20",
        "echo \"Checking service status...\"",
        "docker ps",
        "echo",
        "echo \"Checking for restart loops...\"",
        "for container in mcp-server rest-api worker; do",
        "    RESTARTS=$(docker inspect $container --format='\''{{.RestartCount}}'\'' 2>/dev/null || echo 0)",
        "    echo \"$container restart count: $RESTARTS\"",
        "done"
    ]' \
    --query "Command.CommandId" \
    --output text \
    --region "$AWS_REGION" > /tmp/command-id.txt

COMMAND_ID=$(cat /tmp/command-id.txt)
aws ssm wait command-executed \
    --command-id "$COMMAND_ID" \
    --instance-id "$INSTANCE_ID" \
    --region "$AWS_REGION" 2>/dev/null || true

# Get the output
OUTPUT=$(aws ssm get-command-invocation \
    --command-id "$COMMAND_ID" \
    --instance-id "$INSTANCE_ID" \
    --query "StandardOutputContent" \
    --output text \
    --region "$AWS_REGION")

echo "$OUTPUT"

# Fix 5: Check logs for any remaining issues
echo
echo "=== Checking logs for issues ==="

aws ssm send-command \
    --instance-ids "$INSTANCE_ID" \
    --document-name "AWS-RunShellScript" \
    --parameters 'commands=[
        "cd /home/ec2-user/devops-mcp",
        "for container in mcp-server rest-api worker; do",
        "    echo \"--- $container logs (last 5 lines) ---\"",
        "    docker logs $container --tail 5 2>&1 || echo \"No logs available\"",
        "    echo",
        "done"
    ]' \
    --query "Command.CommandId" \
    --output text \
    --region "$AWS_REGION" > /tmp/command-id.txt

COMMAND_ID=$(cat /tmp/command-id.txt)
aws ssm wait command-executed \
    --command-id "$COMMAND_ID" \
    --instance-id "$INSTANCE_ID" \
    --region "$AWS_REGION" 2>/dev/null || true

OUTPUT=$(aws ssm get-command-invocation \
    --command-id "$COMMAND_ID" \
    --instance-id "$INSTANCE_ID" \
    --query "StandardOutputContent" \
    --output text \
    --region "$AWS_REGION")

echo "$OUTPUT"

# Clean up
rm -f /tmp/command-id.txt /tmp/docker-compose-fix.yml

echo
echo "=== Config loading fix complete ==="
echo "Services should now be running without restart loops."
echo "If issues persist, check:"
echo "  1. Database connectivity"
echo "  2. Redis/ElastiCache connectivity" 
echo "  3. AWS credentials in .env file"