#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Required tables in mcp schema
REQUIRED_TABLES=(
    "mcp.contexts"
    "mcp.context_items"
    "mcp.api_keys"
    "mcp.agents"
    "mcp.models"
    "mcp.users"
    "mcp.tenant_config"
    "mcp.tool_configurations"
    "mcp.tool_discovery_sessions"
    "mcp.tool_discovery_patterns"
    "mcp.tool_executions"
    "mcp.tool_auth_configs"
    "mcp.tool_health_checks"
    "mcp.webhook_configs"
    "mcp.webhook_dlq"
    "mcp.embeddings"
    "mcp.embedding_models"
    "mcp.embedding_cache"
    "mcp.tasks"
    "mcp.task_delegations"
    "mcp.workflows"
    "mcp.workflow_executions"
    "mcp.workspaces"
    "mcp.workspace_members"
    "mcp.shared_documents"
    "mcp.integrations"
    "mcp.events"
    "mcp.audit_log"
)

echo "🔍 Checking required database tables..."
echo "=================================="

MISSING_TABLES=()
FOUND_TABLES=0

for table in "${REQUIRED_TABLES[@]}"; do
    schema="${table%%.*}"
    table_name="${table#*.}"
    
    result=$(docker-compose -f docker-compose.local.yml exec -T database psql -U devmesh -d devmesh_development -tAc "
        SELECT EXISTS (
            SELECT FROM information_schema.tables 
            WHERE table_schema = '${schema}' 
            AND table_name = '${table_name}'
        );")
    
    if [ "$result" = "t" ]; then
        echo -e "${GREEN}✅${NC} $table"
        ((FOUND_TABLES++))
    else
        echo -e "${RED}❌${NC} $table ${RED}MISSING${NC}"
        MISSING_TABLES+=("$table")
    fi
done

echo "=================================="

# Check for partitioned tables
echo ""
echo "📊 Checking partitioned tables..."
PARTITIONED_TABLES=$(docker-compose -f docker-compose.local.yml exec -T database psql -U devmesh -d devmesh_development -tAc "
    SELECT COUNT(*) 
    FROM pg_partitioned_table pt
    JOIN pg_class c ON c.oid = pt.partrelid
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'mcp';")

echo "Found $PARTITIONED_TABLES partitioned tables in mcp schema"

# Summary
echo ""
echo "📈 Summary:"
echo "----------"
echo -e "${GREEN}Found:${NC} $FOUND_TABLES/${#REQUIRED_TABLES[@]} tables"

if [ ${#MISSING_TABLES[@]} -gt 0 ]; then
    echo -e "${RED}Missing tables:${NC}"
    for table in "${MISSING_TABLES[@]}"; do
        echo "  - $table"
    done
    echo ""
    echo -e "${YELLOW}⚠️  Some tables are missing!${NC}"
    echo "Run './scripts/local/reset-db.sh' to reset and recreate all tables"
    exit 1
else
    echo -e "${GREEN}✅ All required tables exist!${NC}"
    
    # Additional checks
    echo ""
    echo "🔍 Additional checks:"
    
    # Check schema migrations table
    MIGRATION_VERSION=$(docker-compose -f docker-compose.local.yml exec -T database psql -U devmesh -d devmesh_development -tAc "
        SELECT version FROM mcp.schema_migrations ORDER BY version DESC LIMIT 1;" 2>/dev/null || echo "0")
    
    if [ "$MIGRATION_VERSION" != "0" ]; then
        echo -e "${GREEN}✅${NC} Latest migration version: $MIGRATION_VERSION"
    else
        echo -e "${YELLOW}⚠️${NC} Could not determine migration version"
    fi
    
    # Check if indexes exist
    INDEX_COUNT=$(docker-compose -f docker-compose.local.yml exec -T database psql -U devmesh -d devmesh_development -tAc "
        SELECT COUNT(*) 
        FROM pg_indexes 
        WHERE schemaname = 'mcp';")
    echo -e "${GREEN}✅${NC} Found $INDEX_COUNT indexes in mcp schema"
    
    # Check if functions exist
    FUNCTION_COUNT=$(docker-compose -f docker-compose.local.yml exec -T database psql -U devmesh -d devmesh_development -tAc "
        SELECT COUNT(*) 
        FROM pg_proc p
        JOIN pg_namespace n ON n.oid = p.pronamespace
        WHERE n.nspname = 'mcp';")
    echo -e "${GREEN}✅${NC} Found $FUNCTION_COUNT functions in mcp schema"
    
    echo ""
    echo -e "${GREEN}✅ Database is properly configured!${NC}"
fi