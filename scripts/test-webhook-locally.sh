#!/bin/bash
# Script to test webhook handling locally without GitHub delivery

set -e

# Load environment
if [ -f .env.test ]; then
    export $(grep -v '^#' .env.test | xargs)
fi

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}Local Webhook Testing${NC}"
echo "====================="

# Start local server
echo -e "${YELLOW}Starting local server...${NC}"
make dev-setup &
SERVER_PID=$!

# Wait for server to be ready
sleep 5

# Test webhook endpoint directly
echo -e "${YELLOW}Testing webhook signature validation...${NC}"

# Create test payload
PAYLOAD='{"action":"opened","pull_request":{"id":1,"title":"Test PR"}}'
TIMESTAMP=$(date +%s)

# Generate signature (this mimics what GitHub does)
if [ -n "$GITHUB_WEBHOOK_SECRET" ]; then
    SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$GITHUB_WEBHOOK_SECRET" | sed 's/^.* //')
    SIGNATURE="sha256=$SIGNATURE"
else
    echo "Warning: GITHUB_WEBHOOK_SECRET not set"
    SIGNATURE="sha256=dummy"
fi

# Send test webhook
echo -e "${YELLOW}Sending test webhook...${NC}"
curl -X POST http://localhost:8080/webhook/github \
    -H "Content-Type: application/json" \
    -H "X-GitHub-Event: pull_request" \
    -H "X-Hub-Signature-256: $SIGNATURE" \
    -H "X-GitHub-Delivery: test-$TIMESTAMP" \
    -d "$PAYLOAD" \
    -w "\nHTTP Status: %{http_code}\n"

# Cleanup
echo -e "\n${YELLOW}Stopping server...${NC}"
kill $SERVER_PID 2>/dev/null || true

echo -e "${GREEN}Done!${NC}"