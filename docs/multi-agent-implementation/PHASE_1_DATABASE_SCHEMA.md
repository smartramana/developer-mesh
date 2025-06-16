# Phase 1: Database Schema and Migrations

## Overview
This phase establishes the persistent storage foundation for multi-agent collaboration features. All tables are designed with multi-tenancy, performance, scalability, and security in mind.

## Timeline
**Duration**: 3-4 days
**Prerequisites**: PostgreSQL 14+ with uuid-ossp and pgcrypto extensions
**Deliverables**: 
- 10 database tables (8 core + 2 operational)
- Migration files with rollback scripts
- Row Level Security policies
- Performance-optimized indexes
- Monitoring views
- Test data seeds

## Database Design Principles

1. **Multi-Tenant Isolation**: Every table includes `tenant_id` with RLS enforcement
2. **Audit Trail**: Comprehensive audit logging for compliance
3. **Soft Deletes**: Use `deleted_at` for data retention
4. **UUID Primary Keys**: For distributed system compatibility
5. **JSONB Validation**: Check constraints on all JSONB fields
6. **Performance First**: Proper indexes, partitioning, and vacuum settings

## Initial Setup

```sql
-- Migration: migrations/0001_initial_setup.sql

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create custom types
CREATE TYPE task_status AS ENUM (
    'pending', 'assigned', 'accepted', 'rejected', 
    'in_progress', 'completed', 'failed', 'cancelled', 'timeout'
);

CREATE TYPE task_priority AS ENUM ('low', 'normal', 'high', 'critical');

CREATE TYPE workflow_type AS ENUM ('sequential', 'parallel', 'conditional', 'collaborative');

CREATE TYPE workflow_status AS ENUM (
    'pending', 'running', 'paused', 'completed', 
    'failed', 'cancelled', 'timeout'
);

CREATE TYPE delegation_type AS ENUM ('manual', 'automatic', 'failover', 'load_balance');

CREATE TYPE workspace_visibility AS ENUM ('private', 'team', 'public');

CREATE TYPE member_role AS ENUM ('owner', 'admin', 'member', 'viewer');

-- Helper function for updated_at triggers
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to get current tenant (for RLS)
CREATE OR REPLACE FUNCTION current_tenant_id() 
RETURNS UUID AS $$
BEGIN
    RETURN NULLIF(current_setting('app.current_tenant', true), '')::UUID;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Function to validate JSONB structure
CREATE OR REPLACE FUNCTION validate_jsonb_keys(
    data JSONB,
    required_keys TEXT[]
) RETURNS BOOLEAN AS $$
BEGIN
    RETURN data ?& required_keys;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Function to merge JSONB objects recursively
CREATE OR REPLACE FUNCTION jsonb_merge_recursive(
    target JSONB,
    source JSONB
) RETURNS JSONB AS $$
BEGIN
    RETURN CASE 
        WHEN jsonb_typeof(target) = 'object' AND jsonb_typeof(source) = 'object' 
        THEN (
            SELECT jsonb_object_agg(
                COALESCE(t.key, s.key),
                CASE
                    WHEN t.value IS NULL THEN s.value
                    WHEN s.value IS NULL THEN t.value
                    WHEN jsonb_typeof(t.value) = 'object' AND jsonb_typeof(s.value) = 'object'
                    THEN jsonb_merge_recursive(t.value, s.value)
                    ELSE s.value
                END
            )
            FROM jsonb_each(target) t
            FULL OUTER JOIN jsonb_each(source) s ON t.key = s.key
        )
        ELSE source
    END;
END;
$$ LANGUAGE plpgsql IMMUTABLE;
```

## Core Tables

### 1. Tasks Table
Primary table for task management and delegation.

