-- ==============================================================================
-- Migration: 000003_webhook_dlq
-- Description: Add webhook dead letter queue table for failed event processing
-- Author: DBA Team
-- Date: 2025-08-05
-- ==============================================================================

BEGIN;

-- ==============================================================================
-- WEBHOOK DLQ TABLE
-- ==============================================================================

-- Dead Letter Queue for failed webhook events
CREATE TABLE IF NOT EXISTS mcp.webhook_dlq (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    error_message TEXT NOT NULL,
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_retry_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    metadata JSONB DEFAULT '{}',
    
    -- Constraints
    CONSTRAINT chk_dlq_status CHECK (status IN ('pending', 'retrying', 'failed', 'resolved')),
    CONSTRAINT chk_retry_count CHECK (retry_count >= 0),
    CONSTRAINT chk_last_retry CHECK (last_retry_at IS NULL OR last_retry_at >= created_at)
);

-- ==============================================================================
-- INDEXES
-- ==============================================================================

-- Primary access patterns
CREATE INDEX idx_webhook_dlq_status_created 
    ON mcp.webhook_dlq(status, created_at) 
    WHERE status = 'pending';

CREATE INDEX idx_webhook_dlq_event_id 
    ON mcp.webhook_dlq(event_id);

CREATE INDEX idx_webhook_dlq_retry_eligible 
    ON mcp.webhook_dlq(status, retry_count, created_at) 
    WHERE status = 'pending' AND retry_count < 3;

-- ==============================================================================
-- ROW LEVEL SECURITY
-- ==============================================================================

-- Enable RLS
ALTER TABLE mcp.webhook_dlq ENABLE ROW LEVEL SECURITY;

-- Create RLS policy - DLQ entries are system-wide (no tenant isolation)
-- This is intentional as the worker processes events for all tenants
CREATE POLICY public_access_webhook_dlq ON mcp.webhook_dlq
    FOR ALL USING (true);

-- ==============================================================================
-- FUNCTIONS
-- ==============================================================================

-- Function to automatically update retry count and last_retry_at
CREATE OR REPLACE FUNCTION mcp.update_dlq_retry()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'retrying' AND OLD.status = 'pending' THEN
        NEW.retry_count = OLD.retry_count + 1;
        NEW.last_retry_at = CURRENT_TIMESTAMP;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for automatic retry updates
CREATE TRIGGER update_dlq_retry_trigger
    BEFORE UPDATE ON mcp.webhook_dlq
    FOR EACH ROW
    WHEN (OLD.status = 'pending' AND NEW.status = 'retrying')
    EXECUTE FUNCTION mcp.update_dlq_retry();

-- ==============================================================================
-- COMMENTS
-- ==============================================================================

COMMENT ON TABLE mcp.webhook_dlq IS 'Dead Letter Queue for failed webhook events that need manual review or retry';
COMMENT ON COLUMN mcp.webhook_dlq.event_id IS 'Original webhook event ID from webhook_events table';
COMMENT ON COLUMN mcp.webhook_dlq.event_type IS 'Type of webhook event (e.g., github.push, github.pull_request)';
COMMENT ON COLUMN mcp.webhook_dlq.payload IS 'Original webhook payload';
COMMENT ON COLUMN mcp.webhook_dlq.error_message IS 'Error message from the last processing attempt';
COMMENT ON COLUMN mcp.webhook_dlq.retry_count IS 'Number of retry attempts made';
COMMENT ON COLUMN mcp.webhook_dlq.last_retry_at IS 'Timestamp of the last retry attempt';
COMMENT ON COLUMN mcp.webhook_dlq.status IS 'Current status: pending, retrying, failed, or resolved';
COMMENT ON COLUMN mcp.webhook_dlq.metadata IS 'Additional metadata about the event or processing attempts';

COMMIT;