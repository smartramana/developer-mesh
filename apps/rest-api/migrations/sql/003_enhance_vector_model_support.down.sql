-- Revert the enhanced vector support for multiple LLM models
-- This down migration removes the enhancements added in the up migration

-- Check if pgvector extension exists
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM pg_available_extensions 
        WHERE name = 'vector'
    ) THEN
        -- Drop the search function
        DROP FUNCTION IF EXISTS mcp.search_similar_embeddings;
        
        -- Drop the indices
        DROP INDEX IF EXISTS idx_embeddings_384;
        DROP INDEX IF EXISTS idx_embeddings_768;
        DROP INDEX IF EXISTS idx_embeddings_1536;
        DROP INDEX IF EXISTS idx_embeddings_context_id;
        DROP INDEX IF EXISTS idx_embeddings_model_id;
        
        -- Drop the embeddings table
        DROP TABLE IF EXISTS mcp.embeddings;
    END IF;
END $$;
