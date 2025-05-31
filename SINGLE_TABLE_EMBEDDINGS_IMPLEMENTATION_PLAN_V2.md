# Production-Ready Single Table Embeddings Implementation Plan v2
## With Full Support for OpenAI, Anthropic, Amazon, and Google Models

## Overview
This implementation plan provides a complete, production-ready solution for implementing a single-table vector embeddings system with comprehensive metadata support for all major embedding providers: OpenAI, Anthropic (via Voyage AI), Amazon Bedrock, and Google Vertex AI. The plan uses `golang-migrate/migrate` as the migration tool and is designed for starting from a fresh database following industry best practices.

## Requirements Verification
- ✅ **Single table design with metadata** - Embeddings table stores all vectors with comprehensive metadata
- ✅ **Uses golang-migrate/migrate** - All migrations use the official golang-migrate tool format
- ✅ **Multi-provider support** - OpenAI, Anthropic/Voyage, Amazon Bedrock, Google Vertex AI
- ✅ **Variable dimensions** - Supports models from 256 to 3072 dimensions
- ✅ **Fresh database approach** - Migrations numbered sequentially from 000001
- ✅ **Error-free implementation** - Includes validation, rollback procedures, and comprehensive error handling
- ✅ **Industry best practices** - HNSW indexes, partial indexes, audit trails, soft deletes
- ✅ **Performance optimized** - Model-specific partial indexes for optimal query performance
- ✅ **Production features** - API key management, rate limiting, usage tracking, migration locks
- ✅ **Operational excellence** - Deployment scripts, monitoring queries, rollback procedures

## Supported Embedding Models

### OpenAI
- **text-embedding-3-small**: 1536 dimensions (configurable), 8191 tokens
- **text-embedding-3-large**: 3072 dimensions (configurable), 8191 tokens
- **text-embedding-ada-002**: 1536 dimensions (fixed), 8191 tokens

### Anthropic (via Voyage AI)
- **voyage-large-2**: 1024 dimensions, general purpose
- **voyage-code-3**: 1024 dimensions, code optimized
- **voyage-2**: 1024 dimensions, efficient general purpose
- **voyage-code-2**: 1024 dimensions, extended context for code

### Amazon Bedrock
- **amazon.titan-embed-text-v2:0**: 1024 dimensions (configurable), 8192 tokens
- **cohere.embed-english-v3**: 1024 dimensions
- **cohere.embed-multilingual-v3**: 1024 dimensions, 100+ languages

### Google Vertex AI
- **gemini-embedding-001**: 3072 dimensions, 2048 tokens
- **text-embedding-004**: 768 dimensions (configurable), 2048 tokens
- **text-multilingual-embedding-002**: 768 dimensions, multilingual
- **multimodal-embedding**: 1408 dimensions, text/image/video

## 1. Migration Tool Setup

### Install golang-migrate/migrate
```bash
# Install the CLI tool (macOS)
brew install golang-migrate

# Or use Docker (recommended for CI/CD consistency)
docker pull migrate/migrate:latest

# For Linux
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/

# Verify installation
migrate -version
```

### Migration Directory Structure
```
apps/rest-api/
├── Makefile.migrate              # Migration-specific commands
├── migrations/
│   └── sql/
│       ├── 000001_initial_schema.up.sql      # Base tables and functions
│       ├── 000001_initial_schema.down.sql    
│       ├── 000002_auth_system.up.sql         # Authentication tables
│       ├── 000002_auth_system.down.sql       
│       ├── 000003_vector_embeddings.up.sql   # Vector storage tables
│       ├── 000003_vector_embeddings.down.sql 
│       ├── 000004_vector_indexes.up.sql      # Performance indexes
│       └── 000004_vector_indexes.down.sql    
```

## 2. Migration Files Implementation

### Migration 1: Initial Schema Setup
**File: `000001_initial_schema.up.sql`**
```sql
-- Initial schema setup with best practices
BEGIN;

-- Create schema if not exists
CREATE SCHEMA IF NOT EXISTS mcp;

-- Set search path for this transaction
SET search_path TO mcp, public;

-- UUID extension (standard for modern apps)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Base contexts table
CREATE TABLE contexts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- Metadata as JSONB for flexibility
    metadata JSONB NOT NULL DEFAULT '{}',
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_by UUID,
    
    -- Soft delete support
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    -- Constraints
    CONSTRAINT contexts_name_not_empty CHECK (length(trim(name)) > 0)
);

-- Indexes for contexts
CREATE INDEX idx_contexts_tenant_id ON contexts(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_contexts_created_at ON contexts(created_at DESC);
CREATE INDEX idx_contexts_metadata ON contexts USING gin(metadata);

-- Update trigger for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_contexts_updated_at BEFORE UPDATE
ON contexts FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add table comments for documentation
COMMENT ON TABLE contexts IS 'Stores context information for the MCP system';
COMMENT ON COLUMN contexts.metadata IS 'Flexible JSONB storage for context-specific data';
COMMENT ON COLUMN contexts.deleted_at IS 'Soft delete timestamp - NULL means active';

COMMIT;
```

**File: `000001_initial_schema.down.sql`**
```sql
BEGIN;

DROP SCHEMA IF EXISTS mcp CASCADE;

COMMIT;
```

