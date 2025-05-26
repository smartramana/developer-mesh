-- Drop context_items table
DROP TABLE IF EXISTS mcp.context_items;

-- Drop contexts table
DROP TABLE IF EXISTS mcp.contexts;

-- Check if mcp schema is empty, and drop if it is
DO $$
DECLARE
    schema_empty BOOLEAN;
BEGIN
    -- Check if schema is empty (no tables)
    SELECT COUNT(*) = 0 INTO schema_empty
    FROM pg_tables
    WHERE schemaname = 'mcp';
    
    -- Drop schema if empty
    IF schema_empty THEN
        DROP SCHEMA IF EXISTS mcp;
    END IF;
END $$;
