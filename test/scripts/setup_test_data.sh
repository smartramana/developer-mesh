#!/bin/bash
# setup_test_data.sh - Script to initialize test data for functional tests

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Setting up test data...${NC}"

# Database connection details
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-dev}"
DB_USER="${DB_USER:-dev}"
DB_PASSWORD="${DB_PASSWORD:-dev}"

# Export for psql
export PGPASSWORD=$DB_PASSWORD

# Function to execute SQL
execute_sql() {
    local sql=$1
    docker exec -e PGPASSWORD=$DB_PASSWORD developer-mesh-database-1 psql -U $DB_USER -d $DB_NAME -c "$sql" > /dev/null 2>&1
}

# Function to execute SQL from file
execute_sql_file() {
    local file=$1
    docker exec -e PGPASSWORD=$DB_PASSWORD -i developer-mesh-database-1 psql -U $DB_USER -d $DB_NAME < "$file" > /dev/null 2>&1
}

# Wait for database to be ready using Docker
echo -e "${YELLOW}Waiting for database to be ready...${NC}"
until docker exec developer-mesh-database-1 pg_isready -U $DB_USER > /dev/null 2>&1; do
    echo -n "."
    sleep 1
done
echo -e "\n${GREEN}Database is ready!${NC}"

# Database should already exist (created by docker-compose), just verify
echo -e "${YELLOW}Verifying database exists...${NC}"
if docker exec developer-mesh-database-1 psql -U $DB_USER -lqt | grep -q "$DB_NAME"; then
    echo -e "${GREEN}Database '$DB_NAME' exists!${NC}"
else
    echo -e "${RED}Database '$DB_NAME' does not exist!${NC}"
    exit 1
fi

# Create test data
echo -e "${YELLOW}Creating test data...${NC}"

# Create test models
execute_sql "
INSERT INTO mcp.models (id, name, tenant_id) VALUES 
    (uuid_generate_v4(), 'GPT-4', 'test-tenant'),
    (uuid_generate_v4(), 'Claude-3', 'test-tenant'),
    (uuid_generate_v4(), 'Test Model', 'test-tenant')
ON CONFLICT (id) DO NOTHING;
"

# Get model IDs for use in agents
MODEL_ID=$(docker exec -e PGPASSWORD=$DB_PASSWORD developer-mesh-database-1 psql -U $DB_USER -d $DB_NAME -t -c "SELECT id FROM mcp.models WHERE name='GPT-4' LIMIT 1" | xargs)

# Create test agents
execute_sql "
INSERT INTO mcp.agents (id, name, tenant_id, description, model_id, created_at, updated_at) VALUES 
    (uuid_generate_v4(), 'Test Agent 1', 'test-tenant', 'Test agent for functional tests', '$MODEL_ID', NOW(), NOW()),
    (uuid_generate_v4(), 'Test Agent 2', 'test-tenant', 'Another test agent', '$MODEL_ID', NOW(), NOW()),
    (uuid_generate_v4(), 'GitHub Agent', 'test-tenant', 'Agent for GitHub integration', '$MODEL_ID', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;
"

# Get agent ID for use in contexts
AGENT_ID=$(docker exec -e PGPASSWORD=$DB_PASSWORD developer-mesh-database-1 psql -U $DB_USER -d $DB_NAME -t -c "SELECT id FROM mcp.agents WHERE name='Test Agent 1' LIMIT 1" | xargs)

# Create test contexts
execute_sql "
INSERT INTO mcp.contexts (id, agent_id, model_id, name, description, max_tokens, current_tokens, created_at, updated_at) VALUES 
    ('test-context-1', '$AGENT_ID', '$MODEL_ID', 'Test Context 1', 'Context for testing', 4000, 0, NOW(), NOW()),
    ('test-context-2', '$AGENT_ID', '$MODEL_ID', 'Test Context 2', 'Another test context', 8000, 0, NOW(), NOW()),
    ('test-context-3', '$AGENT_ID', '$MODEL_ID', 'GitHub Context', 'Context for GitHub operations', 16000, 0, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;
"

# Note: The database schema uses mcp.integrations, not tools
execute_sql "
INSERT INTO mcp.integrations (id, name, type, config, active, created_at, updated_at) VALUES 
    (uuid_generate_v4(), 'GitHub Integration', 'github', '{\"repo\": \"test-repo\", \"owner\": \"test-owner\"}', true, NOW(), NOW()),
    (uuid_generate_v4(), 'Slack Integration', 'slack', '{\"channel\": \"#test\"}', true, NOW(), NOW()),
    (uuid_generate_v4(), 'Generic Integration', 'generic', '{}', true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;
"

# Create test embeddings - check if table exists first
if docker exec -e PGPASSWORD=$DB_PASSWORD developer-mesh-database-1 psql -U $DB_USER -d $DB_NAME -t -c "SELECT 1 FROM information_schema.tables WHERE table_schema='mcp' AND table_name='embeddings'" | grep -q 1; then
    execute_sql "
    INSERT INTO mcp.embeddings (id, context_id, content_index, text, vector_dimensions, model_id, created_at) VALUES 
        ('test-embedding-1', 'test-context-1', 0, 'Test text content', 1536, '$MODEL_ID', NOW()),
        ('test-embedding-2', 'test-context-1', 1, 'Another test content', 1536, '$MODEL_ID', NOW()),
        ('test-embedding-3', 'test-context-2', 0, 'Test document content', 1536, '$MODEL_ID', NOW())
    ON CONFLICT (id) DO NOTHING;
    "
fi

echo -e "${GREEN}✓ Test data setup complete!${NC}"

# Show summary
echo -e "\n${YELLOW}Test data summary:${NC}"
echo "- Models: 3"
echo "- Agents: 3"
echo "- Contexts: 3"
echo "- Integrations: 3"
echo "- Embeddings: 3 (if vector support enabled)"