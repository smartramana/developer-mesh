-- ==============================================================================
-- Migration: 000004_add_deleted_at_columns (ROLLBACK)
-- Description: Remove deleted_at columns from shared_documents and tasks tables
-- ==============================================================================

BEGIN;

-- Drop indexes first
DROP INDEX IF EXISTS mcp.idx_shared_documents_deleted_at;
DROP INDEX IF EXISTS mcp.idx_tasks_deleted_at;

-- Remove columns
ALTER TABLE mcp.shared_documents DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE mcp.tasks DROP COLUMN IF EXISTS deleted_at;

COMMIT;