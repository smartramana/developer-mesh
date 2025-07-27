-- Migration: Add OpenAPI spec caching
-- Version: 20250127_02
-- Description: Creates table for caching OpenAPI specifications to improve performance

-- Insert migration record
INSERT INTO migration_metadata (version, description) 
VALUES ('20250127_02', 'Add OpenAPI spec caching table');

-- Create openapi_cache table
CREATE TABLE openapi_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url VARCHAR(2048) NOT NULL,
    spec_hash VARCHAR(64) NOT NULL,
    spec_data JSONB NOT NULL,
    version VARCHAR(50),
    title VARCHAR(255),
    discovered_actions TEXT[],
    cache_expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    last_accessed_at TIMESTAMP DEFAULT NOW(),
    access_count INTEGER DEFAULT 1,
    UNIQUE(url, spec_hash)
);

-- Create indexes for performance
CREATE INDEX idx_openapi_cache_url ON openapi_cache(url);
CREATE INDEX idx_openapi_cache_expires ON openapi_cache(cache_expires_at) WHERE cache_expires_at > NOW();
CREATE INDEX idx_openapi_cache_accessed ON openapi_cache(last_accessed_at DESC);

-- Create function to update last accessed timestamp
CREATE OR REPLACE FUNCTION update_openapi_cache_access()
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_accessed_at = NOW();
    NEW.access_count = OLD.access_count + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for access tracking
CREATE TRIGGER openapi_cache_access_trigger
BEFORE UPDATE ON openapi_cache
FOR EACH ROW
WHEN (OLD.spec_data IS NOT DISTINCT FROM NEW.spec_data)
EXECUTE FUNCTION update_openapi_cache_access();

-- Create function to cleanup expired cache entries
CREATE OR REPLACE FUNCTION cleanup_expired_openapi_cache()
RETURNS void AS $$
BEGIN
    DELETE FROM openapi_cache 
    WHERE cache_expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- Grant permissions (adjust as needed for your user/role structure)
-- GRANT ALL ON openapi_cache TO mcp_app_user;
-- GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO mcp_app_user;