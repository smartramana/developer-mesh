-- Remove name and description columns from contexts table
ALTER TABLE mcp.contexts 
    DROP COLUMN IF EXISTS name,
    DROP COLUMN IF EXISTS description;