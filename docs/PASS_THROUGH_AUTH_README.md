# Pass-Through Authentication

## Overview

Pass-through authentication allows IDE users to provide their own Personal Access Tokens (PATs) for backend tools (GitHub, Jira, SonarQube, etc.), ensuring that actions are performed with their actual permissions while maintaining security and audit compliance.

## Key Features

- **User Permissions**: Actions execute with user's actual permissions
- **Audit Compliance**: Backend tools see the real user, not a service account
- **Zero Trust**: Credentials are passed through, not stored by default
- **Backward Compatible**: Existing service account authentication continues to work
- **Multi-Tool Support**: Supports GitHub, Jira, SonarQube, Artifactory, Jenkins, GitLab, Bitbucket

## How It Works

```
IDE → MCP Server → Tool Adapter → Backend Tool
 ↓         ↓             ↓              ↓
PAT    Validates    Uses PAT      Sees real user
```

## Usage

### Basic Request with User Credentials

```bash
curl -X POST http://localhost:8080/api/v1/tools/github/actions/create_issue \
  -H "Authorization: Bearer $MCP_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "create_issue",
    "parameters": {
      "owner": "myorg",
      "repo": "myrepo",
      "title": "Test Issue",
      "body": "Created with my PAT"
    },
    "credentials": {
      "github": {
        "token": "ghp_yourPersonalAccessToken",
        "type": "pat"
      }
    }
  }'
```

### Multiple Tool Credentials

```json
{
  "action": "sync_issues",
  "parameters": { ... },
  "credentials": {
    "github": {
      "token": "ghp_githubToken",
      "type": "pat"
    },
    "jira": {
      "token": "jira_apiToken",
      "type": "basic",
      "username": "user@example.com"
    }
  }
}
```

### OAuth Token

```json
{
  "credentials": {
    "github": {
      "token": "gho_oauthToken",
      "type": "oauth",
      "expires_at": "2024-12-31T23:59:59Z"
    }
  }
}
```

### GitHub Enterprise

```json
{
  "credentials": {
    "github": {
      "token": "ghp_enterpriseToken",
      "type": "pat",
      "base_url": "https://github.enterprise.com/"
    }
  }
}
```

## Configuration

### Server Configuration

```yaml
# config.yaml
auth:
  passthrough:
    enabled: true
    fallback_to_service_account: true
    service_accounts:
      github:
        enabled: true
        token: "${GITHUB_SERVICE_TOKEN}"
```

### Environment Variables

```bash
# Required
export MCP_API_KEY="your-mcp-api-key"

# Optional - for service account fallback
export GITHUB_SERVICE_TOKEN="ghp_serviceAccountToken"
export SONARQUBE_SERVICE_TOKEN="sqp_serviceAccountToken"
```

## Security Features

1. **No Credential Logging**: Credentials are never logged, only sanitized hints
2. **Memory Clearing**: Credentials are cleared from memory after use
3. **Expiration Checking**: Expired credentials are automatically rejected
4. **HTTPS Support**: All communication should use HTTPS in production
5. **Audit Trail**: Optional database storage includes full audit logging

## Testing

### Unit Tests
```bash
go test ./pkg/auth -v
go test ./pkg/models -v
go test ./apps/mcp-server/internal/api/tools/github -v
```

### Integration Tests
```bash
go test ./apps/mcp-server/internal/api/handlers -v -run TestPassthrough
```

### Manual Testing
```bash
./scripts/test-passthrough-auth.sh
```

## Monitoring

The feature tracks authentication methods via metrics:

- `github_auth_method{method="user_credential",type="pat"}` - User PAT usage
- `github_auth_method{method="user_credential",type="oauth"}` - OAuth token usage
- `github_auth_method{method="service_account"}` - Service account fallback

## Troubleshooting

### Common Issues

1. **401 Unauthorized**
   - Verify token is valid and has required permissions
   - Check token hasn't expired
   - Ensure correct token format (ghp_ for PAT, gho_ for OAuth)

2. **Service Account Fallback Not Working**
   - Verify `fallback_to_service_account: true` in config
   - Check service account token is configured
   - Verify service account has required permissions

3. **Credentials Not Extracted**
   - Ensure request path matches `/tools/*/actions/*`
   - Verify JSON structure matches examples
   - Check Content-Type is `application/json`

### Debug Logging

Enable debug logging to see credential flow (without actual tokens):

```yaml
monitoring:
  logging:
    level: "debug"
```

## Migration Guide

### For IDE Developers

1. Update your MCP client to include credentials in requests
2. Store user tokens securely (use OS keychain/secret storage)
3. Handle 401 responses by prompting for credentials
4. Test credential validation before making actual requests

### For System Administrators

1. Deploy updated MCP server with pass-through auth support
2. Configure service account tokens for fallback
3. Monitor authentication metrics
4. Educate users about PAT management

## Best Practices

1. **Token Scopes**: Use minimal required permissions for tokens
2. **Token Rotation**: Regularly rotate both user and service tokens
3. **Secure Storage**: Never store tokens in code or config files
4. **Error Handling**: Provide clear error messages for auth failures
5. **Monitoring**: Track authentication patterns and failures

## Implementation Status

- ✅ Core credential models and types
- ✅ Context-based credential propagation
- ✅ Middleware for credential extraction
- ✅ GitHub adapter with pass-through support
- ✅ Service account fallback mechanism
- ✅ Metrics and monitoring
- ✅ Comprehensive test coverage
- ✅ Database migrations for optional storage
- ✅ Production-ready error handling

## Future Enhancements

- [ ] Token refresh for OAuth flows
- [ ] Credential caching with TTL
- [ ] Multi-factor authentication support
- [ ] Webhook signature validation with user tokens
- [ ] Fine-grained permission checking