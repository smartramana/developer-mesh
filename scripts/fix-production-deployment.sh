#!/bin/bash
set -e

# Script to fix production deployment issues
# This connects to EC2 via SSM and resolves the config file issues

echo "=== Production Deployment Fix Script ==="
echo "This script will help fix the production deployment issues"
echo

# Check if AWS CLI is configured
if ! aws sts get-caller-identity &>/dev/null; then
    echo "ERROR: AWS CLI is not configured or credentials are expired"
    echo "Please run: aws configure"
    exit 1
fi

# Configuration
AWS_REGION="${AWS_REGION:-us-east-1}"
EC2_INSTANCE_IP="${EC2_INSTANCE_IP}"

if [ -z "$EC2_INSTANCE_IP" ]; then
    echo "ERROR: EC2_INSTANCE_IP environment variable is not set"
    echo "Please set it to your EC2 instance's public IP"
    exit 1
fi

# Get instance ID from IP
echo "Finding EC2 instance ID for IP: $EC2_INSTANCE_IP"
INSTANCE_ID=$(aws ec2 describe-instances \
    --filters "Name=ip-address,Values=$EC2_INSTANCE_IP" \
    --query "Reservations[0].Instances[0].InstanceId" \
    --output text \
    --region "$AWS_REGION")

if [ "$INSTANCE_ID" = "None" ] || [ -z "$INSTANCE_ID" ]; then
    echo "ERROR: Could not find instance with IP $EC2_INSTANCE_IP"
    exit 1
fi

echo "Found instance: $INSTANCE_ID"
echo

# Function to run commands on EC2
run_ssm_command() {
    local description="$1"
    shift
    local commands=("$@")
    
    echo ">>> $description"
    
    # Create the command
    COMMAND_ID=$(aws ssm send-command \
        --instance-ids "$INSTANCE_ID" \
        --document-name "AWS-RunShellScript" \
        --parameters "commands=$*" \
        --query "Command.CommandId" \
        --output text \
        --region "$AWS_REGION")
    
    # Wait for command to complete
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
    echo
}

# Step 1: Check current container status
echo "=== Step 1: Checking current container status ==="
run_ssm_command "Checking Docker containers" \
    "cd /home/ec2-user/developer-mesh && docker ps -a"

# Step 2: Check container logs for errors
echo "=== Step 2: Checking container logs for errors ==="
run_ssm_command "MCP Server logs" \
    "cd /home/ec2-user/developer-mesh && docker logs mcp-server --tail 20 2>&1 || echo 'No logs available'"

# Step 3: Check if config files exist
echo "=== Step 3: Checking config files ==="
run_ssm_command "Listing config files" \
    "cd /home/ec2-user/developer-mesh && ls -la configs/ 2>&1 || echo 'configs directory not found'"

# Step 4: Download missing config files
echo "=== Step 4: Ensuring all config files are present ==="
run_ssm_command "Creating configs directory and downloading files" \
    "cd /home/ec2-user/developer-mesh && \
    mkdir -p configs && \
    echo 'Downloading config files from GitHub...' && \
    curl -sL https://raw.githubusercontent.com/S-Corkum/developer-mesh/main/configs/config.base.yaml -o configs/config.base.yaml && \
    curl -sL https://raw.githubusercontent.com/S-Corkum/developer-mesh/main/configs/config.production.yaml -o configs/config.production.yaml && \
    curl -sL https://raw.githubusercontent.com/S-Corkum/developer-mesh/main/configs/config.rest-api.yaml -o configs/config.rest-api.yaml && \
    curl -sL https://raw.githubusercontent.com/S-Corkum/developer-mesh/main/configs/auth.production.yaml -o configs/auth.production.yaml && \
    echo 'Verifying downloaded files...' && \
    for file in config.base.yaml config.production.yaml config.rest-api.yaml auth.production.yaml; do \
        if [ -f \"configs/\$file\" ] && [ -s \"configs/\$file\" ]; then \
            echo \"✓ configs/\$file (\$(wc -c < \"configs/\$file\") bytes)\"; \
        else \
            echo \"✗ configs/\$file is missing or empty\"; \
        fi; \
    done"

# Step 5: Check docker-compose.yml
echo "=== Step 5: Checking docker-compose.yml ==="
run_ssm_command "Checking docker-compose file" \
    "cd /home/ec2-user/developer-mesh && \
    if [ -f docker-compose.yml ]; then \
        echo 'docker-compose.yml exists'; \
    else \
        echo 'docker-compose.yml missing, downloading...'; \
        curl -sL https://raw.githubusercontent.com/S-Corkum/developer-mesh/main/docker-compose.production.yml -o docker-compose.yml; \
    fi"

