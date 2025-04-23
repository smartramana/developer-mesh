#!/bin/bash

# Stop any running MCP server and mockserver
echo "Stopping existing servers..."
pkill -f "mockserver"
pkill -f "mcp-server"

# Wait for servers to stop
sleep 2

# Start mockserver in the background
echo "Starting mockserver..."
./mockserver > mockserver.log 2>&1 &
MOCK_PID=$!
echo "Mockserver started with PID $MOCK_PID"

# Wait for mockserver to start
sleep 2

# Update the config to disable rate limiting
echo "Updating config to disable rate limiting..."
sed -i.bak 's/enabled: true/enabled: false/' configs/config.yaml

# Start MCP server in the background
echo "Starting MCP server..."
./mcp-server > mcp-server.log 2>&1 &
MCP_PID=$!
echo "MCP server started with PID $MCP_PID"

# Wait for MCP server to start
sleep 5

# Test health endpoint
echo "Testing health endpoint..."
curl -s http://localhost:8080/health

echo -e "\nStartup complete!"