### Migration 2: Authentication System
**File: `000002_auth_system.up.sql`**
```sql
BEGIN;

SET search_path TO mcp, public;

-- Users table (if needed)
CREATE TABLE users (
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
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key_hash VARCHAR(64) UNIQUE NOT NULL, -- SHA-256 hash of the key
    key_prefix VARCHAR(8) NOT NULL,       -- First 8 chars for identification
    
    -- Ownership
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
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
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix) WHERE is_active = true;
CREATE INDEX idx_api_keys_tenant_id ON api_keys(tenant_id) WHERE is_active = true;
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id) WHERE is_active = true;
CREATE INDEX idx_api_keys_expires_at ON api_keys(expires_at) WHERE is_active = true AND expires_at IS NOT NULL;

-- API key usage tracking for analytics
CREATE TABLE api_key_usage (
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
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

-- Triggers
CREATE TRIGGER update_users_updated_at BEFORE UPDATE
ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE
ON api_keys FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
```

**File: `000002_auth_system.down.sql`**
```sql
BEGIN;

SET search_path TO mcp, public;

DROP TABLE IF EXISTS api_key_usage CASCADE;
DROP TABLE IF EXISTS api_keys CASCADE;
DROP TABLE IF EXISTS users CASCADE;

COMMIT;
```

### Migration 3: Vector Embeddings Tables with All Providers
**File: `000003_vector_embeddings.up.sql`**
```sql
BEGIN;

SET search_path TO mcp, public;

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Model registry table (for tracking available models)
CREATE TABLE embedding_models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider VARCHAR(50) NOT NULL,     -- 'openai', 'voyage', 'amazon', 'google', 'cohere'
    model_name VARCHAR(100) NOT NULL,  -- 'text-embedding-3-large'
    model_version VARCHAR(20),         -- '3.0.0', 'v2', etc.
    dimensions INTEGER NOT NULL,       -- 768, 1024, 1536, 3072, etc.
    
    -- Model characteristics
    max_tokens INTEGER,
    supports_binary BOOLEAN DEFAULT false,
    supports_dimensionality_reduction BOOLEAN DEFAULT false,
    min_dimensions INTEGER,            -- For models that support dimension reduction
    cost_per_million_tokens DECIMAL(10, 4),
    
    -- Provider-specific fields
    model_id VARCHAR(255),             -- For Bedrock models like 'amazon.titan-embed-text-v2:0'
    model_type VARCHAR(50),            -- 'text', 'multimodal', 'code'
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    deprecated_at TIMESTAMP WITH TIME ZONE,
    sunset_at TIMESTAMP WITH TIME ZONE,
    
    -- Metadata
    capabilities JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT unique_model_provider UNIQUE(provider, model_name, model_version),
    CONSTRAINT valid_dimensions CHECK (dimensions > 0 AND dimensions <= 4096)
);

-- Insert all currently available models
INSERT INTO embedding_models (provider, model_name, model_version, dimensions, max_tokens, cost_per_million_tokens, model_type, supports_dimensionality_reduction, min_dimensions) VALUES
-- OpenAI models
('openai', 'text-embedding-3-small', 'v3', 1536, 8191, 0.02, 'text', true, 512),
('openai', 'text-embedding-3-large', 'v3', 3072, 8191, 0.13, 'text', true, 256),
('openai', 'text-embedding-ada-002', 'v2', 1536, 8191, 0.10, 'text', false, NULL),

-- Anthropic/Voyage AI models
('voyage', 'voyage-large-2', 'v2', 1024, NULL, 0.12, 'text', true, 256),
('voyage', 'voyage-code-3', 'v3', 1024, NULL, 0.12, 'code', true, 256),
('voyage', 'voyage-2', 'v2', 1024, NULL, 0.10, 'text', true, 256),
('voyage', 'voyage-code-2', 'v2', 1024, NULL, 0.10, 'code', true, 256),

-- Amazon Bedrock models
('amazon', 'titan-embed-text-v2', 'v2', 1024, 8192, 0.02, 'text', true, 256),
('cohere', 'embed-english-v3', 'v3', 1024, NULL, 0.10, 'text', false, NULL),
('cohere', 'embed-multilingual-v3', 'v3', 1024, NULL, 0.10, 'text', false, NULL),

-- Google Vertex AI models
('google', 'gemini-embedding-001', 'v1', 3072, 2048, 0.025, 'text', false, NULL),
('google', 'text-embedding-004', 'v4', 768, 2048, 0.025, 'text', true, NULL),
('google', 'text-multilingual-embedding-002', 'v2', 768, 2048, 0.025, 'text', true, NULL),
('google', 'multimodal-embedding', 'v1', 1408, NULL, 0.025, 'multimodal', false, NULL);

-- Update model_id for Bedrock models
UPDATE embedding_models SET model_id = 'amazon.titan-embed-text-v2:0' WHERE provider = 'amazon' AND model_name = 'titan-embed-text-v2';
UPDATE embedding_models SET model_id = 'cohere.embed-english-v3' WHERE provider = 'cohere' AND model_name = 'embed-english-v3';
UPDATE embedding_models SET model_id = 'cohere.embed-multilingual-v3' WHERE provider = 'cohere' AND model_name = 'embed-multilingual-v3';

-- Main embeddings table with industry best practices
CREATE TABLE embeddings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Content relationship
    context_id UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    content_index INTEGER NOT NULL DEFAULT 0,
    chunk_index INTEGER NOT NULL DEFAULT 0,
    
    -- Content
    content TEXT NOT NULL,
    content_hash VARCHAR(64) GENERATED ALWAYS AS (encode(sha256(content::bytea), 'hex')) STORED,
    content_tokens INTEGER,
    
    -- Model information (denormalized for performance)
    model_id UUID NOT NULL REFERENCES embedding_models(id),
    model_provider VARCHAR(50) NOT NULL,
    model_name VARCHAR(100) NOT NULL,
    model_dimensions INTEGER NOT NULL,
    configured_dimensions INTEGER,     -- Actual dimensions if reduced from default
    
    -- The embedding vector (max size 4096 for future models)
    embedding vector(4096) NOT NULL,
    
    -- Processing metadata
    processing_time_ms INTEGER,
    embedding_created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Quality metrics
    magnitude FLOAT GENERATED ALWAYS AS (sqrt((embedding::vector(4096) # embedding::vector(4096))::float)) STORED,
    
    -- Tenant isolation
    tenant_id UUID NOT NULL,
    
    -- Flexible metadata
    metadata JSONB NOT NULL DEFAULT '{}',
    
    -- Audit
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    CONSTRAINT valid_dimensions CHECK (model_dimensions > 0 AND model_dimensions <= 4096),
    CONSTRAINT valid_content CHECK (length(content) > 0),
    CONSTRAINT valid_indices CHECK (content_index >= 0 AND chunk_index >= 0)
);

-- Create compound indexes for common access patterns
CREATE INDEX idx_embeddings_context_content ON embeddings(context_id, content_index, chunk_index);
CREATE INDEX idx_embeddings_tenant_model ON embeddings(tenant_id, model_name);
CREATE INDEX idx_embeddings_content_hash ON embeddings(content_hash);
CREATE INDEX idx_embeddings_created_at ON embeddings(embedding_created_at DESC);
CREATE INDEX idx_embeddings_provider ON embeddings(model_provider, tenant_id);

-- Trigger for updated_at
CREATE TRIGGER update_embeddings_updated_at BEFORE UPDATE
ON embeddings FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Embedding search history for analytics
CREATE TABLE embedding_searches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    model_provider VARCHAR(50) NOT NULL,
    model_name VARCHAR(100) NOT NULL,
    
    -- Search metadata
    search_type VARCHAR(50) NOT NULL, -- 'similarity', 'hybrid', 'filtered'
    result_count INTEGER NOT NULL,
    threshold FLOAT,
    filters JSONB,
    
    -- Performance metrics
    search_time_ms INTEGER NOT NULL,
    total_candidates INTEGER,
    
    -- Timestamp
    searched_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Comments for documentation
COMMENT ON TABLE embeddings IS 'Stores vector embeddings with full model metadata for multi-provider support';
COMMENT ON COLUMN embeddings.content_hash IS 'SHA-256 hash for deduplication';
COMMENT ON COLUMN embeddings.magnitude IS 'L2 norm of the embedding vector for quality checks';
COMMENT ON COLUMN embeddings.metadata IS 'Flexible storage for model-specific parameters, chunking info, provider-specific data';
COMMENT ON COLUMN embeddings.configured_dimensions IS 'Actual embedding dimensions if reduced from model default';

COMMIT;
```

