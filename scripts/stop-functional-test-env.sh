#!/bin/bash
# Stop all services started for functional testing

if [ -f .test-pids ]; then
    source .test-pids
    
    echo "Stopping services..."
    
    if [ ! -z "$MCP_PID" ]; then
        kill $MCP_PID 2>/dev/null || true
        echo "Stopped MCP Server (PID: $MCP_PID)"
    fi
    
    if [ ! -z "$API_PID" ]; then
        kill $API_PID 2>/dev/null || true
        echo "Stopped REST API (PID: $API_PID)"
    fi
    
    if [ ! -z "$WORKER_PID" ]; then
        kill $WORKER_PID 2>/dev/null || true
        echo "Stopped Worker (PID: $WORKER_PID)"
    fi
    
    rm .test-pids
else
    echo "No PID file found. Trying to find processes..."
    pkill -f "mcp-server" || true
    pkill -f "rest-api" || true
    pkill -f "worker" || true
fi

echo "All services stopped"