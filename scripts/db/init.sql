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

-- Context table
CREATE TABLE IF NOT EXISTS mcp.contexts (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    model_id TEXT NOT NULL,
    session_id TEXT,
    content JSONB NOT NULL,
    metadata JSONB,
    current_tokens INTEGER NOT NULL DEFAULT 0,
    max_tokens INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE
);

-- Create index on contexts
CREATE INDEX IF NOT EXISTS idx_contexts_agent_id ON mcp.contexts (agent_id);
CREATE INDEX IF NOT EXISTS idx_contexts_session_id ON mcp.contexts (session_id);
CREATE INDEX IF NOT EXISTS idx_contexts_created_at ON mcp.contexts (created_at);
CREATE INDEX IF NOT EXISTS idx_contexts_expires_at ON mcp.contexts (expires_at);

-- Context reference table for S3 storage
CREATE TABLE IF NOT EXISTS mcp.context_references (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    model_id TEXT NOT NULL,
    session_id TEXT,
    storage_key TEXT NOT NULL,
    metadata JSONB,
    current_tokens INTEGER NOT NULL DEFAULT 0,
    max_tokens INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE
);

-- Create index on context references
CREATE INDEX IF NOT EXISTS idx_context_refs_agent_id ON mcp.context_references (agent_id);
CREATE INDEX IF NOT EXISTS idx_context_refs_session_id ON mcp.context_references (session_id);

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

-- Enable vector extension for embedding storage
CREATE EXTENSION IF NOT EXISTS "vector";

-- Embeddings table for vector search with variable-length vectors
CREATE TABLE IF NOT EXISTS mcp.embeddings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    context_id TEXT NOT NULL REFERENCES mcp.contexts(id) ON DELETE CASCADE,
    content_index INTEGER NOT NULL, -- Index into the context content array
    text TEXT NOT NULL,             -- The text that was embedded
    embedding vector,               -- Variable-length vector to support different models
    vector_dimensions INTEGER NOT NULL, -- Store the dimensions for filtering
    model_id TEXT NOT NULL, -- Model used to generate the embedding
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create index for vector similarity search
CREATE INDEX IF NOT EXISTS idx_embeddings_vector ON mcp.embeddings USING ivfflat (embedding vector_cosine_ops);
CREATE INDEX IF NOT EXISTS idx_embeddings_dimensions ON mcp.embeddings (vector_dimensions);