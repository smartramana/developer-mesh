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
-- Note: This needs to be run for each database after creation
-- We can't use ALTER DATABASE here as we don't know the database name
-- Instead, set it for the current session and let applications handle it
SET search_path TO mcp, public;

-- Grant usage on schema to current user
-- Using CURRENT_USER to be environment-agnostic
GRANT USAGE ON SCHEMA mcp TO CURRENT_USER;

-- Grant CREATE on schema to the database owner (which is CURRENT_USER in init scripts)
GRANT CREATE ON SCHEMA mcp TO CURRENT_USER;

-- Ensure permissions for sequence generation
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA mcp TO CURRENT_USER;

-- If we need to grant to additional users, they should be created first
-- or these grants should be run after user creation