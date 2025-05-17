-- Create mcp schema if it doesn't exist
CREATE SCHEMA IF NOT EXISTS mcp;

-- Create contexts table
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

-- Create context_items table
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

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_contexts_agent_id ON mcp.contexts(agent_id);
CREATE INDEX IF NOT EXISTS idx_contexts_session_id ON mcp.contexts(session_id);
CREATE INDEX IF NOT EXISTS idx_contexts_updated_at ON mcp.contexts(updated_at);
CREATE INDEX IF NOT EXISTS idx_context_items_context_id ON mcp.context_items(context_id);
CREATE INDEX IF NOT EXISTS idx_context_items_role ON mcp.context_items(role);
CREATE INDEX IF NOT EXISTS idx_context_items_timestamp ON mcp.context_items(timestamp);

-- Add comment to tables and schema
COMMENT ON SCHEMA mcp IS 'Schema for MCP (Model Context Protocol) related tables';
COMMENT ON TABLE mcp.contexts IS 'Stores conversation contexts for AI agents';
COMMENT ON TABLE mcp.context_items IS 'Stores individual items (messages, events) within contexts';
