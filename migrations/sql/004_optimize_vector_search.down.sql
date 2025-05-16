-- Down migration to revert vector search optimizations

-- Drop the HNSW index
DROP INDEX IF EXISTS mcp.idx_embeddings_embedding_hnsw;

-- Drop the additional indices
DROP INDEX IF EXISTS mcp.idx_embeddings_content_type;
DROP INDEX IF EXISTS mcp.idx_embeddings_metadata;

-- Remove the triggers and functions
DROP TRIGGER IF EXISTS embeddings_updated_at ON mcp.embeddings;
DROP FUNCTION IF EXISTS mcp.update_updated_at();
DROP FUNCTION IF EXISTS mcp.update_last_searched_at();

-- Remove the added columns
ALTER TABLE mcp.embeddings 
DROP COLUMN IF EXISTS created_at,
DROP COLUMN IF EXISTS updated_at,
DROP COLUMN IF EXISTS last_searched_at;

-- Recreate the original index if needed
CREATE INDEX IF NOT EXISTS idx_embeddings_embedding ON mcp.embeddings USING ivfflat (embedding vector_l2_ops)
WITH (lists = 100);

-- Update statistics
ANALYZE mcp.embeddings;
