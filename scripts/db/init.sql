-- MCP Server Database Initialization Script (Consolidated)
-- This script creates all schemas, tables, extensions, and indices for a clean start.

-- Create schema
CREATE SCHEMA IF NOT EXISTS mcp;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Update trigger function for updated_at columns
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

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
    id VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name VARCHAR(255),
    description TEXT,
    agent_id VARCHAR(255) NOT NULL,
    model_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255),
    current_tokens INTEGER NOT NULL DEFAULT 0,
    max_tokens INTEGER NOT NULL DEFAULT 4000,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    -- Additional fields for migration compatibility
    tenant_id UUID,
    created_by UUID,
    updated_by UUID,
    deleted_at TIMESTAMP WITH TIME ZONE
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
-- Additional indexes for migration compatibility
CREATE INDEX IF NOT EXISTS idx_contexts_tenant_id ON mcp.contexts(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_contexts_created_at ON mcp.contexts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_contexts_metadata ON mcp.contexts USING gin(metadata);

-- Create update trigger for contexts table
DROP TRIGGER IF EXISTS update_contexts_updated_at ON mcp.contexts;
CREATE TRIGGER update_contexts_updated_at BEFORE UPDATE
ON mcp.contexts FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

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

-- Models table
CREATE TABLE IF NOT EXISTS mcp.models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    provider VARCHAR(255),
    model_type VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_models_tenant_id ON mcp.models(tenant_id);

-- Agents table
CREATE TABLE IF NOT EXISTS mcp.agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    model_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (model_id) REFERENCES mcp.models(id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_agents_tenant_id ON mcp.agents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agents_model_id ON mcp.agents(model_id);


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
        
        -- Model registry table (for tracking available models)
        CREATE TABLE IF NOT EXISTS mcp.embedding_models (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            provider VARCHAR(50) NOT NULL,
            model_name VARCHAR(100) NOT NULL,
            model_version VARCHAR(20),
            dimensions INTEGER NOT NULL,
            max_tokens INTEGER,
            supports_binary BOOLEAN DEFAULT false,
            supports_dimensionality_reduction BOOLEAN DEFAULT false,
            min_dimensions INTEGER,
            cost_per_million_tokens DECIMAL(10, 4),
            model_id VARCHAR(255),
            model_type VARCHAR(50),
            is_active BOOLEAN DEFAULT true,
            deprecated_at TIMESTAMP WITH TIME ZONE,
            sunset_at TIMESTAMP WITH TIME ZONE,
            capabilities JSONB DEFAULT '{}',
            created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
            CONSTRAINT unique_model_provider UNIQUE(provider, model_name, model_version),
            CONSTRAINT valid_dimensions CHECK (dimensions > 0 AND dimensions <= 4096)
        );
        
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
            id VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
            context_id VARCHAR(36) NOT NULL,
            content_index INTEGER NOT NULL DEFAULT 0,
            chunk_index INTEGER NOT NULL DEFAULT 0,
            content TEXT NOT NULL,  -- renamed from 'text' to match migrations
            content_hash VARCHAR(64),
            content_tokens INTEGER,
            embedding vector,  -- Dynamic dimensions
            vector_dimensions INTEGER NOT NULL,  -- Track dimensions
            model_id VARCHAR(255) NOT NULL,  -- Track model used
            model_provider VARCHAR(50),
            model_name VARCHAR(100),
            model_dimensions INTEGER,
            configured_dimensions INTEGER,
            processing_time_ms INTEGER,
            embedding_created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
            magnitude FLOAT,
            tenant_id UUID,
            metadata JSONB NOT NULL DEFAULT '{}',
            created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (context_id) REFERENCES mcp.contexts(id) ON DELETE CASCADE
        );

        -- Create index on context_id for fast retrieval
        CREATE INDEX IF NOT EXISTS idx_embeddings_context_id
        ON mcp.embeddings(context_id);
        
        -- Create index on model_id for filtering
        CREATE INDEX IF NOT EXISTS idx_embeddings_model_id
        ON mcp.embeddings(model_id);
        
        -- Create additional indexes for migration 004 compatibility
        CREATE INDEX IF NOT EXISTS idx_embeddings_model_dims ON mcp.embeddings(model_name, model_dimensions);
        CREATE INDEX IF NOT EXISTS idx_embeddings_provider_dims ON mcp.embeddings(model_provider, model_dimensions);
        CREATE INDEX IF NOT EXISTS idx_embeddings_tenant_model ON mcp.embeddings(tenant_id, model_name);
        CREATE INDEX IF NOT EXISTS idx_embeddings_context_model ON mcp.embeddings(context_id, model_name) WHERE context_id IS NOT NULL;
        
        -- We'll create indices dynamically when needed since we can't create them now
        -- without knowing the dimensions of the vector column
        RAISE NOTICE 'Vector indices will be created when data is inserted';
    ELSE
        -- Warn that pgvector is not available
        RAISE NOTICE 'pgvector extension is not available. Vector search capabilities will not be enabled.';
    END IF;
END $$;

-- Auth System Tables
-- Users table (if needed)
CREATE TABLE IF NOT EXISTS mcp.users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    tenant_id UUID NOT NULL,
    
    -- User metadata
    metadata JSONB NOT NULL DEFAULT '{}',
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT users_email_valid CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$')
);

