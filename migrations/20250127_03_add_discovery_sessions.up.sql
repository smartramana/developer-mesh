-- Migration: Add tool discovery sessions
-- Version: 20250127_03
-- Description: Creates table for managing tool discovery sessions

-- Insert migration record
INSERT INTO migration_metadata (version, description) 
VALUES ('20250127_03', 'Add tool discovery sessions table');

-- Create tool_discovery_sessions table
CREATE TABLE tool_discovery_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    session_id VARCHAR(255) UNIQUE NOT NULL,
    base_url VARCHAR(2048) NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN (
        'discovering', 'discovered', 'partial', 'failed', 'confirmed', 'expired'
    )),
    discovered_urls TEXT[],
    selected_url VARCHAR(2048),
    discovery_metadata JSONB DEFAULT '{}',
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL
);

-- Create indexes for performance
CREATE INDEX idx_discovery_sessions_tenant ON tool_discovery_sessions(tenant_id, created_at DESC);
CREATE INDEX idx_discovery_sessions_session ON tool_discovery_sessions(session_id);
CREATE INDEX idx_discovery_sessions_status ON tool_discovery_sessions(status, created_at DESC);
CREATE INDEX idx_discovery_sessions_expires ON tool_discovery_sessions(expires_at) WHERE status NOT IN ('confirmed', 'expired');

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_discovery_session_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for timestamp updates
CREATE TRIGGER discovery_sessions_update_timestamp
BEFORE UPDATE ON tool_discovery_sessions
FOR EACH ROW
EXECUTE FUNCTION update_discovery_session_timestamp();

-- Create function to expire old sessions
CREATE OR REPLACE FUNCTION expire_old_discovery_sessions()
RETURNS void AS $$
BEGIN
    UPDATE tool_discovery_sessions
    SET status = 'expired'
    WHERE expires_at < NOW() 
    AND status NOT IN ('confirmed', 'expired');
END;
$$ LANGUAGE plpgsql;

-- Create function to cleanup very old sessions
CREATE OR REPLACE FUNCTION cleanup_old_discovery_sessions(days_to_keep INT DEFAULT 30)
RETURNS void AS $$
BEGIN
    DELETE FROM tool_discovery_sessions
    WHERE created_at < NOW() - INTERVAL '1 day' * days_to_keep;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions (adjust as needed for your user/role structure)
-- GRANT ALL ON tool_discovery_sessions TO mcp_app_user;
-- GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO mcp_app_user;