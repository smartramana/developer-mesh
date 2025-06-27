-- Reset database script
-- This script drops and recreates the mcp schema

-- Drop the schema cascade (this will drop all tables, functions, etc.)
DROP SCHEMA IF EXISTS mcp CASCADE;

-- Recreate the schema
CREATE SCHEMA mcp;

-- Grant permissions
GRANT USAGE ON SCHEMA mcp TO postgres;
GRANT CREATE ON SCHEMA mcp TO postgres;

-- Note: Extensions and other setup will be handled by init.sql and migrations