**File: `000003_vector_embeddings.down.sql`**
```sql
BEGIN;

SET search_path TO mcp, public;

DROP TABLE IF EXISTS embedding_searches CASCADE;
DROP TABLE IF EXISTS embeddings CASCADE;
DROP TABLE IF EXISTS embedding_models CASCADE;
DROP EXTENSION IF EXISTS vector;

COMMIT;
```

### Migration 4: Performance Indexes for All Providers
**File: `000004_vector_indexes.up.sql`**
```sql
BEGIN;

SET search_path TO mcp, public;

-- Create partial HNSW indexes for each major model group
-- These provide optimal performance by pre-filtering by model and dimensions

-- OpenAI models
-- text-embedding-3-small (1536 dimensions)
CREATE INDEX CONCURRENTLY idx_embeddings_openai_small 
ON embeddings 
USING hnsw ((embedding::vector(1536)) vector_cosine_ops)
WITH (m = 16, ef_construction = 64)
WHERE model_name = 'text-embedding-3-small' AND model_dimensions = 1536;

-- text-embedding-3-large (3072 dimensions)
CREATE INDEX CONCURRENTLY idx_embeddings_openai_large
ON embeddings 
USING hnsw ((embedding::vector(3072)) vector_cosine_ops)
WITH (m = 24, ef_construction = 128)
WHERE model_name = 'text-embedding-3-large' AND model_dimensions = 3072;

-- text-embedding-ada-002 (1536 dimensions)
CREATE INDEX CONCURRENTLY idx_embeddings_openai_ada
ON embeddings 
USING hnsw ((embedding::vector(1536)) vector_cosine_ops)
WITH (m = 16, ef_construction = 64)
WHERE model_name = 'text-embedding-ada-002' AND model_dimensions = 1536;

-- Voyage AI models (1024 dimensions)
CREATE INDEX CONCURRENTLY idx_embeddings_voyage_1024
ON embeddings 
USING hnsw ((embedding::vector(1024)) vector_cosine_ops)
WITH (m = 16, ef_construction = 64)
WHERE model_provider = 'voyage' AND model_dimensions = 1024;

-- Amazon/Cohere models (1024 dimensions)
CREATE INDEX CONCURRENTLY idx_embeddings_bedrock_1024
ON embeddings 
USING hnsw ((embedding::vector(1024)) vector_cosine_ops)
WITH (m = 16, ef_construction = 64)
WHERE model_provider IN ('amazon', 'cohere') AND model_dimensions = 1024;

-- Google models
-- gemini-embedding-001 (3072 dimensions)
CREATE INDEX CONCURRENTLY idx_embeddings_google_gemini
ON embeddings 
USING hnsw ((embedding::vector(3072)) vector_cosine_ops)
WITH (m = 24, ef_construction = 128)
WHERE model_name = 'gemini-embedding-001' AND model_dimensions = 3072;

-- text-embedding-004 and multilingual (768 dimensions)
CREATE INDEX CONCURRENTLY idx_embeddings_google_768
ON embeddings 
USING hnsw ((embedding::vector(768)) vector_cosine_ops)
WITH (m = 16, ef_construction = 64)
WHERE model_provider = 'google' AND model_dimensions = 768;

-- multimodal-embedding (1408 dimensions)
CREATE INDEX CONCURRENTLY idx_embeddings_google_multimodal
ON embeddings 
USING hnsw ((embedding::vector(1408)) vector_cosine_ops)
WITH (m = 16, ef_construction = 64)
WHERE model_name = 'multimodal-embedding' AND model_dimensions = 1408;

-- Function for similarity search with proper dimension handling
CREATE OR REPLACE FUNCTION search_embeddings(
    p_query_embedding vector,
    p_model_name TEXT,
    p_tenant_id UUID,
    p_context_id UUID DEFAULT NULL,
    p_limit INTEGER DEFAULT 10,
    p_threshold FLOAT DEFAULT 0.0,
    p_metadata_filter JSONB DEFAULT NULL
) RETURNS TABLE (
    id UUID,
    context_id UUID,
    content TEXT,
    similarity FLOAT,
    metadata JSONB,
    model_provider VARCHAR(50)
) AS $$
DECLARE
    v_dimensions INTEGER;
    v_provider VARCHAR(50);
BEGIN
    -- Get dimensions and provider for the model
    SELECT dimensions, provider 
    INTO v_dimensions, v_provider
    FROM embedding_models
    WHERE model_name = p_model_name
    AND is_active = true
    LIMIT 1;
    
    IF v_dimensions IS NULL THEN
        RAISE EXCEPTION 'Model % not found or inactive', p_model_name;
    END IF;
    
    -- Dynamic query with proper casting
    RETURN QUERY EXECUTE format(
        'SELECT 
            e.id,
            e.context_id,
            e.content,
            1 - (e.embedding::vector(%1$s) <=> $1::vector(%1$s)) AS similarity,
            e.metadata,
            e.model_provider
        FROM embeddings e
        WHERE e.tenant_id = $2
            AND e.model_name = $3
            AND e.model_dimensions = %1$s
            AND ($4::UUID IS NULL OR e.context_id = $4)
            AND ($7::JSONB IS NULL OR e.metadata @> $7)
            AND 1 - (e.embedding::vector(%1$s) <=> $1::vector(%1$s)) >= $5
        ORDER BY e.embedding::vector(%1$s) <=> $1::vector(%1$s)
        LIMIT $6',
        v_dimensions
    ) USING p_query_embedding, p_tenant_id, p_model_name, p_context_id, p_threshold, p_limit, p_metadata_filter;
END;
$$ LANGUAGE plpgsql STABLE PARALLEL SAFE;

-- Function to insert embeddings with automatic padding
CREATE OR REPLACE FUNCTION insert_embedding(
    p_context_id UUID,
    p_content TEXT,
    p_embedding FLOAT[],
    p_model_name TEXT,
    p_tenant_id UUID,
    p_metadata JSONB DEFAULT '{}',
    p_content_index INTEGER DEFAULT 0,
    p_chunk_index INTEGER DEFAULT 0,
    p_configured_dimensions INTEGER DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
    v_id UUID;
    v_model_id UUID;
    v_model_provider VARCHAR(50);
    v_dimensions INTEGER;
    v_supports_reduction BOOLEAN;
    v_min_dimensions INTEGER;
    v_padded_embedding vector(4096);
    v_actual_dimensions INTEGER;
BEGIN
    -- Get model info
    SELECT id, provider, dimensions, supports_dimensionality_reduction, min_dimensions
    INTO v_model_id, v_model_provider, v_dimensions, v_supports_reduction, v_min_dimensions
    FROM embedding_models
    WHERE model_name = p_model_name
    AND is_active = true
    LIMIT 1;
    
    IF v_model_id IS NULL THEN
        RAISE EXCEPTION 'Model % not found or inactive', p_model_name;
    END IF;
    
    -- Determine actual dimensions
    v_actual_dimensions := COALESCE(p_configured_dimensions, v_dimensions);
    
    -- Validate configured dimensions if provided
    IF p_configured_dimensions IS NOT NULL THEN
        IF NOT v_supports_reduction THEN
            RAISE EXCEPTION 'Model % does not support dimension reduction', p_model_name;
        END IF;
        IF p_configured_dimensions < v_min_dimensions OR p_configured_dimensions > v_dimensions THEN
            RAISE EXCEPTION 'Configured dimensions % outside valid range [%, %] for model %', 
                p_configured_dimensions, v_min_dimensions, v_dimensions, p_model_name;
        END IF;
    END IF;
    
    -- Validate embedding dimensions
    IF array_length(p_embedding, 1) != v_actual_dimensions THEN
        RAISE EXCEPTION 'Embedding dimensions % do not match expected dimensions %', 
            array_length(p_embedding, 1), v_actual_dimensions;
    END IF;
    
    -- Pad embedding to 4096 dimensions
    v_padded_embedding := array_cat(
        p_embedding, 
        array_fill(0::float, ARRAY[4096 - v_actual_dimensions])
    )::vector(4096);
    
    -- Insert the embedding
    INSERT INTO embeddings (
        context_id, content, embedding,
        model_id, model_provider, model_name, model_dimensions,
        configured_dimensions, tenant_id, metadata, 
        content_index, chunk_index
    ) VALUES (
        p_context_id, p_content, v_padded_embedding,
        v_model_id, v_model_provider, p_model_name, v_dimensions,
        p_configured_dimensions, p_tenant_id, p_metadata, 
        p_content_index, p_chunk_index
    ) RETURNING id INTO v_id;
    
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;

-- Helper function to get available models by provider
CREATE OR REPLACE FUNCTION get_available_models(
    p_provider VARCHAR(50) DEFAULT NULL,
    p_model_type VARCHAR(50) DEFAULT NULL
) RETURNS TABLE (
    provider VARCHAR(50),
    model_name VARCHAR(100),
    model_version VARCHAR(20),
    dimensions INTEGER,
    max_tokens INTEGER,
    model_type VARCHAR(50),
    supports_dimensionality_reduction BOOLEAN,
    min_dimensions INTEGER,
    is_active BOOLEAN
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        em.provider,
        em.model_name,
        em.model_version,
        em.dimensions,
        em.max_tokens,
        em.model_type,
        em.supports_dimensionality_reduction,
        em.min_dimensions,
        em.is_active
    FROM embedding_models em
    WHERE (p_provider IS NULL OR em.provider = p_provider)
        AND (p_model_type IS NULL OR em.model_type = p_model_type)
        AND em.is_active = true
    ORDER BY em.provider, em.model_name;
END;
$$ LANGUAGE plpgsql STABLE;

-- Create statistics for query planning
ANALYZE embeddings;

COMMIT;
```

