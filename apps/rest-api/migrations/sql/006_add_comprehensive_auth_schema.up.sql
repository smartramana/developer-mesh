-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Tenant management
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    external_id VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tenants_external_id ON tenants(external_id);
CREATE INDEX idx_tenants_status ON tenants(status);

-- GitHub App installations
CREATE TABLE IF NOT EXISTS github_installations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    installation_id BIGINT NOT NULL,
    app_id BIGINT NOT NULL,
    account_login VARCHAR(255),
    account_type VARCHAR(50),
    account_id BIGINT,
    repository_selection VARCHAR(50),
    permissions JSONB DEFAULT '{}',
    events JSONB DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(app_id, installation_id)
);

CREATE INDEX idx_github_installations_lookup ON github_installations(app_id, installation_id);
CREATE INDEX idx_github_installations_tenant ON github_installations(tenant_id);

-- Service accounts
CREATE TABLE IF NOT EXISTS service_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    service_name VARCHAR(255) UNIQUE NOT NULL,
    api_key_hash VARCHAR(255) UNIQUE NOT NULL,
    permissions JSONB DEFAULT '{}',
    rate_limit_override INTEGER,
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),
    last_used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_service_accounts_api_key ON service_accounts(api_key_hash);
CREATE INDEX idx_service_accounts_status ON service_accounts(status);

-- API Keys
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key_hash VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('agent', 'external_app')),
    permissions JSONB DEFAULT '{}',
    rate_limit_override INTEGER,
    expires_at TIMESTAMP,
    last_used_at TIMESTAMP,
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_tenant ON api_keys(tenant_id);
CREATE INDEX idx_api_keys_status_expires ON api_keys(status, expires_at);

-- Audit log
CREATE TABLE IF NOT EXISTS auth_audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    auth_type VARCHAR(50) NOT NULL,
    principal_id VARCHAR(255) NOT NULL,
    principal_type VARCHAR(50) NOT NULL,
    tenant_id UUID REFERENCES tenants(id),
    action VARCHAR(100) NOT NULL,
    resource VARCHAR(255),
    ip_address INET,
    user_agent TEXT,
    success BOOLEAN NOT NULL,
    error_code VARCHAR(50),
    error_message TEXT,
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_auth_audit_timestamp ON auth_audit_log(timestamp);
CREATE INDEX idx_auth_audit_principal ON auth_audit_log(principal_id);
CREATE INDEX idx_auth_audit_tenant ON auth_audit_log(tenant_id);