# Multi-Organization Webhook Setup Guide

## Overview

Developer Mesh now supports webhooks from multiple GitHub organizations. Each organization can have its own webhook secret and configuration, managed through environment variables or database entries.

## Architecture

The multi-organization webhook support includes:

1. **Database Storage**: Each organization's webhook configuration is stored in the `webhook_configs` table
2. **Automatic Initialization**: Webhook secrets are loaded from environment variables on startup
3. **Dynamic Validation**: Each webhook is validated using the organization-specific secret
4. **Flexible Configuration**: Support for different allowed events per organization

## Adding a New Organization

### Method 1: Environment Variables (Recommended for K8s/Docker)

Add environment variables to your deployment configuration:

```bash
# For the default organization (developer-mesh)
GITHUB_WEBHOOK_SECRET=your-secret-here
MCP_GITHUB_ALLOWED_EVENTS=issues,issue_comment,pull_request,push,release

# For additional organizations
WEBHOOK_ORG_ACME_CORP_SECRET=acme-webhook-secret
WEBHOOK_ORG_ACME_CORP_EVENTS=push,pull_request

WEBHOOK_ORG_ANOTHER_ORG_SECRET=another-secret
WEBHOOK_ORG_ANOTHER_ORG_EVENTS=issues,pull_request
```

**Note**: Organization names in environment variables:
- Use uppercase with underscores
- Will be converted to lowercase with hyphens
- Example: `ACME_CORP` → `acme-corp`

### Method 2: Direct Database Insert

For manual configuration, insert directly into the database:

```sql
INSERT INTO webhook_configs (
    organization_name, 
    webhook_secret, 
    enabled, 
    allowed_events
) VALUES (
    'new-org-name',
    'your-webhook-secret-here',
    true,
    ARRAY['issues', 'issue_comment', 'pull_request', 'push', 'release']
);
```

### Method 3: Admin API (Future)

A REST API for webhook management is planned but not yet implemented.

## Deployment Configuration

### Kubernetes ConfigMap Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: webhook-config
data:
  # Default organization
  GITHUB_WEBHOOK_SECRET: "your-encrypted-secret"
  MCP_GITHUB_ALLOWED_EVENTS: "issues,issue_comment,pull_request,push,release"
  
  # Additional organizations
  WEBHOOK_ORG_PARTNER_COMPANY_SECRET: "partner-secret"
  WEBHOOK_ORG_PARTNER_COMPANY_EVENTS: "push,pull_request"
```

### Docker Compose Example

```yaml
services:
  rest-api:
    environment:
      # Default organization
      - GITHUB_WEBHOOK_SECRET=${GITHUB_WEBHOOK_SECRET}
      - MCP_GITHUB_ALLOWED_EVENTS=issues,issue_comment,pull_request,push,release
      
      # Additional organizations
      - WEBHOOK_ORG_CLIENT_A_SECRET=${CLIENT_A_WEBHOOK_SECRET}
      - WEBHOOK_ORG_CLIENT_A_EVENTS=issues,pull_request
```

## GitHub Webhook Configuration

For each organization:

1. Go to: `https://github.com/organizations/<org-name>/settings/hooks`
2. Click "Add webhook"
3. Configure:
   - **Payload URL**: `https://api.dev-mesh.io/api/webhooks/github`
   - **Content type**: `application/json`
   - **Secret**: Use the organization-specific secret
   - **Events**: Select events matching your configuration

## Security Considerations

1. **Secret Strength**: Use strong, unique secrets for each organization (32+ characters)
2. **Secret Rotation**: Regularly rotate webhook secrets
3. **Environment Isolation**: Never commit secrets to version control
4. **IP Validation**: Optional GitHub IP validation is available via `MCP_GITHUB_IP_VALIDATION`

## Troubleshooting

### Webhook Returns 401 Unauthorized

1. Check the organization name in the webhook payload matches your configuration
2. Verify the secret is correctly set in environment variables
3. Check application logs for the specific organization name being used

### Organization Not Found

The application logs will show:
```
GitHub webhook from unknown organization organization=<org-name>
```

This means you need to add configuration for that organization.

### Viewing Configured Organizations

Check application logs on startup:
```
Webhook configurations initialized count=3 organizations=[developer-mesh, acme-corp, partner-org]
```

## Implementation Details

### Database Schema

```sql
CREATE TABLE webhook_configs (
    id UUID PRIMARY KEY,
    organization_name VARCHAR(255) UNIQUE NOT NULL,
    webhook_secret TEXT NOT NULL,
    enabled BOOLEAN DEFAULT true,
    allowed_events TEXT[],
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### Webhook Processing Flow

1. GitHub sends webhook to `/api/webhooks/github`
2. Handler extracts organization from `repository.owner.login`
3. Database lookup for organization's webhook configuration
4. Validates signature using organization-specific secret
5. Checks if event type is allowed for the organization
6. Processes webhook if all validations pass

## Future Enhancements

- REST API for webhook configuration management
- Secret rotation with grace period
- Per-organization rate limiting
- Webhook event history and analytics
- Support for other webhook sources (GitLab, Bitbucket, etc.)