-- Migration to optimize vector search with HNSW index in pgvector
-- Enable the necessary extensions if not already enabled
CREATE EXTENSION IF NOT EXISTS vector;

-- Create the HNSW index for optimized vector search
-- First, drop the existing index if it exists
DROP INDEX IF EXISTS mcp.idx_embeddings_embedding;

-- Create the HNSW index with optimal parameters
-- The 'ivfflat' index is a basic inverted file index that works well for medium-sized datasets
-- For larger datasets with millions of vectors, HNSW provides better performance
CREATE INDEX idx_embeddings_embedding_hnsw ON mcp.embeddings USING hnsw (embedding vector_l2_ops)
WITH (m = 16, ef_construction = 64);
-- m: number of connections per element in the graph (more = better recall but slower)
-- ef_construction: controls the quality of the index (higher = better recall but slower to build)

-- Additional indices for hybrid filtering and sorting
-- First add the content_type column if it doesn't exist
ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS content_type VARCHAR(50);

-- Index for content_type filtering (used in almost all queries)
CREATE INDEX IF NOT EXISTS idx_embeddings_content_type ON mcp.embeddings (content_type);

-- Create GIN index on the metadata JSONB field for faster metadata filtering
CREATE INDEX IF NOT EXISTS idx_embeddings_metadata ON mcp.embeddings USING GIN (metadata);

-- Enhanced embeddings table with additional columns for improved search and filtering
ALTER TABLE mcp.embeddings 
ADD COLUMN IF NOT EXISTS created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
ADD COLUMN IF NOT EXISTS last_searched_at TIMESTAMP WITH TIME ZONE;

-- Function to update the updated_at timestamp on row updates
CREATE OR REPLACE FUNCTION mcp.update_updated_at()
RETURNS TRIGGER AS $UPDATED_AT$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$UPDATED_AT$ LANGUAGE plpgsql;

-- Trigger to automatically update the updated_at timestamp
DROP TRIGGER IF EXISTS embeddings_updated_at ON mcp.embeddings;
CREATE TRIGGER embeddings_updated_at
BEFORE UPDATE ON mcp.embeddings
FOR EACH ROW
EXECUTE FUNCTION mcp.update_updated_at();

-- Function to update the last_searched_at timestamp when conducting similarity searches
CREATE OR REPLACE FUNCTION mcp.update_last_searched_at()
RETURNS TRIGGER AS $LAST_SEARCHED$
BEGIN
    UPDATE mcp.embeddings
    SET last_searched_at = NOW()
    WHERE id = NEW.id;
    RETURN NEW;
END;
$LAST_SEARCHED$ LANGUAGE plpgsql;

-- Statistics for query optimizations
ANALYZE mcp.embeddings;

-- Add comments to document the changes
COMMENT ON INDEX mcp.idx_embeddings_embedding_hnsw IS 'HNSW index for efficient approximate nearest neighbor search';
COMMENT ON INDEX mcp.idx_embeddings_content_type IS 'Index for filtering by content type';
COMMENT ON INDEX mcp.idx_embeddings_metadata IS 'GIN index for efficient JSON queries on metadata';
COMMENT ON COLUMN mcp.embeddings.last_searched_at IS 'Timestamp of last time this embedding was returned in search results';
