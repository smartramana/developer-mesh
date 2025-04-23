-- Setup test database for functional tests

-- Enable vector extension for pgvector
CREATE EXTENSION IF NOT EXISTS vector;

-- Setup test data for functional tests

-- Insert test integration
INSERT INTO mcp.integrations (id, name, type, config, active, created_at, updated_at)
VALUES (
  '11111111-1111-1111-1111-111111111111',
  'test-github-integration',
  'github',
  '{"baseUrl": "https://api.github.com", "token": "test-token", "owner": "test-owner", "repo": "test-repo"}',
  true,
  NOW(),
  NOW()
);

-- Create test contexts table if it doesn't exist (for backward compatibility)
CREATE TABLE IF NOT EXISTS mcp.contexts (
    id VARCHAR(36) PRIMARY KEY,
    agent_id VARCHAR(255) NOT NULL,
    model_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255),
    current_tokens INTEGER NOT NULL DEFAULT 0,
    max_tokens INTEGER NOT NULL DEFAULT 4000,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE
);

-- Insert test context
INSERT INTO mcp.contexts (id, agent_id, model_id, session_id, current_tokens, max_tokens, metadata, created_at, updated_at, expires_at)
VALUES (
  'ctx-test-001',
  'test-agent',
  'gpt-4',
  'test-session',
  0,
  4000,
  '{"test": true}',
  NOW(),
  NOW(),
  NOW() + INTERVAL '1 day'
);

-- Create context_items table if it doesn't exist
CREATE TABLE IF NOT EXISTS mcp.context_items (
    id VARCHAR(36) PRIMARY KEY,
    context_id VARCHAR(36) NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    tokens INTEGER NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    metadata JSONB,
    FOREIGN KEY (context_id) REFERENCES mcp.contexts(id) ON DELETE CASCADE
);

-- Insert test context item
INSERT INTO mcp.context_items (id, context_id, role, content, tokens, timestamp, metadata)
VALUES (
  'item-test-001',
  'ctx-test-001',
  'user',
  'This is a test message',
  5,
  NOW(),
  '{"test": true}'
);

-- Skip vector operations for now to avoid potential issues
-- CREATE TABLE IF NOT EXISTS mcp.context_item_vectors (
--     id VARCHAR(36) PRIMARY KEY,
--     context_id VARCHAR(36) NOT NULL,
--     item_id VARCHAR(36) NOT NULL,
--     embedding vector(1536),
--     FOREIGN KEY (context_id) REFERENCES mcp.contexts(id) ON DELETE CASCADE,
--     FOREIGN KEY (item_id) REFERENCES mcp.context_items(id) ON DELETE CASCADE
-- );
