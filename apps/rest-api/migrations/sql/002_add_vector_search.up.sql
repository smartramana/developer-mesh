-- Check if pgvector extension exists
DO $OUTER$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM pg_available_extensions 
        WHERE name = 'vector'
    ) THEN
        -- Create vector extension for semantic search
        CREATE EXTENSION IF NOT EXISTS vector;
        
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
        
        -- Create function to search for similar vectors
        CREATE OR REPLACE FUNCTION mcp.search_similar_context_items(
            p_context_id VARCHAR(36),
            p_embedding vector,
            p_limit INTEGER DEFAULT 10
        )
        RETURNS TABLE (
            item_id VARCHAR(36),
            distance FLOAT
        )
        AS $INNER$
        BEGIN
            RETURN QUERY
            SELECT 
                civ.item_id,
                (civ.embedding <-> p_embedding)::FLOAT AS distance
            FROM 
                mcp.context_item_vectors civ
            WHERE 
                civ.context_id = p_context_id
            ORDER BY 
                distance
            LIMIT p_limit;
        END;
        $INNER$ LANGUAGE plpgsql;
        
        -- Add comment to vector tables
        COMMENT ON TABLE mcp.context_item_vectors IS 'Stores vector embeddings for context items to enable semantic search';
        COMMENT ON FUNCTION mcp.search_similar_context_items IS 'Searches for context items with similar embeddings within a context';
    ELSE
        -- Warn that pgvector is not available
        RAISE NOTICE 'pgvector extension is not available. Vector search capabilities will not be enabled.';
    END IF;
END $OUTER$;
