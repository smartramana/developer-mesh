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
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    data JSONB NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create index on contexts
CREATE INDEX IF NOT EXISTS idx_contexts_name ON mcp.contexts(name);
CREATE INDEX IF NOT EXISTS idx_contexts_active ON mcp.contexts(active);

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

-- Embeddings table for vector search
CREATE TABLE IF NOT EXISTS mcp.embeddings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    context_id UUID NOT NULL REFERENCES mcp.contexts(id) ON DELETE CASCADE,
    content_index INTEGER NOT NULL, -- Index into the context content array
    text TEXT NOT NULL,             -- The text that was embedded
    embedding vector(1536),         -- Default to 1536 dimensions (common for many models)
    model_id VARCHAR(100) NOT NULL, -- Model used to generate the embedding
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create index for vector similarity search
CREATE INDEX IF NOT EXISTS idx_embeddings_vector ON mcp.embeddings USING ivfflat (embedding vector_cosine_ops);