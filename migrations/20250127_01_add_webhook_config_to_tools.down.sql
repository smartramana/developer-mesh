-- Migration Rollback: Remove webhook configuration from dynamic tools
-- Version: 20250127_01
-- Description: Removes webhook configuration column and webhook event tables

-- Drop functions
DROP FUNCTION IF EXISTS update_webhook_event_status(UUID, VARCHAR(20), TEXT);
DROP FUNCTION IF EXISTS cleanup_old_webhook_events(INT);

-- Drop webhook event logs table
DROP TABLE IF EXISTS webhook_event_logs;

-- Drop webhook events table
DROP TABLE IF EXISTS webhook_events;

-- Remove webhook_config column from tool_configurations
ALTER TABLE tool_configurations 
DROP COLUMN IF EXISTS webhook_config;

-- Update migration metadata
UPDATE migration_metadata 
SET rollback_at = NOW(), 
    status = 'rolled_back' 
WHERE version = '20250127_01';