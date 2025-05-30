-- Create API keys table for authentication
CREATE TABLE IF NOT EXISTS api_keys (
    key VARCHAR(255) PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    scopes TEXT[] DEFAULT '{}',
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used TIMESTAMP,
    active BOOLEAN NOT NULL DEFAULT true,
    
    -- Indexes for performance
    INDEX idx_api_keys_tenant_id (tenant_id),
    INDEX idx_api_keys_user_id (user_id),
    INDEX idx_api_keys_active (active),
    INDEX idx_api_keys_expires_at (expires_at)
);

-- Add comment
COMMENT ON TABLE api_keys IS 'API keys for authentication and authorization';
COMMENT ON COLUMN api_keys.key IS 'The API key string (should be hashed in production)';
COMMENT ON COLUMN api_keys.scopes IS 'Array of permission scopes granted to this key';
COMMENT ON COLUMN api_keys.expires_at IS 'Optional expiration timestamp';
COMMENT ON COLUMN api_keys.last_used IS 'Timestamp of last usage for audit purposes';