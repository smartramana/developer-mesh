-- Enhanced vector support for multiple LLM models with different dimensions
-- This migration enhances the vector storage to support multiple models

-- Check if pgvector extension exists
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM pg_available_extensions 
        WHERE name = 'vector'
    ) THEN
        -- Create a more flexible embeddings table that tracks model and dimensions
        CREATE TABLE IF NOT EXISTS mcp.embeddings (
            id VARCHAR(36) PRIMARY KEY,
            context_id VARCHAR(36) NOT NULL,
            content_index INTEGER NOT NULL,
            text TEXT NOT NULL,
            embedding vector,  -- Dynamic dimensions
            vector_dimensions INTEGER NOT NULL,  -- Track dimensions
            model_id VARCHAR(255) NOT NULL,  -- Track model used
            created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (context_id) REFERENCES mcp.contexts(id) ON DELETE CASCADE
        );

        -- Create index on context_id for fast retrieval
        CREATE INDEX IF NOT EXISTS idx_embeddings_context_id
        ON mcp.embeddings(context_id);
        
        -- Create index on model_id for filtering
        CREATE INDEX IF NOT EXISTS idx_embeddings_model_id
        ON mcp.embeddings(model_id);
        
        -- Create indices for common dimension sizes
        -- 384 dimensions (e.g., MiniLM, some Sentence Transformers)
        DO $$
        BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_indexes 
                WHERE indexname = 'idx_embeddings_384'
            ) THEN
                CREATE INDEX idx_embeddings_384
                ON mcp.embeddings USING ivfflat (embedding vector_cosine_ops)
                WITH (lists = 100)
                WHERE vector_dimensions = 384;
            END IF;
        END $$;
        
        -- 768 dimensions (e.g., BERT based models)
        DO $$
        BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_indexes 
                WHERE indexname = 'idx_embeddings_768'
            ) THEN
                CREATE INDEX idx_embeddings_768
                ON mcp.embeddings USING ivfflat (embedding vector_cosine_ops)
                WITH (lists = 100)
                WHERE vector_dimensions = 768;
            END IF;
        END $$;
        
        -- 1536 dimensions (e.g., OpenAI text-embedding-ada-002)
        DO $$
        BEGIN
            IF NOT EXISTS (
                SELECT 1 FROM pg_indexes 
                WHERE indexname = 'idx_embeddings_1536'
            ) THEN
                CREATE INDEX idx_embeddings_1536
                ON mcp.embeddings USING ivfflat (embedding vector_cosine_ops)
                WITH (lists = 100)
                WHERE vector_dimensions = 1536;
            END IF;
        END $$;
        
        -- Create flexible similarity search function that respects model and dimensions
        CREATE OR REPLACE FUNCTION mcp.search_similar_embeddings(
            p_context_id VARCHAR(36),
            p_embedding vector,
            p_model_id VARCHAR(255),
            p_dimensions INTEGER,
            p_limit INTEGER DEFAULT 10,
            p_threshold FLOAT DEFAULT 0.7
        )
        RETURNS TABLE (
            id VARCHAR(36),
            context_id VARCHAR(36),
            content_index INTEGER,
            text TEXT,
            model_id VARCHAR(255),
            similarity FLOAT
        )
        AS $$
        BEGIN
            RETURN QUERY
            SELECT 
                e.id,
                e.context_id,
                e.content_index,
                e.text,
                e.model_id,
                (1 - (e.embedding <=> p_embedding))::FLOAT AS similarity
            FROM 
                mcp.embeddings e
            WHERE 
                e.context_id = p_context_id
                AND e.model_id = p_model_id
                AND e.vector_dimensions = p_dimensions
                AND (1 - (e.embedding <=> p_embedding))::FLOAT >= p_threshold
            ORDER BY 
                similarity DESC
            LIMIT p_limit;
        END;
        $$ LANGUAGE plpgsql;
        
        -- Add comments for documentation
        COMMENT ON TABLE mcp.embeddings IS 'Stores vector embeddings for multiple LLM models with different dimensions';
        COMMENT ON FUNCTION mcp.search_similar_embeddings IS 'Searches for similar embeddings within a context, respecting model type and dimensions';
    ELSE
        -- Warn that pgvector is not available
        RAISE NOTICE 'pgvector extension is not available. Multi-model vector search capabilities will not be enabled.';
    END IF;
END $$;
