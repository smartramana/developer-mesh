-- Add name and description columns to contexts table
ALTER TABLE mcp.contexts 
    ADD COLUMN IF NOT EXISTS name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS description TEXT;

-- Add comments for new columns
COMMENT ON COLUMN mcp.contexts.name IS 'Human-readable name for the context';
COMMENT ON COLUMN mcp.contexts.description IS 'Optional description of the context purpose';