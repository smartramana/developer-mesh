-- Migration: Add base_url column to tool_configurations
-- Version: 20250127_05
-- Description: Adds base_url column to tool_configurations table

-- Insert migration record
INSERT INTO migration_metadata (version, description) 
VALUES ('20250127_05', 'Add base_url column to tool_configurations');

-- Add base_url column to tool_configurations table
ALTER TABLE tool_configurations 
ADD COLUMN IF NOT EXISTS base_url VARCHAR(2048);

-- Update existing records to extract base_url from config if available
UPDATE tool_configurations
SET base_url = config->>'base_url'
WHERE base_url IS NULL 
AND config ? 'base_url';

-- Create index on base_url for performance
CREATE INDEX IF NOT EXISTS idx_tool_configurations_base_url 
ON tool_configurations(base_url);

-- Add comment for documentation
COMMENT ON COLUMN tool_configurations.base_url IS 'Base URL for the tool API endpoint';