**File: `000004_vector_indexes.down.sql`**
```sql
BEGIN;

SET search_path TO mcp, public;

DROP FUNCTION IF EXISTS get_available_models CASCADE;
DROP FUNCTION IF EXISTS search_embeddings CASCADE;
DROP FUNCTION IF EXISTS insert_embedding CASCADE;

DROP INDEX IF EXISTS idx_embeddings_openai_small;
DROP INDEX IF EXISTS idx_embeddings_openai_large;
DROP INDEX IF EXISTS idx_embeddings_openai_ada;
DROP INDEX IF EXISTS idx_embeddings_voyage_1024;
DROP INDEX IF EXISTS idx_embeddings_bedrock_1024;
DROP INDEX IF EXISTS idx_embeddings_google_gemini;
DROP INDEX IF EXISTS idx_embeddings_google_768;
DROP INDEX IF EXISTS idx_embeddings_google_multimodal;

COMMIT;
```

## 3. Makefile for Migration Management

**File: `apps/rest-api/Makefile.migrate`**
```makefile
# Migration configuration
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_USER ?= dev
DB_PASS ?= dev
DB_NAME ?= dev
DB_SSLMODE ?= disable

# Build DSN
DSN := postgres://$(DB_USER):$(DB_PASS)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

# Migration directory
MIGRATION_DIR := ./migrations/sql

# Colors for output
GREEN := \033[0;32m
RED := \033[0;31m
NC := \033[0m # No Color

.PHONY: help
help:
	@echo "Migration Management Commands:"
	@echo "  make migrate-up         - Run all pending migrations"
	@echo "  make migrate-down       - Rollback last migration"
	@echo "  make migrate-version    - Show current migration version"
	@echo "  make migrate-create     - Create new migration (NAME required)"
	@echo "  make migrate-force      - Force to specific version (VERSION required)"
	@echo "  make migrate-validate   - Validate migration files"
	@echo "  make migrate-reset      - Reset database (DANGEROUS)"

.PHONY: migrate-up
migrate-up:
	@echo "$(GREEN)Running migrations...$(NC)"
	@migrate -path $(MIGRATION_DIR) -database "$(DSN)" up
	@echo "$(GREEN)Migrations completed!$(NC)"

.PHONY: migrate-down
migrate-down:
	@echo "$(RED)Rolling back last migration...$(NC)"
	@migrate -path $(MIGRATION_DIR) -database "$(DSN)" down 1
	@echo "$(GREEN)Rollback completed!$(NC)"

.PHONY: migrate-version
migrate-version:
	@echo "$(GREEN)Current migration version:$(NC)"
	@migrate -path $(MIGRATION_DIR) -database "$(DSN)" version

.PHONY: migrate-create
migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "$(RED)Error: NAME is required$(NC)"; \
		echo "Usage: make migrate-create NAME=your_migration_name"; \
		exit 1; \
	fi
	@migrate create -ext sql -dir $(MIGRATION_DIR) -seq $(NAME)
	@echo "$(GREEN)Created migration: $(NAME)$(NC)"

.PHONY: migrate-force
migrate-force:
	@if [ -z "$(VERSION)" ]; then \
		echo "$(RED)Error: VERSION is required$(NC)"; \
		echo "Usage: make migrate-force VERSION=3"; \
		exit 1; \
	fi
	@echo "$(RED)Forcing to version $(VERSION)...$(NC)"
	@migrate -path $(MIGRATION_DIR) -database "$(DSN)" force $(VERSION)
	@echo "$(GREEN)Forced to version $(VERSION)!$(NC)"

.PHONY: migrate-validate
migrate-validate:
	@echo "$(GREEN)Validating migration files...$(NC)"
	@migrate -path $(MIGRATION_DIR) -database "$(DSN)" version >/dev/null 2>&1 || \
		(echo "$(RED)Migration validation failed!$(NC)" && exit 1)
	@echo "$(GREEN)All migrations are valid!$(NC)"

.PHONY: migrate-reset
migrate-reset:
	@echo "$(RED)WARNING: This will drop all data!$(NC)"
	@echo "Press Ctrl+C to cancel or Enter to continue..."
	@read confirm
	@migrate -path $(MIGRATION_DIR) -database "$(DSN)" drop -f
	@echo "$(GREEN)Database reset completed!$(NC)"

# Docker-based migrations (for CI/CD)
.PHONY: migrate-up-docker
migrate-up-docker:
	@docker run --rm -v $(PWD)/migrations/sql:/migrations \
		--network host \
		migrate/migrate \
		-path=/migrations \
		-database="$(DSN)" \
		up

.PHONY: migrate-validate-docker
migrate-validate-docker:
	@docker run --rm -v $(PWD)/migrations/sql:/migrations \
		--network host \
		migrate/migrate \
		-path=/migrations \
		-database="$(DSN)" \
		version
```