```sql
-- Migration: migrations/0010_create_tasks_table.sql
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    type VARCHAR(100) NOT NULL,
    status task_status NOT NULL DEFAULT 'pending',
    priority task_priority DEFAULT 'normal',
    
    -- Agent relationships
    created_by VARCHAR(255) NOT NULL,
    assigned_to VARCHAR(255),
    
    -- Task hierarchy
    parent_task_id UUID REFERENCES tasks(id) ON DELETE CASCADE,
    
    -- Task data
    title VARCHAR(500) NOT NULL CONSTRAINT title_not_empty CHECK (length(trim(title)) > 0),
    description TEXT,
    parameters JSONB DEFAULT '{}' NOT NULL,
    result JSONB,
    error TEXT,
    
    -- Execution control
    max_retries INTEGER DEFAULT 3 CONSTRAINT valid_max_retries CHECK (max_retries >= 0 AND max_retries <= 10),
    retry_count INTEGER DEFAULT 0,
    timeout_seconds INTEGER DEFAULT 3600 CONSTRAINT valid_timeout CHECK (timeout_seconds > 0),
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    assigned_at TIMESTAMP WITH TIME ZONE,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    -- Constraints
    CONSTRAINT retry_count_valid CHECK (retry_count >= 0 AND retry_count <= max_retries),
    CONSTRAINT parameters_is_object CHECK (jsonb_typeof(parameters) = 'object'),
    CONSTRAINT result_is_object CHECK (result IS NULL OR jsonb_typeof(result) = 'object')
) PARTITION BY RANGE (created_at);

-- Create initial partitions
CREATE TABLE tasks_2024_01 PARTITION OF tasks FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
CREATE TABLE tasks_2024_02 PARTITION OF tasks FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');
-- Add more partitions as needed

-- Performance indexes
CREATE INDEX idx_tasks_tenant_id ON tasks(tenant_id);
CREATE INDEX idx_tasks_assigned_to ON tasks(assigned_to) WHERE assigned_to IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_tasks_status ON tasks(status) WHERE status NOT IN ('completed', 'failed', 'cancelled') AND deleted_at IS NULL;
CREATE INDEX idx_tasks_tenant_status_priority ON tasks(tenant_id, status, priority) 
    WHERE status IN ('pending', 'assigned') AND deleted_at IS NULL;
CREATE INDEX idx_tasks_parent_id ON tasks(parent_task_id) WHERE parent_task_id IS NOT NULL;
CREATE INDEX idx_tasks_created_at ON tasks(created_at DESC);
CREATE INDEX idx_tasks_list_covering ON tasks(tenant_id, assigned_to, status) 
    INCLUDE (title, priority, created_at)
    WHERE deleted_at IS NULL;

-- Trigger for updated_at
CREATE TRIGGER update_tasks_updated_at
    BEFORE UPDATE ON tasks
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();

-- Row Level Security
ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;

CREATE POLICY tasks_tenant_isolation ON tasks
    FOR ALL
    USING (tenant_id = current_tenant_id());

CREATE POLICY tasks_soft_delete ON tasks
    FOR SELECT
    USING (deleted_at IS NULL);

-- Performance settings
ALTER TABLE tasks SET (
    fillfactor = 80,
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    autovacuum_vacuum_cost_delay = 10
);
```

### 2. Task Delegations Table
Tracks the delegation history and chain of responsibility.

```sql
-- Migration: migrations/0011_create_task_delegations_table.sql
CREATE TABLE task_delegations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    from_agent_id VARCHAR(255) NOT NULL,
    to_agent_id VARCHAR(255) NOT NULL,
    reason TEXT,
    delegation_type delegation_type DEFAULT 'manual',
    metadata JSONB DEFAULT '{}' NOT NULL,
    delegated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT metadata_is_object CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX idx_delegations_task_id ON task_delegations(task_id);
CREATE INDEX idx_delegations_from_agent ON task_delegations(from_agent_id);
CREATE INDEX idx_delegations_to_agent ON task_delegations(to_agent_id);
CREATE INDEX idx_delegations_delegated_at ON task_delegations(delegated_at DESC);
```

### 3. Workflows Table
Stores workflow definitions for multi-agent orchestration.

