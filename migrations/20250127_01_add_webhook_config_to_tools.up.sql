-- Migration: Add webhook configuration to dynamic tools
-- Version: 20250127_01
-- Description: Adds webhook configuration column to tool_configurations table and creates webhook event tables

-- Insert migration record
INSERT INTO migration_metadata (version, description) 
VALUES ('20250127_01', 'Add webhook configuration to dynamic tools');

-- 1. Add webhook_config column to tool_configurations table
ALTER TABLE tool_configurations 
ADD COLUMN webhook_config JSONB DEFAULT NULL;

-- 2. Add provider column if not exists
ALTER TABLE tool_configurations 
ADD COLUMN IF NOT EXISTS provider VARCHAR(100);

-- 3. Add passthrough_config column if not exists
ALTER TABLE tool_configurations 
ADD COLUMN IF NOT EXISTS passthrough_config JSONB DEFAULT NULL;

-- 4. Create webhook_events table
CREATE TABLE webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    event_type VARCHAR(100),
    payload JSONB NOT NULL,
    headers JSONB,
    source_ip VARCHAR(45),
    received_at TIMESTAMP DEFAULT NOW(),
    processed_at TIMESTAMP,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN (
        'pending', 'processing', 'processed', 'failed', 'ignored'
    )),
    error TEXT,
    metadata JSONB DEFAULT '{}',
    FOREIGN KEY (tool_id) REFERENCES tool_configurations(id) ON DELETE CASCADE
);

-- Create indexes for webhook_events
CREATE INDEX idx_webhook_events_tool_time ON webhook_events(tool_id, received_at DESC);
CREATE INDEX idx_webhook_events_tenant_time ON webhook_events(tenant_id, received_at DESC);
CREATE INDEX idx_webhook_events_status ON webhook_events(status, received_at) WHERE status IN ('pending', 'processing');
CREATE INDEX idx_webhook_events_type ON webhook_events(event_type, received_at DESC);

-- 5. Create webhook_event_logs table for processing history
CREATE TABLE webhook_event_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID REFERENCES webhook_events(id) ON DELETE CASCADE,
    action VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL,
    message TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for webhook_event_logs
CREATE INDEX idx_webhook_logs_event ON webhook_event_logs(event_id, created_at DESC);

-- 6. Update tool_configurations to add webhook-related constraints
ALTER TABLE tool_configurations 
ADD CONSTRAINT chk_webhook_config CHECK (
    webhook_config IS NULL OR (
        webhook_config ? 'enabled' AND 
        webhook_config ? 'auth_type'
    )
);

-- 7. Create function to cleanup old webhook events
CREATE OR REPLACE FUNCTION cleanup_old_webhook_events(days_to_keep INT DEFAULT 7)
RETURNS void AS $$
BEGIN
    DELETE FROM webhook_events 
    WHERE received_at < NOW() - INTERVAL '1 day' * days_to_keep
    AND status IN ('processed', 'ignored');
END;
$$ LANGUAGE plpgsql;

-- 8. Create function to update webhook event status
CREATE OR REPLACE FUNCTION update_webhook_event_status(
    p_event_id UUID,
    p_status VARCHAR(20),
    p_error TEXT DEFAULT NULL
)
RETURNS void AS $$
BEGIN
    UPDATE webhook_events 
    SET 
        status = p_status,
        processed_at = CASE WHEN p_status IN ('processed', 'failed', 'ignored') THEN NOW() ELSE processed_at END,
        error = p_error
    WHERE id = p_event_id;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions (adjust as needed for your user/role structure)
-- GRANT ALL ON webhook_events, webhook_event_logs TO mcp_app_user;
-- GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO mcp_app_user;