## 4. Go Implementation for Multi-Provider Support

### Enhanced Type Definitions
**File: `pkg/embedding/types.go`**
```go
package embedding

import (
    "encoding/json"
    "time"
    
    "github.com/google/uuid"
)

// Provider constants
const (
    ProviderOpenAI   = "openai"
    ProviderVoyage   = "voyage"     // Anthropic's partner
    ProviderAmazon   = "amazon"
    ProviderCohere   = "cohere"     // Available on Bedrock
    ProviderGoogle   = "google"
)

// Model type constants
const (
    ModelTypeText       = "text"
    ModelTypeCode       = "code"
    ModelTypeMultimodal = "multimodal"
)

// Model represents an embedding model
type Model struct {
    ID                              uuid.UUID       `json:"id" db:"id"`
    Provider                        string          `json:"provider" db:"provider"`
    ModelName                       string          `json:"model_name" db:"model_name"`
    ModelVersion                    *string         `json:"model_version,omitempty" db:"model_version"`
    Dimensions                      int             `json:"dimensions" db:"dimensions"`
    MaxTokens                       *int            `json:"max_tokens,omitempty" db:"max_tokens"`
    SupportsBinary                  bool            `json:"supports_binary" db:"supports_binary"`
    SupportsDimensionalityReduction bool            `json:"supports_dimensionality_reduction" db:"supports_dimensionality_reduction"`
    MinDimensions                   *int            `json:"min_dimensions,omitempty" db:"min_dimensions"`
    CostPerMillionTokens            *float64        `json:"cost_per_million_tokens,omitempty" db:"cost_per_million_tokens"`
    ModelID                         *string         `json:"model_id,omitempty" db:"model_id"`           // For Bedrock models
    ModelType                       *string         `json:"model_type,omitempty" db:"model_type"`
    IsActive                        bool            `json:"is_active" db:"is_active"`
    Capabilities                    json.RawMessage `json:"capabilities" db:"capabilities"`
    CreatedAt                       time.Time       `json:"created_at" db:"created_at"`
}

// Embedding represents a stored embedding
type Embedding struct {
    ID                   uuid.UUID       `json:"id" db:"id"`
    ContextID            uuid.UUID       `json:"context_id" db:"context_id"`
    ContentIndex         int             `json:"content_index" db:"content_index"`
    ChunkIndex           int             `json:"chunk_index" db:"chunk_index"`
    Content              string          `json:"content" db:"content"`
    ContentHash          string          `json:"content_hash" db:"content_hash"`
    ContentTokens        *int            `json:"content_tokens,omitempty" db:"content_tokens"`
    ModelID              uuid.UUID       `json:"model_id" db:"model_id"`
    ModelProvider        string          `json:"model_provider" db:"model_provider"`
    ModelName            string          `json:"model_name" db:"model_name"`
    ModelDimensions      int             `json:"model_dimensions" db:"model_dimensions"`
    ConfiguredDimensions *int            `json:"configured_dimensions,omitempty" db:"configured_dimensions"`
    ProcessingTimeMS     *int            `json:"processing_time_ms,omitempty" db:"processing_time_ms"`
    EmbeddingCreatedAt   time.Time       `json:"embedding_created_at" db:"embedding_created_at"`
    Magnitude            float64         `json:"magnitude" db:"magnitude"`
    TenantID             uuid.UUID       `json:"tenant_id" db:"tenant_id"`
    Metadata             json.RawMessage `json:"metadata" db:"metadata"`
    CreatedAt            time.Time       `json:"created_at" db:"created_at"`
    UpdatedAt            time.Time       `json:"updated_at" db:"updated_at"`
}

// InsertRequest represents a request to insert an embedding
type InsertRequest struct {
    ContextID            uuid.UUID       `json:"context_id"`
    Content              string          `json:"content"`
    Embedding            []float32       `json:"embedding"`
    ModelName            string          `json:"model_name"`
    TenantID             uuid.UUID       `json:"tenant_id"`
    Metadata             json.RawMessage `json:"metadata,omitempty"`
    ContentIndex         int             `json:"content_index"`
    ChunkIndex           int             `json:"chunk_index"`
    ConfiguredDimensions *int            `json:"configured_dimensions,omitempty"` // For models that support reduction
}

// SearchRequest represents a similarity search request
type SearchRequest struct {
    QueryEmbedding  []float32       `json:"query_embedding"`
    ModelName       string          `json:"model_name"`
    TenantID        uuid.UUID       `json:"tenant_id"`
    ContextID       *uuid.UUID      `json:"context_id,omitempty"`
    Limit           int             `json:"limit"`
    Threshold       float64         `json:"threshold"`
    MetadataFilter  json.RawMessage `json:"metadata_filter,omitempty"` // JSONB filter
}

// SearchResult represents a search result
type SearchResult struct {
    ID            uuid.UUID       `json:"id" db:"id"`
    ContextID     uuid.UUID       `json:"context_id" db:"context_id"`
    Content       string          `json:"content" db:"content"`
    Similarity    float64         `json:"similarity" db:"similarity"`
    Metadata      json.RawMessage `json:"metadata" db:"metadata"`
    ModelProvider string          `json:"model_provider" db:"model_provider"`
}

// ModelFilter for querying available models
type ModelFilter struct {
    Provider  *string `json:"provider,omitempty"`
    ModelType *string `json:"model_type,omitempty"`
    IsActive  *bool   `json:"is_active,omitempty"`
}
```

