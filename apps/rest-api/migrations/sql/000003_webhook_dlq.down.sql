-- ==============================================================================
-- Migration: 000003_webhook_dlq (DOWN)
-- Description: Remove webhook dead letter queue table
-- Author: DBA Team
-- Date: 2025-08-05
-- ==============================================================================

BEGIN;

-- Drop trigger first
DROP TRIGGER IF EXISTS update_dlq_retry_trigger ON mcp.webhook_dlq;

-- Drop function
DROP FUNCTION IF EXISTS mcp.update_dlq_retry();

-- Drop RLS policy
DROP POLICY IF EXISTS public_access_webhook_dlq ON mcp.webhook_dlq;

-- Drop indexes
DROP INDEX IF EXISTS mcp.idx_webhook_dlq_status_created;
DROP INDEX IF EXISTS mcp.idx_webhook_dlq_event_id;
DROP INDEX IF EXISTS mcp.idx_webhook_dlq_retry_eligible;

-- Drop table
DROP TABLE IF EXISTS mcp.webhook_dlq;

COMMIT;