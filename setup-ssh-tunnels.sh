#!/bin/bash
# Set up SSH tunnels through NAT instance for local testing

# Load environment variables
source .env

NAT_PUBLIC_IP=${NAT_INSTANCE_IP:-54.145.71.11}
KEY_FILE=${SSH_KEY_PATH:-~/.ssh/dev-bastion-key.pem}
RDS_HOST=${RDS_ENDPOINT:-developer-mesh-postgres.cshaq28kmnw8.us-east-1.rds.amazonaws.com}
REDIS_HOST=${ELASTICACHE_ENDPOINT:-developer-mesh-redis.qem3fz.0001.use1.cache.amazonaws.com}

echo "Setting up SSH tunnels through NAT instance..."
echo "NAT Instance IP: $NAT_PUBLIC_IP"
echo "Key File: $KEY_FILE"
echo ""
echo "Creating tunnels:"
echo "  - PostgreSQL: localhost:5432 -> $RDS_HOST:5432"
echo "  - Redis: localhost:6379 -> $REDIS_HOST:6379"
echo ""

# Check if key file exists
if [ ! -f "$KEY_FILE" ]; then
    echo "ERROR: SSH key file not found at: $KEY_FILE"
    echo "Please update SSH_KEY_PATH in .env"
    exit 1
fi

# Create SSH tunnels
ssh -i "$KEY_FILE" \
  -L 5432:$RDS_HOST:5432 \
  -L 6379:$REDIS_HOST:6379 \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o ConnectTimeout=10 \
  -o ServerAliveInterval=60 \
  -o ServerAliveCountMax=3 \
  -N \
  ec2-user@$NAT_PUBLIC_IP