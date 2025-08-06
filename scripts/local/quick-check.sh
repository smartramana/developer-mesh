#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔍 Quick System Check${NC}"
echo "=========================="

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}❌ Docker is not running${NC}"
    echo "Please start Docker Desktop or Colima"
    exit 1
fi

# Check if containers are running
echo ""
echo -e "${BLUE}📦 Container Status:${NC}"

REQUIRED_SERVICES=("database" "redis" "mcp-server" "rest-api" "worker")
ALL_RUNNING=true

for service in "${REQUIRED_SERVICES[@]}"; do
    if docker-compose -f docker-compose.local.yml ps | grep -q "${service}.*Up"; then
        echo -e "${GREEN}✅${NC} $service is running"
    else
        echo -e "${RED}❌${NC} $service is NOT running"
        ALL_RUNNING=false
    fi
done

if [ "$ALL_RUNNING" = false ]; then
    echo ""
    echo -e "${YELLOW}⚠️  Some services are not running${NC}"
    echo "Run: docker-compose -f docker-compose.local.yml up -d"
    exit 1
fi

# Check database tables
echo ""
echo -e "${BLUE}🗄️  Database Check:${NC}"

# Check if mcp schema exists
SCHEMA_EXISTS=$(docker-compose -f docker-compose.local.yml exec -T database psql -U devmesh -d devmesh_development -tAc "
    SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = 'mcp');")

if [ "$SCHEMA_EXISTS" = "t" ]; then
    echo -e "${GREEN}✅${NC} MCP schema exists"
else
    echo -e "${RED}❌${NC} MCP schema missing"
    echo "Run: ./scripts/local/reset-db.sh"
    exit 1
fi

# Count tables in mcp schema
TABLE_COUNT=$(docker-compose -f docker-compose.local.yml exec -T database psql -U devmesh -d devmesh_development -tAc "
    SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'mcp';")

if [ "$TABLE_COUNT" -gt 40 ]; then
    echo -e "${GREEN}✅${NC} Found $TABLE_COUNT tables in mcp schema"
else
    echo -e "${YELLOW}⚠️${NC} Only $TABLE_COUNT tables found (expected 40+)"
    echo "Run: ./scripts/local/verify-migrations.sh for details"
fi

# Check service health endpoints
echo ""
echo -e "${BLUE}🏥 Service Health:${NC}"

# MCP Server health check
if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    echo -e "${GREEN}✅${NC} MCP Server (8080) - Healthy"
else
    echo -e "${RED}❌${NC} MCP Server (8080) - Unhealthy or unreachable"
fi

# REST API health check
if curl -sf http://localhost:8081/health > /dev/null 2>&1; then
    echo -e "${GREEN}✅${NC} REST API (8081) - Healthy"
else
    echo -e "${RED}❌${NC} REST API (8081) - Unhealthy or unreachable"
fi

# Redis connectivity check
if docker-compose -f docker-compose.local.yml exec -T redis redis-cli ping 2>/dev/null | grep -q PONG; then
    echo -e "${GREEN}✅${NC} Redis - Connected"
else
    echo -e "${RED}❌${NC} Redis - Connection failed"
fi

# Check for recent errors in logs
echo ""
echo -e "${BLUE}📋 Recent Errors (last 5 minutes):${NC}"

ERROR_COUNT=0

for service in "mcp-server" "rest-api" "worker"; do
    ERRORS=$(docker-compose -f docker-compose.local.yml logs --since 5m $service 2>&1 | grep -c "ERROR\|FATAL\|panic" || true)
    if [ "$ERRORS" -gt 0 ]; then
        echo -e "${YELLOW}⚠️${NC} $service has $ERRORS errors in recent logs"
        ERROR_COUNT=$((ERROR_COUNT + ERRORS))
    fi
done

if [ "$ERROR_COUNT" -eq 0 ]; then
    echo -e "${GREEN}✅${NC} No recent errors found"
fi

# Summary
echo ""
echo "=========================="

if [ "$ALL_RUNNING" = true ] && [ "$SCHEMA_EXISTS" = "t" ] && [ "$TABLE_COUNT" -gt 40 ] && [ "$ERROR_COUNT" -eq 0 ]; then
    echo -e "${GREEN}✅ System is ready!${NC}"
    echo ""
    echo "Quick links:"
    echo "  • MCP Server: http://localhost:8080"
    echo "  • REST API: http://localhost:8081"
    echo "  • Database: localhost:5432 (devmesh/devmesh)"
    echo "  • Redis: localhost:6379"
    echo ""
    echo "View logs: docker-compose -f docker-compose.local.yml logs -f"
else
    echo -e "${YELLOW}⚠️  System needs attention${NC}"
    echo ""
    echo "Troubleshooting:"
    echo "  1. Run: ./scripts/local/verify-migrations.sh"
    echo "  2. Check logs: docker-compose -f docker-compose.local.yml logs --tail=50"
    echo "  3. Reset if needed: ./scripts/local/reset-db.sh"
    exit 1
fi