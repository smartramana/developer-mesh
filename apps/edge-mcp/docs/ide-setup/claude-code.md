# Claude Code Configuration with Pass-Through Authentication

## Prerequisites

1. DevMesh account with API key and tenant ID
2. Edge MCP installed and in PATH
3. Your personal access tokens for services you want to use (GitHub, AWS, etc.)

## Configuration

Create `.claude/mcp.json` in your project root:

```json
{
  "mcpServers": {
    "devmesh": {
      "command": "edge-mcp",
      "args": ["--port", "8082"],
      "env": {
        // Required: DevMesh Platform credentials
        "CORE_PLATFORM_URL": "${CORE_PLATFORM_URL}",
        "CORE_PLATFORM_API_KEY": "${CORE_PLATFORM_API_KEY}",
        "TENANT_ID": "${TENANT_ID}",
        
        // Optional: Your personal access tokens for pass-through auth
        "GITHUB_TOKEN": "${GITHUB_TOKEN}",
        "AWS_ACCESS_KEY_ID": "${AWS_ACCESS_KEY_ID}",
        "AWS_SECRET_ACCESS_KEY": "${AWS_SECRET_ACCESS_KEY}",
        "SLACK_TOKEN": "${SLACK_TOKEN}",
        "JIRA_TOKEN": "${JIRA_TOKEN}"
      }
    }
  }
}
```

## Environment Setup

Set these environment variables before starting Claude Code:

### Required: DevMesh Platform Credentials
```bash
export CORE_PLATFORM_URL="https://api.devmesh.ai"
export CORE_PLATFORM_API_KEY="your-devmesh-api-key"  # From DevMesh dashboard
export TENANT_ID="your-tenant-id"                    # From DevMesh dashboard
```

### Optional but Recommended: Personal Access Tokens

These tokens allow Edge MCP to perform actions as YOU, not as a service account:

```bash
# GitHub Personal Access Token
export GITHUB_TOKEN="ghp_your_personal_access_token"

# AWS Credentials (for your personal AWS account)
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
export AWS_REGION="us-east-1"  # Optional

# Other service tokens (as needed)
export SLACK_TOKEN="xoxb-your-slack-token"
export JIRA_TOKEN="your-jira-api-token"
export GITLAB_TOKEN="glpat-your-gitlab-token"
```

## How Pass-Through Authentication Works

1. **Without Personal Tokens**: Actions are performed using DevMesh's service credentials
   - PRs created by "DevMesh Bot"
   - AWS resources created by service account
   - Less personal attribution

2. **With Personal Tokens**: Actions are performed as YOU
   - PRs created by your GitHub account
   - AWS resources created in your account
   - Full audit trail with your identity
   - Respects your personal permissions/limits

## Verification

After restarting Claude Code:

1. Check Edge MCP logs for "Extracted passthrough authentication"
2. Test with a GitHub command - the PR should be created as you
3. Verify in GitHub/AWS/etc. that actions show your username

## Security Notes

- Tokens are only held in memory during your session
- Tokens are never logged or stored persistently
- Tokens are only sent to DevMesh Platform over TLS
- Each action is logged with full attribution

## Available Tools

Tools are dynamically discovered from your DevMesh tenant. With pass-through auth, you can use:

- **GitHub**: Create PRs, issues, and releases as yourself
- **AWS**: Manage resources in your personal or work AWS account
- **Slack**: Send messages as yourself (not as a bot)
- **Jira**: Create and update issues with your identity
- Plus any custom tools configured in your tenant

## Troubleshooting

### "Actions still showing as bot/service account"
- Verify your personal token is set: `echo $GITHUB_TOKEN`
- Check Edge MCP logs for "Found GitHub passthrough token"
- Ensure token has required scopes (repo, write, etc.)

### "Authentication failed"
- Verify personal token is valid and not expired
- Check token permissions match the action you're trying
- Some organizations may restrict personal tokens

### "Tool execution failed"
- Check if your personal account has access to the resource
- Verify you're not hitting personal rate limits
- Some tools may not support pass-through auth yet

## Token Management Best Practices

1. **Use minimal scopes**: Only grant permissions you need
2. **Rotate regularly**: Update tokens every 30-90 days
3. **Use separate tokens**: Don't reuse tokens across services
4. **Store securely**: Use a password manager or secure keychain
5. **Monitor usage**: Check your GitHub/AWS audit logs regularly

## Example: Creating a PR as Yourself

```json
// When Edge MCP executes this tool:
{
  "tool": "github.create_pr",
  "arguments": {
    "repo": "myorg/myrepo",
    "title": "Fix: Update documentation",
    "body": "This PR updates the README"
  }
}

// With GITHUB_TOKEN set, the PR will show:
// - Author: Your GitHub username
// - Signed commits (if GPG configured)
// - Your profile picture
// - Counts toward your contribution graph
```