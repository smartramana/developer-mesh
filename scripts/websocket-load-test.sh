#!/bin/bash

# WebSocket Load Test Script
# This script performs load testing on the WebSocket server

set -e

# Configuration
MCP_SERVER_URL=${MCP_SERVER_URL:-"http://localhost:8080"}
TEST_API_KEY=${TEST_API_KEY:-"test-key-admin"}
NUM_CONNECTIONS=${NUM_CONNECTIONS:-10}
MESSAGES_PER_CONNECTION=${MESSAGES_PER_CONNECTION:-100}
DURATION_SECONDS=${DURATION_SECONDS:-60}

# Convert HTTP URL to WebSocket URL
WS_URL=$(echo $MCP_SERVER_URL | sed 's/http:/ws:/g' | sed 's/https:/wss:/g')
WS_URL="${WS_URL}/ws"

echo "WebSocket Load Test Configuration:"
echo "  WebSocket URL: $WS_URL"
echo "  Connections: $NUM_CONNECTIONS"
echo "  Messages per connection: $MESSAGES_PER_CONNECTION"
echo "  Duration: ${DURATION_SECONDS}s"
echo ""

# Check if k6 is installed
if command -v k6 &> /dev/null; then
    echo "Running k6 load test..."
    export MCP_SERVER_URL
    export TEST_API_KEY
    k6 run scripts/websocket-load-test.js
else
    echo "k6 not found. Running basic load test..."
    
    # Run Go-based load test
    go run scripts/websocket-load-test.go \
        -url="$WS_URL" \
        -apikey="$TEST_API_KEY" \
        -connections=$NUM_CONNECTIONS \
        -messages=$MESSAGES_PER_CONNECTION \
        -duration=${DURATION_SECONDS}s
fi