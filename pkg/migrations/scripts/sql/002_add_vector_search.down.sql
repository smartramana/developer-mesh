-- Drop function if exists
DROP FUNCTION IF EXISTS mcp.search_similar_context_items;

-- Drop vector table if exists
DROP TABLE IF EXISTS mcp.context_item_vectors;

-- Remove vector extension if no other tables depend on it
DO $$
BEGIN
    -- Check if vector extension is installed
    IF EXISTS (
        SELECT 1
        FROM pg_extension
        WHERE extname = 'vector'
    ) THEN
        -- Check if any other tables use vector columns
        IF NOT EXISTS (
            SELECT 1
            FROM pg_attribute a
            JOIN pg_class c ON a.attrelid = c.oid
            JOIN pg_namespace n ON c.relnamespace = n.oid
            JOIN pg_type t ON a.atttypid = t.oid
            JOIN pg_type bt ON t.typbasetype = bt.oid
            WHERE bt.typname = 'vector'
            AND (n.nspname || '.' || c.relname) != 'mcp.context_item_vectors'
        ) THEN
            -- Drop extension if no other tables use it
            DROP EXTENSION IF EXISTS vector;
        END IF;
    END IF;
END $$;