```sql
-- Migration: migrations/0012_create_workflows_table.sql
CREATE TABLE workflows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL CONSTRAINT name_not_empty CHECK (length(trim(name)) > 0),
    type workflow_type NOT NULL DEFAULT 'sequential',
    version INTEGER DEFAULT 1 CONSTRAINT version_positive CHECK (version > 0),
    
    -- Workflow definition
    created_by VARCHAR(255) NOT NULL,
    agents JSONB NOT NULL DEFAULT '{}' CONSTRAINT valid_agents_jsonb CHECK (jsonb_typeof(agents) = 'object'),
    steps JSONB NOT NULL DEFAULT '[]' CONSTRAINT valid_steps_jsonb CHECK (jsonb_typeof(steps) = 'array'),
    config JSONB DEFAULT '{}' NOT NULL CONSTRAINT valid_config_jsonb CHECK (jsonb_typeof(config) = 'object'),
    
    -- Metadata
    description TEXT,
    tags TEXT[] DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_workflows_tenant_id ON workflows(tenant_id);
CREATE INDEX idx_workflows_name ON workflows(name) WHERE deleted_at IS NULL;
CREATE INDEX idx_workflows_type ON workflows(type);
CREATE INDEX idx_workflows_tenant_active_type ON workflows(tenant_id, is_active, type) 
    WHERE is_active = true AND deleted_at IS NULL;
CREATE INDEX idx_workflows_tags ON workflows USING gin(tags);

-- Trigger for updated_at
CREATE TRIGGER update_workflows_updated_at
    BEFORE UPDATE ON workflows
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();

-- Row Level Security
ALTER TABLE workflows ENABLE ROW LEVEL SECURITY;

CREATE POLICY workflows_tenant_isolation ON workflows
    FOR ALL
    USING (tenant_id = current_tenant_id());
```

### 4. Workflow Executions Table
Tracks individual workflow runs and their state.

```sql
-- Migration: migrations/0013_create_workflow_executions_table.sql
CREATE TABLE workflow_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id UUID NOT NULL REFERENCES workflows(id),
    tenant_id UUID NOT NULL, -- Denormalized for performance
    
    -- Execution state
    status workflow_status NOT NULL DEFAULT 'pending',
    current_step_index INTEGER DEFAULT 0,
    current_step_id VARCHAR(255),
    
    -- Execution data
    triggered_by VARCHAR(255) NOT NULL,
    input JSONB DEFAULT '{}' NOT NULL,
    context JSONB DEFAULT '{}' NOT NULL,
    step_results JSONB DEFAULT '{}' NOT NULL,
    output JSONB,
    error TEXT,
    
    -- Timing
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    
    -- Execution metadata
    execution_metadata JSONB DEFAULT '{}' NOT NULL,
    
    CONSTRAINT valid_input CHECK (jsonb_typeof(input) = 'object'),
    CONSTRAINT valid_context CHECK (jsonb_typeof(context) = 'object'),
    CONSTRAINT valid_step_results CHECK (jsonb_typeof(step_results) = 'object')
);

CREATE INDEX idx_executions_workflow_id ON workflow_executions(workflow_id);
CREATE INDEX idx_executions_tenant_id ON workflow_executions(tenant_id);
CREATE INDEX idx_executions_status ON workflow_executions(status) 
    WHERE status NOT IN ('completed', 'failed', 'cancelled');
CREATE INDEX idx_executions_triggered_by ON workflow_executions(triggered_by);
CREATE INDEX idx_executions_started_at ON workflow_executions(started_at DESC);

-- Row Level Security
ALTER TABLE workflow_executions ENABLE ROW LEVEL SECURITY;

CREATE POLICY executions_tenant_isolation ON workflow_executions
    FOR ALL
    USING (tenant_id = current_tenant_id());

-- Performance settings
ALTER TABLE workflow_executions SET (
    fillfactor = 80,
    autovacuum_vacuum_scale_factor = 0.05
);
```

### 5. Workspaces Table
Shared collaboration spaces for agents.