# Step 6: Check .env file
echo "=== Step 6: Checking .env file ==="
run_ssm_command "Verifying .env file exists" \
    "cd /home/ec2-user/developer-mesh && \
    if [ -f .env ]; then \
        echo '.env file exists'; \
        echo 'Checking key environment variables:'; \
        grep -E '^(MCP_CONFIG_FILE|CONFIG_PATH|IMAGE_TAG)=' .env | sed 's/=.*$/=<redacted>/' || true; \
    else \
        echo 'ERROR: .env file is missing!'; \
    fi"

# Step 7: Stop all containers
echo "=== Step 7: Stopping all containers ==="
run_ssm_command "Stopping containers" \
    "cd /home/ec2-user/developer-mesh && docker-compose down"

# Step 8: Update docker-compose.yml to use correct image tags
echo "=== Step 8: Fixing docker-compose.yml image tags ==="
run_ssm_command "Updating image tags" \
    "cd /home/ec2-user/developer-mesh && \
    if [ -f docker-compose.yml ]; then \
        # First, try to get the latest SHA from git
        LATEST_SHA=\$(git rev-parse HEAD 2>/dev/null | cut -c1-7 || echo '') ; \
        if [ -z \"\$LATEST_SHA\" ]; then \
            echo 'Using latest tag (git not available)'; \
            sed -i 's/:main-[a-f0-9]\\{7\\}/:latest/g' docker-compose.yml; \
        else \
            echo \"Using SHA tag: main-\$LATEST_SHA\"; \
            sed -i \"s/:main-[a-f0-9]\\{7\\}/:main-\$LATEST_SHA/g\" docker-compose.yml; \
            # Also update IMAGE_TAG in .env
            if grep -q '^IMAGE_TAG=' .env; then \
                sed -i \"s/^IMAGE_TAG=.*/IMAGE_TAG=main-\$LATEST_SHA/\" .env; \
            else \
                echo \"IMAGE_TAG=main-\$LATEST_SHA\" >> .env; \
            fi; \
        fi; \
    fi"

# Step 9: Pull latest images
echo "=== Step 9: Pulling latest images ==="
run_ssm_command "Pulling Docker images" \
    "cd /home/ec2-user/developer-mesh && \
    docker-compose pull"

# Step 10: Start services
echo "=== Step 10: Starting services ==="
run_ssm_command "Starting containers" \
    "cd /home/ec2-user/developer-mesh && \
    docker-compose up -d"

# Step 11: Wait for services to stabilize
echo "=== Step 11: Waiting for services to stabilize ==="
echo "Waiting 30 seconds for containers to start..."
sleep 30

# Step 12: Check final status
echo "=== Step 12: Checking final status ==="
run_ssm_command "Final container status" \
    "cd /home/ec2-user/developer-mesh && \
    docker ps && \
    echo && \
    echo 'Container health status:' && \
    for container in mcp-server rest-api worker; do \
        STATUS=\$(docker inspect \$container --format='{{.State.Status}}' 2>/dev/null || echo 'not found'); \
        HEALTH=\$(docker inspect \$container --format='{{.State.Health.Status}}' 2>/dev/null || echo 'no health check'); \
        echo \"\$container: status=\$STATUS, health=\$HEALTH\"; \
    done"

# Step 13: Check logs again if any containers are unhealthy
echo "=== Step 13: Checking logs for any issues ==="
run_ssm_command "Checking recent logs" \
    "cd /home/ec2-user/developer-mesh && \
    for container in mcp-server rest-api worker; do \
        if docker ps | grep -q \"\$container\"; then \
            echo \"--- \$container logs (last 10 lines) ---\"; \
            docker logs \$container --tail 10 2>&1 || true; \
            echo; \
        fi; \
    done"

# Step 14: Test endpoints
echo "=== Step 14: Testing endpoints ==="
echo "Testing REST API health endpoint..."
curl -s -o /dev/null -w "REST API: HTTP %{http_code}\n" https://api.dev-mesh.io/health || echo "REST API: Failed to connect"

echo "Testing MCP Server health endpoint..."
curl -s -o /dev/null -w "MCP Server: HTTP %{http_code}\n" https://mcp.dev-mesh.io/health || echo "MCP Server: Failed to connect"

echo
echo "=== Deployment fix complete ==="
echo "If services are still failing, check the logs above for specific errors."
echo "Common issues:"
echo "  - Missing environment variables in .env file"
echo "  - Database connection issues"
echo "  - Redis/ElastiCache connection issues"
echo "  - Incorrect image tags"