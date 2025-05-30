-- Create table for storing user tool credentials (optional feature)
CREATE TABLE IF NOT EXISTS user_tool_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL,
    tenant_id VARCHAR(255) NOT NULL,
    tool_name VARCHAR(100) NOT NULL,
    credential_type VARCHAR(50) DEFAULT 'pat',
    encrypted_token TEXT NOT NULL,
    token_hint VARCHAR(20), -- Last 4 chars for identification
    base_url VARCHAR(500), -- For self-hosted instances
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_by VARCHAR(255),
    
    -- Ensure one credential per tool per user per tenant
    CONSTRAINT unique_user_tool_credential UNIQUE(user_id, tenant_id, tool_name)
);

-- Create indexes for efficient queries
CREATE INDEX idx_user_tool_credentials_user ON user_tool_credentials(user_id);
CREATE INDEX idx_user_tool_credentials_tenant ON user_tool_credentials(tenant_id);
CREATE INDEX idx_user_tool_credentials_tool ON user_tool_credentials(tool_name);
CREATE INDEX idx_user_tool_credentials_expires ON user_tool_credentials(expires_at) WHERE expires_at IS NOT NULL;

-- Audit log for credential access
CREATE TABLE IF NOT EXISTS credential_access_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    credential_id UUID REFERENCES user_tool_credentials(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    tenant_id VARCHAR(255) NOT NULL,
    tool_name VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL, -- 'create', 'read', 'update', 'delete', 'use'
    accessed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ip_address INET,
    user_agent TEXT,
    success BOOLEAN DEFAULT true,
    error_message TEXT,
    metadata JSONB DEFAULT '{}'
);

-- Create indexes for audit queries
CREATE INDEX idx_credential_access_log_credential ON credential_access_log(credential_id);
CREATE INDEX idx_credential_access_log_user ON credential_access_log(user_id);
CREATE INDEX idx_credential_access_log_accessed ON credential_access_log(accessed_at);
CREATE INDEX idx_credential_access_log_action ON credential_access_log(action);

-- Function to update the updated_at timestamp
CREATE OR REPLACE FUNCTION update_user_tool_credentials_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update updated_at
CREATE TRIGGER update_user_tool_credentials_updated_at_trigger
    BEFORE UPDATE ON user_tool_credentials
    FOR EACH ROW
    EXECUTE FUNCTION update_user_tool_credentials_updated_at();

-- Function to clean up expired credentials
CREATE OR REPLACE FUNCTION cleanup_expired_credentials()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM user_tool_credentials
    WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP;
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Add comments for documentation
COMMENT ON TABLE user_tool_credentials IS 'Stores encrypted tool credentials for users (optional feature)';
COMMENT ON COLUMN user_tool_credentials.encrypted_token IS 'AES-256-GCM encrypted token';
COMMENT ON COLUMN user_tool_credentials.token_hint IS 'Last 4 characters of token for user identification';
COMMENT ON COLUMN user_tool_credentials.credential_type IS 'Type of credential: pat, oauth, basic, etc.';
COMMENT ON TABLE credential_access_log IS 'Audit log for all credential operations';