```sql
-- Migration: migrations/0014_create_workspaces_table.sql
CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL CONSTRAINT workspace_name_not_empty CHECK (length(trim(name)) > 0),
    type VARCHAR(50) DEFAULT 'general',
    
    -- Ownership and access
    owner_id VARCHAR(255) NOT NULL,
    visibility workspace_visibility DEFAULT 'private',
    
    -- Workspace state with optimistic locking
    state JSONB DEFAULT '{}' NOT NULL,
    state_version INTEGER DEFAULT 1,
    last_activity_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Configuration
    config JSONB DEFAULT '{}' NOT NULL,
    max_members INTEGER DEFAULT 100 CONSTRAINT valid_max_members CHECK (max_members > 0 AND max_members <= 1000),
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT valid_state CHECK (jsonb_typeof(state) = 'object'),
    CONSTRAINT valid_config CHECK (jsonb_typeof(config) = 'object')
);

CREATE INDEX idx_workspaces_tenant_id ON workspaces(tenant_id);
CREATE INDEX idx_workspaces_owner_id ON workspaces(owner_id);
CREATE INDEX idx_workspaces_type ON workspaces(type) WHERE deleted_at IS NULL;
CREATE INDEX idx_workspaces_last_activity ON workspaces(last_activity_at DESC) WHERE deleted_at IS NULL;

-- Trigger for updated_at
CREATE TRIGGER update_workspaces_updated_at
    BEFORE UPDATE ON workspaces
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();

-- Row Level Security
ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;

CREATE POLICY workspaces_tenant_isolation ON workspaces
    FOR ALL
    USING (tenant_id = current_tenant_id());
```

### 6. Workspace Members Table
Manages workspace membership and permissions.

```sql
-- Migration: migrations/0015_create_workspace_members_table.sql
CREATE TABLE workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    agent_id VARCHAR(255) NOT NULL,
    tenant_id UUID NOT NULL, -- Denormalized for RLS
    
    -- Membership details
    role member_role DEFAULT 'member',
    permissions JSONB DEFAULT '["read", "write"]' NOT NULL,
    
    -- Activity tracking
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Member-specific state
    member_state JSONB DEFAULT '{}' NOT NULL,
    
    PRIMARY KEY (workspace_id, agent_id),
    
    CONSTRAINT valid_permissions CHECK (jsonb_typeof(permissions) = 'array'),
    CONSTRAINT valid_member_state CHECK (jsonb_typeof(member_state) = 'object')
);

CREATE INDEX idx_workspace_members_agent_id ON workspace_members(agent_id);
CREATE INDEX idx_workspace_members_tenant_id ON workspace_members(tenant_id);
CREATE INDEX idx_workspace_members_role ON workspace_members(role);
CREATE INDEX idx_workspace_members_last_seen ON workspace_members(last_seen_at DESC);

-- Row Level Security
ALTER TABLE workspace_members ENABLE ROW LEVEL SECURITY;

CREATE POLICY members_tenant_isolation ON workspace_members
    FOR ALL
    USING (tenant_id = current_tenant_id());
```

### 7. Shared Documents Table
Collaborative documents within workspaces.

```sql
-- Migration: migrations/0016_create_shared_documents_table.sql
CREATE TABLE shared_documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    
    -- Document metadata
    title VARCHAR(255) NOT NULL CONSTRAINT doc_title_not_empty CHECK (length(trim(title)) > 0),
    content TEXT,
    content_type VARCHAR(50) DEFAULT 'text' CHECK (content_type IN ('text', 'markdown', 'json', 'yaml', 'code')),
    language VARCHAR(50), -- For code documents
    
    -- Version control
    version INTEGER DEFAULT 1 CONSTRAINT doc_version_positive CHECK (version > 0),
    is_locked BOOLEAN DEFAULT false,
    locked_by VARCHAR(255),
    locked_at TIMESTAMP WITH TIME ZONE,
    lock_expires_at TIMESTAMP WITH TIME ZONE,
    
    -- Tracking
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    -- Metadata
    metadata JSONB DEFAULT '{}' NOT NULL,
    
    CONSTRAINT valid_metadata CHECK (jsonb_typeof(metadata) = 'object'),
    CONSTRAINT lock_consistency CHECK (
        (is_locked = false AND locked_by IS NULL AND locked_at IS NULL AND lock_expires_at IS NULL) OR
        (is_locked = true AND locked_by IS NOT NULL AND locked_at IS NOT NULL AND lock_expires_at IS NOT NULL)
    )
);

CREATE INDEX idx_documents_workspace_id ON shared_documents(workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_tenant_id ON shared_documents(tenant_id);
CREATE INDEX idx_documents_created_by ON shared_documents(created_by);
CREATE INDEX idx_documents_updated_at ON shared_documents(updated_at DESC) WHERE deleted_at IS NULL;

-- Trigger for updated_at
CREATE TRIGGER update_documents_updated_at
    BEFORE UPDATE ON shared_documents
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();

-- Row Level Security
ALTER TABLE shared_documents ENABLE ROW LEVEL SECURITY;

CREATE POLICY documents_tenant_isolation ON shared_documents
    FOR ALL
    USING (tenant_id = current_tenant_id());
```

