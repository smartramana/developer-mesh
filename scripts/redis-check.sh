#!/bin/bash
echo "Testing Redis connectivity..."
echo "Checking from host to Docker Redis container:"
redis-cli -h redis ping || echo "Failed to connect from host to Redis"

# Check network
echo "Docker network information:"
docker network inspect developer-mesh_mcp-network

# Check Redis container
echo "Redis container information:"
docker inspect developer-mesh-redis-1

echo "Executing ping from within the Redis container:"
docker exec developer-mesh-redis-1 redis-cli ping || echo "Failed to ping from within Redis container"

echo "Checking connection from MCP server container to Redis:"
docker exec developer-mesh-mcp-server-1 sh -c "redis-cli -h redis ping" || echo "Failed to connect from MCP server to Redis"

# Try telnet to check basic TCP connectivity
echo "Checking TCP connectivity to Redis port:"
docker exec developer-mesh-mcp-server-1 sh -c "apt-get update && apt-get install -y netcat && nc -vz redis 6379" || echo "TCP connection failed"
