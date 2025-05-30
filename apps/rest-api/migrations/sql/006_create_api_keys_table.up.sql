-- Create API keys table for enhanced authentication system
CREATE TABLE IF NOT EXISTS api_keys (
    key VARCHAR(255) PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL DEFAULT 'default',
    user_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used TIMESTAMP,
    active BOOLEAN NOT NULL DEFAULT true
);

-- Create indexes for performance (PostgreSQL syntax)
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant ON api_keys(tenant_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_active_expires ON api_keys(active, expires_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);

-- Create trigger for updated_at
CREATE OR REPLACE FUNCTION update_api_keys_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER api_keys_updated_at_trigger
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION update_api_keys_updated_at();

-- Add comments
COMMENT ON TABLE api_keys IS 'API keys for enhanced authentication and authorization system';
COMMENT ON COLUMN api_keys.key IS 'The API key string (should be hashed in production)';
COMMENT ON COLUMN api_keys.scopes IS 'Array of permission scopes granted to this key';
COMMENT ON COLUMN api_keys.expires_at IS 'Optional expiration timestamp';
COMMENT ON COLUMN api_keys.last_used IS 'Timestamp of last usage for audit purposes';
COMMENT ON COLUMN api_keys.updated_at IS 'Automatically updated on each modification';