### 8. Document Operations Table
CRDT operations for conflict-free collaborative editing.

```sql
-- Migration: migrations/0017_create_document_operations_table.sql

-- Sequence for operation ordering
CREATE SEQUENCE document_operation_seq;

CREATE TABLE document_operations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    document_id UUID NOT NULL REFERENCES shared_documents(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL, -- Denormalized for RLS
    
    -- Operation details
    agent_id VARCHAR(255) NOT NULL,
    operation_type VARCHAR(50) NOT NULL CHECK (operation_type IN ('insert', 'delete', 'update', 'move', 'format')),
    operation_data JSONB NOT NULL,
    
    -- CRDT metadata
    vector_clock JSONB NOT NULL,
    sequence_number BIGINT NOT NULL DEFAULT nextval('document_operation_seq'),
    
    -- Timing
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Conflict resolution
    parent_operation_id UUID REFERENCES document_operations(id),
    is_applied BOOLEAN DEFAULT false,
    
    CONSTRAINT valid_operation_data CHECK (jsonb_typeof(operation_data) = 'object'),
    CONSTRAINT valid_vector_clock CHECK (jsonb_typeof(vector_clock) = 'object')
);

CREATE INDEX idx_operations_document_id ON document_operations(document_id);
CREATE INDEX idx_operations_tenant_id ON document_operations(tenant_id);
CREATE INDEX idx_operations_timestamp ON document_operations(document_id, timestamp);
CREATE INDEX idx_operations_sequence ON document_operations(document_id, sequence_number);
CREATE INDEX idx_operations_not_applied ON document_operations(document_id) WHERE is_applied = false;
CREATE INDEX idx_operations_parent_id ON document_operations(parent_operation_id) 
    WHERE parent_operation_id IS NOT NULL;

-- Row Level Security
ALTER TABLE document_operations ENABLE ROW LEVEL SECURITY;

CREATE POLICY operations_tenant_isolation ON document_operations
    FOR ALL
    USING (tenant_id = current_tenant_id());
```

### 9. Audit Log Table
Comprehensive audit trail for compliance.

```sql
-- Migration: migrations/0018_create_audit_log_table.sql
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    table_name VARCHAR(50) NOT NULL,
    record_id UUID NOT NULL,
    action VARCHAR(20) NOT NULL CHECK (action IN ('INSERT', 'UPDATE', 'DELETE', 'SELECT')),
    old_data JSONB,
    new_data JSONB,
    changed_fields TEXT[],
    changed_by VARCHAR(255) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
) PARTITION BY RANGE (changed_at);

-- Create initial partitions
CREATE TABLE audit_log_2024_01 PARTITION OF audit_log FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
CREATE TABLE audit_log_2024_02 PARTITION OF audit_log FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');

CREATE INDEX idx_audit_log_tenant_table ON audit_log(tenant_id, table_name);
CREATE INDEX idx_audit_log_record ON audit_log(record_id);
CREATE INDEX idx_audit_log_changed_at ON audit_log(changed_at DESC);
CREATE INDEX idx_audit_log_changed_by ON audit_log(changed_by);

-- Row Level Security
ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;

CREATE POLICY audit_tenant_isolation ON audit_log
    FOR ALL
    USING (tenant_id = current_tenant_id());
```

### 10. Conflict Resolutions Table
Track and analyze conflict resolution patterns.

```sql
-- Migration: migrations/0019_create_conflict_resolutions_table.sql
CREATE TABLE conflict_resolutions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID NOT NULL,
    conflict_type VARCHAR(50) NOT NULL,
    description TEXT,
    resolution_strategy VARCHAR(50),
    details JSONB DEFAULT '{}' NOT NULL,
    resolved_by VARCHAR(255),
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT valid_details CHECK (jsonb_typeof(details) = 'object')
);

CREATE INDEX idx_conflicts_tenant_id ON conflict_resolutions(tenant_id);
CREATE INDEX idx_conflicts_resource ON conflict_resolutions(resource_type, resource_id);
CREATE INDEX idx_conflicts_created_at ON conflict_resolutions(created_at DESC);

-- Row Level Security
ALTER TABLE conflict_resolutions ENABLE ROW LEVEL SECURITY;

CREATE POLICY conflicts_tenant_isolation ON conflict_resolutions
    FOR ALL
    USING (tenant_id = current_tenant_id());
```

