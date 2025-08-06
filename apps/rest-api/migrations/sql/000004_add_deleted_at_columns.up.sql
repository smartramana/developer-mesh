-- ==============================================================================
-- Migration: 000004_add_deleted_at_columns
-- Description: Add deleted_at column to shared_documents table for soft deletes
-- Author: Sr Staff Engineer
-- Date: 2025-08-06
-- ==============================================================================

BEGIN;

-- ==============================================================================
-- ADD DELETED_AT COLUMN TO SHARED_DOCUMENTS
-- ==============================================================================

-- Add deleted_at column to shared_documents for soft deletes
ALTER TABLE mcp.shared_documents 
ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;

-- Add index for performance on non-deleted records
CREATE INDEX IF NOT EXISTS idx_shared_documents_deleted_at 
ON mcp.shared_documents(deleted_at) 
WHERE deleted_at IS NULL;

-- ==============================================================================
-- CHECK IF TASKS TABLE NEEDS DELETED_AT
-- ==============================================================================

-- Check and add deleted_at to tasks table if needed
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_schema = 'mcp' 
        AND table_name = 'tasks' 
        AND column_name = 'deleted_at'
    ) THEN
        ALTER TABLE mcp.tasks 
        ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE;
        
        CREATE INDEX idx_tasks_deleted_at 
        ON mcp.tasks(deleted_at) 
        WHERE deleted_at IS NULL;
    END IF;
END $$;

-- ==============================================================================
-- COMMENTS
-- ==============================================================================

COMMENT ON COLUMN mcp.shared_documents.deleted_at IS 'Timestamp when the document was soft deleted';
COMMENT ON COLUMN mcp.tasks.deleted_at IS 'Timestamp when the task was soft deleted';

COMMIT;