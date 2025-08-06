#!/bin/bash
set -e

echo "🔄 Resetting local database..."

# Stop all services
echo "⏹️  Stopping services..."
docker-compose -f docker-compose.local.yml down

# Remove volumes to ensure clean state
echo "🗑️  Removing database volumes..."
docker-compose -f docker-compose.local.yml down -v

# Start only database and redis
echo "🚀 Starting database and redis..."
docker-compose -f docker-compose.local.yml up -d database redis

# Wait for database to be ready
echo "⏳ Waiting for database to be ready..."
for i in {1..30}; do
    if docker-compose -f docker-compose.local.yml exec database pg_isready -U devmesh -d devmesh_development > /dev/null 2>&1; then
        echo "✅ Database is ready!"
        break
    fi
    echo -n "."
    sleep 1
done

# Give a bit more time for database initialization
sleep 2

# Check if mcp schema was created by init.sql
echo "🔍 Checking database schemas..."
docker-compose -f docker-compose.local.yml exec database psql -U devmesh -d devmesh_development -c "\dn"

# Start rest-api to run migrations
echo "🔧 Starting REST API to run migrations..."
docker-compose -f docker-compose.local.yml up -d rest-api

# Wait for REST API to initialize and run migrations
echo "⏳ Waiting for migrations to complete..."
sleep 10

# Verify tables were created
echo "✅ Verifying tables..."
docker-compose -f docker-compose.local.yml exec database psql -U devmesh -d devmesh_development -c "
SELECT table_schema, table_name 
FROM information_schema.tables 
WHERE table_schema = 'mcp' 
ORDER BY table_name
LIMIT 10;"

# Count total tables
TOTAL_TABLES=$(docker-compose -f docker-compose.local.yml exec -T database psql -U devmesh -d devmesh_development -tAc "
SELECT COUNT(*) 
FROM information_schema.tables 
WHERE table_schema = 'mcp';")

echo "📊 Total tables in mcp schema: $TOTAL_TABLES"

# Start all remaining services
echo "🚀 Starting all services..."
docker-compose -f docker-compose.local.yml up -d

echo "✅ Database reset complete!"
echo ""
echo "📋 Quick checks:"
echo "  - Database: http://localhost:5432 (devmesh/devmesh)"
echo "  - MCP Server: http://localhost:8080/health"
echo "  - REST API: http://localhost:8081/health"
echo "  - Redis: localhost:6379"
echo ""
echo "Run 'docker-compose -f docker-compose.local.yml logs -f' to view logs"