-- Add discovery patterns table for learning from successful discoveries
CREATE TABLE IF NOT EXISTS tool_discovery_patterns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain VARCHAR(255) UNIQUE NOT NULL,
    successful_paths JSONB NOT NULL DEFAULT '[]',
    auth_method VARCHAR(50),
    api_format VARCHAR(50),
    last_updated TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    success_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Add index for domain lookups
CREATE INDEX idx_tool_discovery_patterns_domain ON tool_discovery_patterns(domain);

-- Add discovery hints to tool configurations
ALTER TABLE tool_configurations 
ADD COLUMN IF NOT EXISTS discovery_metadata JSONB DEFAULT '{}';

-- Add comments for documentation
COMMENT ON TABLE tool_discovery_patterns IS 'Stores successful discovery patterns for different domains to enable learning';
COMMENT ON COLUMN tool_discovery_patterns.domain IS 'The domain/host of the tool (e.g., api.github.com)';
COMMENT ON COLUMN tool_discovery_patterns.successful_paths IS 'Array of paths that successfully returned OpenAPI specs';
COMMENT ON COLUMN tool_discovery_patterns.auth_method IS 'The authentication method that worked (bearer, basic, apikey, etc)';
COMMENT ON COLUMN tool_discovery_patterns.api_format IS 'The API format discovered (openapi3, swagger2, custom_json, etc)';
COMMENT ON COLUMN tool_discovery_patterns.success_count IS 'Number of successful discoveries for this domain';