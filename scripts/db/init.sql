-- Initialize database tables for MCP Server

-- Create schema
CREATE SCHEMA IF NOT EXISTS mcp;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Events table
CREATE TABLE IF NOT EXISTS mcp.events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source VARCHAR(50) NOT NULL,
    type VARCHAR(100) NOT NULL,
    data JSONB NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    processed_at TIMESTAMP WITH TIME ZONE,
    error TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create index on events
CREATE INDEX IF NOT EXISTS idx_events_source_type ON mcp.events(source, type);
CREATE INDEX IF NOT EXISTS idx_events_processed ON mcp.events(processed);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON mcp.events(timestamp);

-- Context tables
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

-- Integrations table
CREATE TABLE IF NOT EXISTS mcp.integrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    config JSONB NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create index on integrations
CREATE INDEX IF NOT EXISTS idx_integrations_type ON mcp.integrations(type);
CREATE INDEX IF NOT EXISTS idx_integrations_active ON mcp.integrations(active);

-- Metrics table
CREATE TABLE IF NOT EXISTS mcp.metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    labels JSONB,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create index on metrics
CREATE INDEX IF NOT EXISTS idx_metrics_name ON mcp.metrics(name);
CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON mcp.metrics(timestamp);

-- Vector extension and embeddings table for semantic search
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM pg_available_extensions 
        WHERE name = 'vector'
    ) THEN
        -- Create vector extension for semantic search
        CREATE EXTENSION IF NOT EXISTS vector;
        
        -- Create vector table for context items
        CREATE TABLE IF NOT EXISTS mcp.context_item_vectors (
            id VARCHAR(36) PRIMARY KEY,
            context_id VARCHAR(36) NOT NULL,
            item_id VARCHAR(36) NOT NULL,
            embedding vector(1536),
            FOREIGN KEY (context_id) REFERENCES mcp.contexts(id) ON DELETE CASCADE,
            FOREIGN KEY (item_id) REFERENCES mcp.context_items(id) ON DELETE CASCADE
        );
        
        -- Create index for vector similarity search
        CREATE INDEX IF NOT EXISTS idx_context_item_vectors_embedding
        ON mcp.context_item_vectors
        USING ivfflat (embedding vector_l2_ops)
        WITH (lists = 100);
        
        -- Create flexible embeddings table that tracks model and dimensions
        CREATE TABLE IF NOT EXISTS mcp.embeddings (
            id VARCHAR(36) PRIMARY KEY,
            context_id VARCHAR(36) NOT NULL,
            content_index INTEGER NOT NULL,
            text TEXT NOT NULL,
            embedding vector,  -- Dynamic dimensions
            vector_dimensions INTEGER NOT NULL,  -- Track dimensions
            model_id VARCHAR(255) NOT NULL,  -- Track model used
            created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (context_id) REFERENCES mcp.contexts(id) ON DELETE CASCADE
        );

        -- Create index on context_id for fast retrieval
        CREATE INDEX IF NOT EXISTS idx_embeddings_context_id
        ON mcp.embeddings(context_id);
        
        -- Create index on model_id for filtering
        CREATE INDEX IF NOT EXISTS idx_embeddings_model_id
        ON mcp.embeddings(model_id);
        
        -- We'll create indices dynamically when needed since we can't create them now
        -- without knowing the dimensions of the vector column
        RAISE NOTICE 'Vector indices will be created when data is inserted';
    ELSE
        -- Warn that pgvector is not available
        RAISE NOTICE 'pgvector extension is not available. Vector search capabilities will not be enabled.';
    END IF;
END $$;