-- API Keys table with production features
CREATE TABLE IF NOT EXISTS mcp.api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key_hash VARCHAR(64) UNIQUE NOT NULL, -- SHA-256 hash of the key
    key_prefix VARCHAR(8) NOT NULL,       -- First 8 chars for identification
    
    -- Ownership
    user_id UUID REFERENCES mcp.users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    
    -- Key metadata
    name VARCHAR(255) NOT NULL,
    description TEXT,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    
    -- Expiration and rotation
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    last_rotated_at TIMESTAMP WITH TIME ZONE,
    rotation_version INTEGER NOT NULL DEFAULT 1,
    
    -- Status tracking
    is_active BOOLEAN NOT NULL DEFAULT true,
    revoked_at TIMESTAMP WITH TIME ZONE,
    revoked_reason TEXT,
    
    -- Rate limiting
    rate_limit_requests INTEGER DEFAULT 1000,
    rate_limit_window_seconds INTEGER DEFAULT 3600,
    
    -- Metadata
    metadata JSONB NOT NULL DEFAULT '{}',
    ip_whitelist INET[] DEFAULT '{}',
    
    -- Audit
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    
    CONSTRAINT api_keys_name_not_empty CHECK (length(trim(name)) > 0),
    CONSTRAINT api_keys_valid_expiry CHECK (expires_at IS NULL OR expires_at > created_at)
);

-- Indexes for API keys
CREATE INDEX IF NOT EXISTS idx_api_keys_key_prefix ON mcp.api_keys(key_prefix) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_id ON mcp.api_keys(tenant_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON mcp.api_keys(user_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON mcp.api_keys(expires_at) WHERE is_active = true AND expires_at IS NOT NULL;

-- API key usage tracking for analytics
CREATE TABLE IF NOT EXISTS mcp.api_key_usage (
    api_key_id UUID NOT NULL REFERENCES mcp.api_keys(id) ON DELETE CASCADE,
    used_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip_address INET,
    user_agent TEXT,
    endpoint VARCHAR(255),
    status_code INTEGER,
    response_time_ms INTEGER,
    
    -- Partitioned by month for performance
    PRIMARY KEY (api_key_id, used_at)
) PARTITION BY RANGE (used_at);

-- Create partitions for the next 12 months
DO $$
DECLARE
    start_date date;
    end_date date;
    partition_name text;
BEGIN
    FOR i IN 0..11 LOOP
        start_date := date_trunc('month', CURRENT_DATE + (i || ' months')::interval);
        end_date := start_date + interval '1 month';
        partition_name := 'api_key_usage_' || to_char(start_date, 'YYYY_MM');
        
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS mcp.%I PARTITION OF mcp.api_key_usage
            FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
    END LOOP;
END $$;

-- Entity Relationships table
CREATE TABLE IF NOT EXISTS mcp.entity_relationships (
    id VARCHAR(255) PRIMARY KEY,
    relationship_type VARCHAR(50) NOT NULL,
    direction VARCHAR(20) NOT NULL,
    source_type VARCHAR(50) NOT NULL,
    source_owner VARCHAR(255) NOT NULL,
    source_repo VARCHAR(255) NOT NULL,
    source_id VARCHAR(255) NOT NULL,
    target_type VARCHAR(50) NOT NULL,
    target_owner VARCHAR(255) NOT NULL,
    target_repo VARCHAR(255) NOT NULL,
    target_id VARCHAR(255) NOT NULL,
    strength FLOAT8 NOT NULL,
    context TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Indexes for entity relationships
CREATE INDEX IF NOT EXISTS idx_relationships_source_type ON mcp.entity_relationships(source_type);
CREATE INDEX IF NOT EXISTS idx_relationships_target_type ON mcp.entity_relationships(target_type);
CREATE INDEX IF NOT EXISTS idx_relationships_source_owner_repo ON mcp.entity_relationships(source_owner, source_repo);
CREATE INDEX IF NOT EXISTS idx_relationships_target_owner_repo ON mcp.entity_relationships(target_owner, target_repo);
CREATE INDEX IF NOT EXISTS idx_relationships_relationship_type ON mcp.entity_relationships(relationship_type);
CREATE INDEX IF NOT EXISTS idx_relationships_direction ON mcp.entity_relationships(direction);
CREATE INDEX IF NOT EXISTS idx_relationships_source_id ON mcp.entity_relationships(source_id);
CREATE INDEX IF NOT EXISTS idx_relationships_target_id ON mcp.entity_relationships(target_id);
CREATE INDEX IF NOT EXISTS idx_relationships_updated_at ON mcp.entity_relationships(updated_at);

-- GitHub content metadata table
CREATE TABLE IF NOT EXISTS mcp.github_content_metadata (
    id VARCHAR(255) PRIMARY KEY,
    owner VARCHAR(255) NOT NULL,
    repo VARCHAR(255) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    content_id VARCHAR(255) NOT NULL,
    checksum VARCHAR(64),
    uri TEXT,
    size BIGINT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}'
);

-- Indexes for GitHub content
CREATE INDEX IF NOT EXISTS idx_github_content_owner ON mcp.github_content_metadata(owner);
CREATE INDEX IF NOT EXISTS idx_github_content_repo ON mcp.github_content_metadata(repo);
CREATE INDEX IF NOT EXISTS idx_github_content_type ON mcp.github_content_metadata(content_type);
CREATE INDEX IF NOT EXISTS idx_github_content_content_id ON mcp.github_content_metadata(content_id);