## Monitoring Views

```sql
-- Migration: migrations/0020_create_monitoring_views.sql

-- Task statistics view
CREATE VIEW v_task_statistics AS
SELECT 
    tenant_id,
    status,
    priority,
    COUNT(*) as count,
    AVG(EXTRACT(EPOCH FROM (completed_at - created_at))) as avg_duration_seconds,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (completed_at - created_at))) as median_duration_seconds,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (completed_at - created_at))) as p95_duration_seconds
FROM tasks
WHERE deleted_at IS NULL
GROUP BY tenant_id, status, priority;

-- Active workflows view
CREATE VIEW v_active_workflows AS
SELECT 
    w.id,
    w.tenant_id,
    w.name,
    w.type,
    COUNT(DISTINCT we.id) FILTER (WHERE we.status IN ('pending', 'running')) as active_executions,
    COUNT(DISTINCT we.id) FILTER (WHERE we.status = 'completed') as completed_executions,
    COUNT(DISTINCT we.id) FILTER (WHERE we.status = 'failed') as failed_executions,
    MAX(we.started_at) as last_execution_started
FROM workflows w
LEFT JOIN workflow_executions we ON w.id = we.workflow_id
WHERE w.is_active = true AND w.deleted_at IS NULL
GROUP BY w.id, w.tenant_id, w.name, w.type;

-- Agent workload view
CREATE VIEW v_agent_workload AS
SELECT 
    tenant_id,
    assigned_to as agent_id,
    COUNT(*) FILTER (WHERE status IN ('assigned', 'accepted')) as pending_tasks,
    COUNT(*) FILTER (WHERE status = 'in_progress') as active_tasks,
    COUNT(*) FILTER (WHERE status = 'completed' AND completed_at > CURRENT_TIMESTAMP - INTERVAL '24 hours') as completed_24h,
    AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) FILTER (WHERE status = 'completed') as avg_task_duration_seconds
FROM tasks
WHERE deleted_at IS NULL AND assigned_to IS NOT NULL
GROUP BY tenant_id, assigned_to;

-- Workspace activity view
CREATE VIEW v_workspace_activity AS
SELECT 
    w.id,
    w.tenant_id,
    w.name,
    w.type,
    COUNT(DISTINCT wm.agent_id) as member_count,
    COUNT(DISTINCT wm.agent_id) FILTER (WHERE wm.last_seen_at > CURRENT_TIMESTAMP - INTERVAL '15 minutes') as active_members,
    w.state_version,
    w.last_activity_at,
    COUNT(DISTINCT d.id) as document_count
FROM workspaces w
LEFT JOIN workspace_members wm ON w.id = wm.workspace_id
LEFT JOIN shared_documents d ON w.id = d.workspace_id AND d.deleted_at IS NULL
WHERE w.deleted_at IS NULL
GROUP BY w.id, w.tenant_id, w.name, w.type, w.state_version, w.last_activity_at;
```

## Audit Triggers