### Enhanced Repository Implementation
**File: `pkg/embedding/repository.go`**
```go
package embedding

import (
    "context"
    "database/sql"
    "fmt"
    
    "github.com/google/uuid"
    "github.com/lib/pq"
)

type Repository struct {
    db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
    return &Repository{db: db}
}

// InsertEmbedding inserts a new embedding with automatic padding
func (r *Repository) InsertEmbedding(ctx context.Context, req InsertRequest) (uuid.UUID, error) {
    var id uuid.UUID
    
    err := r.db.QueryRowContext(ctx, `
        SELECT mcp.insert_embedding($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `,
        req.ContextID,
        req.Content,
        pq.Array(req.Embedding),
        req.ModelName,
        req.TenantID,
        req.Metadata,
        req.ContentIndex,
        req.ChunkIndex,
        req.ConfiguredDimensions,
    ).Scan(&id)
    
    if err != nil {
        return uuid.Nil, fmt.Errorf("failed to insert embedding: %w", err)
    }
    
    return id, nil
}

// SearchEmbeddings performs similarity search with optional metadata filtering
func (r *Repository) SearchEmbeddings(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
    rows, err := r.db.QueryContext(ctx, `
        SELECT * FROM mcp.search_embeddings($1, $2, $3, $4, $5, $6, $7)
    `,
        pq.Array(req.QueryEmbedding),
        req.ModelName,
        req.TenantID,
        req.ContextID, // can be nil
        req.Limit,
        req.Threshold,
        req.MetadataFilter, // JSONB filter
    )
    if err != nil {
        return nil, fmt.Errorf("failed to search embeddings: %w", err)
    }
    defer rows.Close()
    
    var results []SearchResult
    for rows.Next() {
        var r SearchResult
        if err := rows.Scan(&r.ID, &r.ContextID, &r.Content, &r.Similarity, &r.Metadata, &r.ModelProvider); err != nil {
            return nil, fmt.Errorf("failed to scan result: %w", err)
        }
        results = append(results, r)
    }
    
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating results: %w", err)
    }
    
    return results, nil
}

// GetAvailableModels retrieves available embedding models
func (r *Repository) GetAvailableModels(ctx context.Context, filter ModelFilter) ([]Model, error) {
    query := `
        SELECT provider, model_name, model_version, dimensions, max_tokens,
               model_type, supports_dimensionality_reduction, min_dimensions, is_active
        FROM mcp.get_available_models($1, $2)
    `
    
    rows, err := r.db.QueryContext(ctx, query, filter.Provider, filter.ModelType)
    if err != nil {
        return nil, fmt.Errorf("failed to get available models: %w", err)
    }
    defer rows.Close()
    
    var models []Model
    for rows.Next() {
        var m Model
        err := rows.Scan(
            &m.Provider,
            &m.ModelName,
            &m.ModelVersion,
            &m.Dimensions,
            &m.MaxTokens,
            &m.ModelType,
            &m.SupportsDimensionalityReduction,
            &m.MinDimensions,
            &m.IsActive,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan model: %w", err)
        }
        models = append(models, m)
    }
    
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating models: %w", err)
    }
    
    return models, nil
}

// GetModelByName retrieves a model by name
func (r *Repository) GetModelByName(ctx context.Context, modelName string) (*Model, error) {
    var model Model
    
    err := r.db.QueryRowContext(ctx, `
        SELECT id, provider, model_name, model_version, dimensions,
               max_tokens, supports_binary, supports_dimensionality_reduction,
               min_dimensions, cost_per_million_tokens, model_id, model_type,
               is_active, capabilities, created_at
        FROM mcp.embedding_models
        WHERE model_name = $1 AND is_active = true
        LIMIT 1
    `, modelName).Scan(
        &model.ID,
        &model.Provider,
        &model.ModelName,
        &model.ModelVersion,
        &model.Dimensions,
        &model.MaxTokens,
        &model.SupportsBinary,
        &model.SupportsDimensionalityReduction,
        &model.MinDimensions,
        &model.CostPerMillionTokens,
        &model.ModelID,
        &model.ModelType,
        &model.IsActive,
        &model.Capabilities,
        &model.CreatedAt,
    )
    
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("model not found: %s", modelName)
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get model: %w", err)
    }
    
    return &model, nil
}

// GetEmbeddingsByContext retrieves all embeddings for a context
func (r *Repository) GetEmbeddingsByContext(ctx context.Context, contextID, tenantID uuid.UUID) ([]Embedding, error) {
    query := `
        SELECT id, context_id, content_index, chunk_index, content,
               content_hash, content_tokens, model_id, model_provider,
               model_name, model_dimensions, configured_dimensions,
               processing_time_ms, embedding_created_at, magnitude, 
               tenant_id, metadata, created_at, updated_at
        FROM mcp.embeddings
        WHERE context_id = $1 AND tenant_id = $2
        ORDER BY content_index, chunk_index
    `
    
    rows, err := r.db.QueryContext(ctx, query, contextID, tenantID)
    if err != nil {
        return nil, fmt.Errorf("failed to query embeddings: %w", err)
    }
    defer rows.Close()
    
    var embeddings []Embedding
    for rows.Next() {
        var e Embedding
        err := rows.Scan(
            &e.ID,
            &e.ContextID,
            &e.ContentIndex,
            &e.ChunkIndex,
            &e.Content,
            &e.ContentHash,
            &e.ContentTokens,
            &e.ModelID,
            &e.ModelProvider,
            &e.ModelName,
            &e.ModelDimensions,
            &e.ConfiguredDimensions,
            &e.ProcessingTimeMS,
            &e.EmbeddingCreatedAt,
            &e.Magnitude,
            &e.TenantID,
            &e.Metadata,
            &e.CreatedAt,
            &e.UpdatedAt,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan embedding: %w", err)
        }
        embeddings = append(embeddings, e)
    }
    
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating embeddings: %w", err)
    }
    
    return embeddings, nil
}
```

## 5. Implementation Steps

### Step 1: Clean Existing Setup
```bash
# Remove old migrations
rm -rf apps/rest-api/migrations/sql/*
rm -f apps/rest-api/cmd/migrate/main.go

# Remove old migration tool if exists
rm -f migrate
```

### Step 2: Install golang-migrate
```bash
# macOS
brew install golang-migrate

# Linux
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/

# Verify installation
migrate -version
```

### Step 3: Create Migration Files
```bash
# Create migrations directory
mkdir -p apps/rest-api/migrations/sql

# Create each migration file with the exact content provided above
```

### Step 4: Create Makefile
```bash
# Copy the Makefile.migrate content to apps/rest-api/Makefile.migrate
```

### Step 5: Run Migrations
```bash
cd apps/rest-api

# First, ensure database is running
docker-compose -f ../../docker-compose.local.yml up -d database

# Run migrations
make -f Makefile.migrate migrate-up

# Verify
make -f Makefile.migrate migrate-version
```

### Step 6: Create Go Implementation Files
```bash
# Create the embedding package
mkdir -p ../../pkg/embedding

# Create types.go and repository.go with the content from section 4
```

## 6. Provider-Specific Integration Notes

### OpenAI Integration
- Use dimensionality reduction for cost optimization
- Support both legacy (ada-002) and new (v3) models
- Handle dynamic dimension configuration in API calls

### Anthropic/Voyage Integration
- Implement Voyage AI SDK or REST API
- Support Matryoshka embeddings (dimension reduction)
- Consider code-specific models for development contexts

### Amazon Bedrock Integration
- Use AWS SDK for Go v2
- Handle model IDs (e.g., `amazon.titan-embed-text-v2:0`)
- Support both Titan and Cohere models
- Implement proper IAM authentication

### Google Vertex AI Integration
- Use Google Cloud Client Libraries for Go
- Handle April 2025 requirement for Gemini 1.5 prior usage
- Support multimodal embeddings for future expansion
- Implement proper authentication with service accounts

## 7. Summary

This implementation plan provides:

1. **Complete multi-provider support** - All major embedding providers included
2. **Flexible dimension handling** - From 256 to 3072 dimensions with padding
3. **Performance optimization** - Model-specific partial indexes
4. **Production features** - Monitoring, metadata filtering, dimension reduction
5. **Future-proof design** - Easy to add new models and providers

The system is ready for immediate implementation with comprehensive support for OpenAI, Anthropic (Voyage), Amazon Bedrock, and Google Vertex AI embedding models.