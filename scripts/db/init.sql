-- MCP Server Database Initialization Script (Schema Only)
-- This script only creates the schema and necessary extensions
-- All tables are created through migrations

-- Create schema
CREATE SCHEMA IF NOT EXISTS mcp;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Enable pgvector extension if available
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM pg_available_extensions 
        WHERE name = 'vector'
    ) THEN
        CREATE EXTENSION IF NOT EXISTS vector;
        RAISE NOTICE 'pgvector extension enabled successfully';
    ELSE
        RAISE NOTICE 'pgvector extension is not available. Vector search capabilities will not be enabled.';
    END IF;
END $$;

-- Update trigger function for updated_at columns (used by migrations)
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Set default search path to include mcp schema
ALTER DATABASE devops_mcp_dev SET search_path TO mcp, public;

-- Grant usage on schema to postgres user
GRANT USAGE ON SCHEMA mcp TO postgres;
GRANT CREATE ON SCHEMA mcp TO postgres;

-- Ensure permissions for sequence generation
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA mcp TO postgres;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO postgres;