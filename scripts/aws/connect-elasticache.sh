#!/bin/bash
# SSH tunnel to access ElastiCache through bastion host

# Source environment variables
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

# Use environment variables with secure defaults
BASTION_IP="${BASTION_HOST_IP:-}"
ELASTICACHE_ENDPOINT="${ELASTICACHE_ENDPOINT:-${REDIS_HOST:-}}"
LOCAL_PORT="${REDIS_TUNNEL_PORT:-6379}"
KEY_FILE="${BASTION_KEY_FILE:-$HOME/.ssh/dev-bastion-key.pem}"

# Validate required variables
if [ -z "$BASTION_IP" ]; then
    echo "Error: BASTION_HOST_IP not set in environment"
    echo "Please add BASTION_HOST_IP to your .env file"
    exit 1
fi

if [ -z "$ELASTICACHE_ENDPOINT" ]; then
    echo "Error: ELASTICACHE_ENDPOINT or REDIS_HOST not set"
    echo "Please add ELASTICACHE_ENDPOINT to your .env file"
    exit 1
fi

if [ ! -f "$KEY_FILE" ]; then
    echo "Error: SSH key file not found: $KEY_FILE"
    echo "Please ensure BASTION_KEY_FILE points to your SSH key"
    exit 1
fi

echo "Setting up SSH tunnel to ElastiCache..."
echo "Local port $LOCAL_PORT will forward to $ELASTICACHE_ENDPOINT:6379"
echo ""
echo "Once connected, you can access Redis at: localhost:$LOCAL_PORT"
echo "Example: redis-cli -h localhost -p $LOCAL_PORT"
echo ""
echo "Press Ctrl+C to close the tunnel"
echo ""

# Create SSH tunnel
ssh -i "$KEY_FILE" \
    -L "$LOCAL_PORT:$ELASTICACHE_ENDPOINT:6379" \
    -N \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    ec2-user@"$BASTION_IP"