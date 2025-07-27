-- Migration: Add tool executions tracking
-- Version: 20250127_04
-- Description: Creates table for tracking tool action executions

-- Insert migration record
INSERT INTO migration_metadata (version, description) 
VALUES ('20250127_04', 'Add tool executions tracking table');

-- Create tool_executions table
CREATE TABLE tool_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_config_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    action VARCHAR(255) NOT NULL,
    parameters JSONB,
    status VARCHAR(20) NOT NULL CHECK (status IN ('success', 'failed', 'timeout', 'cancelled')),
    result JSONB,
    error_message TEXT,
    response_time_ms INTEGER,
    request_id VARCHAR(255),
    user_id UUID,
    context_id VARCHAR(255),
    executed_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    metadata JSONB DEFAULT '{}',
    FOREIGN KEY (tool_config_id) REFERENCES tool_configurations(id) ON DELETE CASCADE
);

-- Create indexes for performance
CREATE INDEX idx_tool_executions_tool ON tool_executions(tool_config_id, executed_at DESC);
CREATE INDEX idx_tool_executions_tenant ON tool_executions(tenant_id, executed_at DESC);
CREATE INDEX idx_tool_executions_action ON tool_executions(action, executed_at DESC);
CREATE INDEX idx_tool_executions_status ON tool_executions(status, executed_at DESC);
CREATE INDEX idx_tool_executions_user ON tool_executions(user_id, executed_at DESC);
CREATE INDEX idx_tool_executions_context ON tool_executions(context_id, executed_at DESC);

-- Create materialized view for execution statistics
CREATE MATERIALIZED VIEW tool_execution_stats AS
SELECT 
    tool_config_id,
    tenant_id,
    action,
    DATE_TRUNC('hour', executed_at) as hour,
    COUNT(*) as total_executions,
    COUNT(*) FILTER (WHERE status = 'success') as successful_executions,
    COUNT(*) FILTER (WHERE status = 'failed') as failed_executions,
    AVG(response_time_ms) as avg_response_time_ms,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY response_time_ms) as median_response_time_ms,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY response_time_ms) as p95_response_time_ms,
    MAX(response_time_ms) as max_response_time_ms
FROM tool_executions
WHERE executed_at > NOW() - INTERVAL '7 days'
GROUP BY tool_config_id, tenant_id, action, hour;

-- Create index on materialized view
CREATE UNIQUE INDEX idx_tool_execution_stats_unique 
ON tool_execution_stats(tool_config_id, tenant_id, action, hour);

-- Create function to refresh stats
CREATE OR REPLACE FUNCTION refresh_tool_execution_stats()
RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY tool_execution_stats;
END;
$$ LANGUAGE plpgsql;

-- Create function to cleanup old executions
CREATE OR REPLACE FUNCTION cleanup_old_tool_executions(days_to_keep INT DEFAULT 30)
RETURNS void AS $$
BEGIN
    DELETE FROM tool_executions
    WHERE executed_at < NOW() - INTERVAL '1 day' * days_to_keep;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions (adjust as needed for your user/role structure)
-- GRANT ALL ON tool_executions TO mcp_app_user;
-- GRANT SELECT ON tool_execution_stats TO mcp_app_user;
-- GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO mcp_app_user;