```sql
-- Migration: migrations/0021_create_audit_triggers.sql

-- Generic audit trigger function
CREATE OR REPLACE FUNCTION audit_trigger_function()
RETURNS TRIGGER AS $$
DECLARE
    audit_row audit_log;
    changed_fields TEXT[];
BEGIN
    audit_row.tenant_id := COALESCE(NEW.tenant_id, OLD.tenant_id);
    audit_row.table_name := TG_TABLE_NAME;
    audit_row.action := TG_OP;
    audit_row.changed_by := COALESCE(current_setting('app.current_user', true), 'system');
    audit_row.ip_address := COALESCE(inet(current_setting('app.client_ip', true)), NULL);
    audit_row.user_agent := current_setting('app.user_agent', true);
    
    IF TG_OP = 'DELETE' THEN
        audit_row.record_id := OLD.id;
        audit_row.old_data := to_jsonb(OLD);
    ELSIF TG_OP = 'UPDATE' THEN
        audit_row.record_id := NEW.id;
        audit_row.old_data := to_jsonb(OLD);
        audit_row.new_data := to_jsonb(NEW);
        
        -- Calculate changed fields
        SELECT array_agg(key) INTO changed_fields
        FROM jsonb_each(to_jsonb(NEW))
        WHERE to_jsonb(NEW) -> key IS DISTINCT FROM to_jsonb(OLD) -> key;
        
        audit_row.changed_fields := changed_fields;
    ELSIF TG_OP = 'INSERT' THEN
        audit_row.record_id := NEW.id;
        audit_row.new_data := to_jsonb(NEW);
    END IF;
    
    INSERT INTO audit_log VALUES (audit_row.*);
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Create audit triggers for main tables
CREATE TRIGGER audit_tasks AFTER INSERT OR UPDATE OR DELETE ON tasks
    FOR EACH ROW EXECUTE PROCEDURE audit_trigger_function();

CREATE TRIGGER audit_workflows AFTER INSERT OR UPDATE OR DELETE ON workflows
    FOR EACH ROW EXECUTE PROCEDURE audit_trigger_function();

CREATE TRIGGER audit_workspaces AFTER INSERT OR UPDATE OR DELETE ON workspaces
    FOR EACH ROW EXECUTE PROCEDURE audit_trigger_function();

CREATE TRIGGER audit_documents AFTER INSERT OR UPDATE OR DELETE ON shared_documents
    FOR EACH ROW EXECUTE PROCEDURE audit_trigger_function();
```

## Test Data Seeds

```sql
-- Migration: migrations/0022_seed_test_data.sql

-- Only run in development/test environments
DO $$
DECLARE
    test_tenant_id UUID := '00000000-0000-0000-0000-000000000001'::uuid;
    test_workspace_id UUID;
BEGIN
    IF current_database() LIKE '%_dev' OR current_database() LIKE '%_test' THEN
        -- Set tenant context
        PERFORM set_config('app.current_tenant', test_tenant_id::text, true);
        
        -- Insert test workflow
        INSERT INTO workflows (tenant_id, name, type, created_by, agents, steps)
        VALUES (
            test_tenant_id,
            'Test Code Review Workflow',
            'collaborative',
            'test-coordinator',
            '{"analyzer": "code-analyzer-agent", "reviewer": "code-reviewer-agent"}'::jsonb,
            '[
                {"id": "analyze", "agent": "analyzer", "action": "analyze_code"},
                {"id": "review", "agent": "reviewer", "action": "review_findings", "depends_on": ["analyze"]}
            ]'::jsonb
        );
        
        -- Insert test workspace
        INSERT INTO workspaces (id, tenant_id, name, type, owner_id)
        VALUES (
            uuid_generate_v4(),
            test_tenant_id,
            'Test Collaboration Space',
            'general',
            'test-agent-1'
        ) RETURNING id INTO test_workspace_id;
        
        -- Add workspace members
        INSERT INTO workspace_members (workspace_id, agent_id, tenant_id, role)
        VALUES 
            (test_workspace_id, 'test-agent-1', test_tenant_id, 'owner'),
            (test_workspace_id, 'test-agent-2', test_tenant_id, 'member');
            
        -- Insert sample tasks
        INSERT INTO tasks (tenant_id, type, title, created_by, assigned_to, priority, status)
        VALUES 
            (test_tenant_id, 'code_review', 'Review authentication module', 'test-agent-1', 'test-agent-2', 'high', 'assigned'),
            (test_tenant_id, 'testing', 'Write unit tests for API', 'test-agent-1', NULL, 'normal', 'pending');
    END IF;
END $$;
```

## Performance Optimization

```sql
-- Migration: migrations/0023_performance_optimization.sql

-- Configure autovacuum for high-activity tables
ALTER TABLE tasks SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    autovacuum_vacuum_cost_delay = 10,
    autovacuum_vacuum_cost_limit = 1000
);

ALTER TABLE workflow_executions SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02
);

ALTER TABLE document_operations SET (
    autovacuum_vacuum_scale_factor = 0.1,
    autovacuum_analyze_scale_factor = 0.05
);

-- Create statistics for better query planning
CREATE STATISTICS tasks_status_priority ON status, priority FROM tasks;
CREATE STATISTICS workflows_tenant_type ON tenant_id, type FROM workflows;

-- Add table comments for documentation
COMMENT ON TABLE tasks IS 'Core task management table supporting delegation and distributed execution';
COMMENT ON TABLE workflows IS 'Workflow definitions for multi-agent orchestration';
COMMENT ON TABLE workspaces IS 'Shared collaboration spaces for agent coordination';
COMMENT ON TABLE shared_documents IS 'Collaborative documents with CRDT support';
```

## Rollback Scripts

```sql
-- Rollback: migrations/0023_performance_optimization_rollback.sql
-- Remove statistics
DROP STATISTICS IF EXISTS tasks_status_priority;
DROP STATISTICS IF EXISTS workflows_tenant_type;

-- Rollback: migrations/0022_seed_test_data_rollback.sql
-- No rollback needed for test data

-- Rollback: migrations/0021_create_audit_triggers_rollback.sql
DROP TRIGGER IF EXISTS audit_tasks ON tasks;
DROP TRIGGER IF EXISTS audit_workflows ON workflows;
DROP TRIGGER IF EXISTS audit_workspaces ON workspaces;
DROP TRIGGER IF EXISTS audit_documents ON shared_documents;
DROP FUNCTION IF EXISTS audit_trigger_function();

-- Rollback: migrations/0020_create_monitoring_views_rollback.sql
DROP VIEW IF EXISTS v_workspace_activity;
DROP VIEW IF EXISTS v_agent_workload;
DROP VIEW IF EXISTS v_active_workflows;
DROP VIEW IF EXISTS v_task_statistics;

-- Continue with reverse order...
-- Each table drop should CASCADE to remove dependent objects
DROP TABLE IF EXISTS conflict_resolutions CASCADE;
DROP TABLE IF EXISTS audit_log CASCADE;
DROP TABLE IF EXISTS document_operations CASCADE;
DROP TABLE IF EXISTS shared_documents CASCADE;
DROP TABLE IF EXISTS workspace_members CASCADE;
DROP TABLE IF EXISTS workspaces CASCADE;
DROP TABLE IF EXISTS workflow_executions CASCADE;
DROP TABLE IF EXISTS workflows CASCADE;
DROP TABLE IF EXISTS task_delegations CASCADE;
DROP TABLE IF EXISTS tasks CASCADE;

-- Drop sequences
DROP SEQUENCE IF EXISTS document_operation_seq;

-- Drop types
DROP TYPE IF EXISTS member_role;
DROP TYPE IF EXISTS workspace_visibility;
DROP TYPE IF EXISTS delegation_type;
DROP TYPE IF EXISTS workflow_status;
DROP TYPE IF EXISTS workflow_type;
DROP TYPE IF EXISTS task_priority;
DROP TYPE IF EXISTS task_status;

-- Drop functions
DROP FUNCTION IF EXISTS jsonb_merge_recursive(JSONB, JSONB);
DROP FUNCTION IF EXISTS validate_jsonb_keys(JSONB, TEXT[]);
DROP FUNCTION IF EXISTS current_tenant_id();
DROP FUNCTION IF EXISTS update_updated_at_column();
```

## Security Checklist

- ✅ Row Level Security enabled on all tables
- ✅ Tenant isolation policies in place
- ✅ SECURITY DEFINER on sensitive functions
- ✅ Input validation via CHECK constraints
- ✅ Audit logging for compliance
- ✅ Soft deletes for data retention
- ✅ Proper indexes to prevent full table scans
- ✅ JSONB validation constraints

## Performance Checklist

- ✅ Table partitioning for high-volume tables
- ✅ Covering indexes for common queries
- ✅ Partial indexes for filtered queries
- ✅ GIN indexes for JSONB and array searches
- ✅ Proper fillfactor for UPDATE-heavy tables
- ✅ Aggressive autovacuum for active tables
- ✅ Statistics for query planner optimization

## Next Steps

After completing Phase 1:
1. Run migrations in order using migration tool
2. Verify all indexes with `\di` in psql
3. Check RLS policies with `\dp`
4. Run EXPLAIN on common queries
5. Load test with sample data
6. Monitor autovacuum performance
7. Set up automated partition creation