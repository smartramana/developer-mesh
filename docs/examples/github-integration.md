# GitHub Integration Example

This guide demonstrates how to use the DevOps MCP platform to interact with GitHub repositories, including performing operations like creating issues, managing pull requests, and responding to webhooks.

## Overview

DevOps MCP provides a standardized interface to GitHub operations, allowing you to:

1. Perform GitHub operations through a consistent API
2. Receive and process GitHub webhook events
3. Track repository activities and changes
4. Integrate GitHub operations with AI agent workflows

## Prerequisites

- A GitHub account and personal access token (or GitHub App credentials)
- DevOps MCP server running and configured
- Basic familiarity with GitHub's API concepts

## Example Implementation

### 1. Setup and Authentication

```python
import requests
import json
import os

# Set up API key
API_KEY = os.environ.get("MCP_API_KEY", "your-api-key")

# Set MCP base URL
MCP_BASE_URL = "http://localhost:8080/api/v1"

# Create headers with authentication
headers = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}
```

### 2. GitHub Repository Operations

Here's a class to interact with GitHub repositories through MCP:

```python
class GitHubClient:
    def __init__(self, base_url, headers):
        self.base_url = base_url
        self.headers = headers
        self.tool_path = f"{self.base_url}/tools/github/actions"
    
    def create_issue(self, owner, repo, title, body, labels=None, assignees=None):
        """Create a new issue in a GitHub repository"""
        data = {
            "owner": owner,
            "repo": repo,
            "title": title,
            "body": body,
            "labels": labels or [],
            "assignees": assignees or []
        }
        
        response = requests.post(
            f"{self.tool_path}/create_issue",
            headers=self.headers,
            data=json.dumps(data)
        )
        
        if response.status_code == 200:
            return response.json()
        else:
            print(f"Error creating issue: {response.text}")
            return None
    
    def list_issues(self, owner, repo, state="open", limit=10):
        """List issues in a GitHub repository"""
        data = {
            "owner": owner,
            "repo": repo,
            "state": state,
            "per_page": limit
        }
        
        response = requests.post(
            f"{self.tool_path}/list_issues",
            headers=self.headers,
            data=json.dumps(data)
        )
        
        if response.status_code == 200:
            return response.json()
        else:
            print(f"Error listing issues: {response.text}")
            return []
    
    def create_pull_request(self, owner, repo, title, body, head, base):
        """Create a new pull request"""
        data = {
            "owner": owner,
            "repo": repo,
            "title": title, 
            "body": body,
            "head": head,
            "base": base
        }
        
        response = requests.post(
            f"{self.tool_path}/create_pull_request",
            headers=self.headers,
            data=json.dumps(data)
        )
        
        if response.status_code == 200:
            return response.json()
        else:
            print(f"Error creating pull request: {response.text}")
            return None
    
    def get_pull_request(self, owner, repo, pull_number):
        """Get details of a specific pull request"""
        data = {
            "owner": owner,
            "repo": repo,
            "pull_number": pull_number
        }
        
        response = requests.post(
            f"{self.tool_path}/get_pull_request",
            headers=self.headers,
            data=json.dumps(data)
        )
        
        if response.status_code == 200:
            return response.json()
        else:
            print(f"Error getting pull request: {response.text}")
            return None
    
    def create_branch(self, owner, repo, branch, from_branch=None):
        """Create a new branch in a repository"""
        data = {
            "owner": owner,
            "repo": repo,
            "branch": branch
        }
        
        if from_branch:
            data["from_branch"] = from_branch
        
        response = requests.post(
            f"{self.tool_path}/create_branch",
            headers=self.headers,
            data=json.dumps(data)
        )
        
        if response.status_code == 200:
            return response.json()
        else:
            print(f"Error creating branch: {response.text}")
            return None
    
    def create_or_update_file(self, owner, repo, path, content, message, branch, sha=None):
        """Create or update a file in a repository"""
        data = {
            "owner": owner,
            "repo": repo,
            "path": path,
            "content": content,
            "message": message,
            "branch": branch
        }
        
        if sha:
            data["sha"] = sha
        
        response = requests.post(
            f"{self.tool_path}/create_or_update_file",
            headers=self.headers,
            data=json.dumps(data)
        )
        
        if response.status_code == 200:
            return response.json()
        else:
            print(f"Error creating/updating file: {response.text}")
            return None
```

### 3. Example: Automated PR Creation Workflow

This example shows how to automate creating a pull request with file changes:

```python
def automated_pr_workflow(github_client, owner, repo, feature_name, file_changes):
    """
    Automate a PR workflow:
    1. Create a new branch
    2. Make changes to files
    3. Create a pull request
    """
    base_branch = "main"  # or master, depending on repository
    feature_branch = f"feature/{feature_name}"
    
    # Create a new branch
    branch_result = github_client.create_branch(
        owner, 
        repo, 
        feature_branch, 
        from_branch=base_branch
    )
    
    if not branch_result:
        return None
    
    print(f"Created branch: {feature_branch}")
    
    # Update files
    for file_change in file_changes:
        # Get current file content if it exists
        file_sha = None
        
        # Update or create the file
        file_result = github_client.create_or_update_file(
            owner,
            repo,
            file_change["path"],
            file_change["content"],
            file_change["message"],
            feature_branch,
            sha=file_sha
        )
        
        if not file_result:
            print(f"Failed to update {file_change['path']}")
            return None
        
        print(f"Updated file: {file_change['path']}")
    
    # Create a pull request
    pr_result = github_client.create_pull_request(
        owner,
        repo,
        f"Feature: {feature_name}",
        f"This PR implements {feature_name}\n\nAutomatically generated by DevOps MCP",
        feature_branch,
        base_branch
    )
    
    if pr_result:
        print(f"Created PR #{pr_result['number']}: {pr_result['html_url']}")
    
    return pr_result
```

### 4. Example Usage: Bug Fix Workflow

```python
# Initialize the GitHub client
github = GitHubClient(MCP_BASE_URL, headers)

# Define bug fix changes
bug_fix_changes = [
    {
        "path": "src/app.js",
        "content": "// Fixed login bug\nfunction login(username, password) {\n  // Sanitize inputs\n  const cleanUsername = sanitizeInput(username);\n  return authService.authenticate(cleanUsername, password);\n}",
        "message": "Fix login function to sanitize usernames"
    },
    {
        "path": "src/utils.js",
        "content": "function sanitizeInput(input) {\n  return input.replace(/[<>&\"']/g, '');\n}",
        "message": "Add input sanitization utility function"
    }
]

# Execute the workflow
pr = automated_pr_workflow(
    github,
    "S-Corkum",
    "my-project",
    "fix-login-bug",
    bug_fix_changes
)

# Check the result
if pr:
    print(f"Bug fix PR created: {pr['html_url']}")
else:
    print("Failed to create bug fix PR")
```

### 5. Integration with Issue Tracking

Here's an example of how to track and manage issues:

```python
def track_and_update_issue(github_client, owner, repo, issue_number, update_message, status=None):
    """Track an issue and add updates to it"""
    # Get the issue details
    issue = github_client.get_issue(owner, repo, issue_number)
    if not issue:
        return None
    
    # Add a comment with the update
    comment_data = {
        "owner": owner,
        "repo": repo,
        "issue_number": issue_number,
        "body": update_message
    }
    
    response = requests.post(
        f"{github_client.tool_path}/add_issue_comment",
        headers=github_client.headers,
        data=json.dumps(comment_data)
    )
    
    # Update issue status if needed
    if status:
        update_data = {
            "owner": owner,
            "repo": repo,
            "issue_number": issue_number,
            "state": status
        }
        
        requests.post(
            f"{github_client.tool_path}/update_issue",
            headers=github_client.headers,
            data=json.dumps(update_data)
        )
    
    return response.json() if response.status_code == 200 else None
```

### 6. Listening for GitHub Webhooks

DevOps MCP can receive GitHub webhooks. Here's how to process them:

```python
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/webhook/github', methods=['POST'])
def github_webhook():
    # Get webhook payload
    payload = request.json
    
    # Check the event type
    event_type = request.headers.get('X-GitHub-Event')
    
    if event_type == 'issues':
        handle_issue_event(payload)
    elif event_type == 'pull_request':
        handle_pull_request_event(payload)
    elif event_type == 'push':
        handle_push_event(payload)
    
    return jsonify({'status': 'received'})

def handle_issue_event(payload):
    """Handle GitHub issue events"""
    action = payload.get('action')
    issue = payload.get('issue', {})
    repo = payload.get('repository', {})
    
    print(f"Issue #{issue.get('number')} {action} in {repo.get('full_name')}")
    
    # Forward to DevOps MCP for processing
    requests.post(
        f"{MCP_BASE_URL}/webhooks/github",
        headers=headers,
        json=payload
    )

# Similar handlers for pull_request and push events

if __name__ == '__main__':
    app.run(port=5000)
```

### 7. Complete GitHub Automation Example

Here's a more complex example that monitors issues and automatically creates branches and PRs:

```python
class GitHubAutomation:
    def __init__(self, github_client):
        self.github = github_client
    
    def handle_new_issue(self, issue_event):
        """Handle a new issue event"""
        if issue_event['action'] != 'opened':
            return
        
        issue = issue_event['issue']
        repo = issue_event['repository']
        
        # Check if the issue has an automation label
        labels = [label['name'] for label in issue.get('labels', [])]
        
        if 'bug' in labels and 'auto-fix' in labels:
            self.create_fix_branch(
                repo['owner']['login'],
                repo['name'],
                issue['number'],
                issue['title'],
                issue['body']
            )
    
    def create_fix_branch(self, owner, repo, issue_number, title, description):
        """Create a fix branch for an issue"""
        # Clean up branch name
        branch_name = f"fix/issue-{issue_number}-{title.replace(' ', '-').lower()}"
        branch_name = re.sub(r'[^a-z0-9-]', '', branch_name)[:50]  # Keep it clean and short
        
        # Create the branch
        branch_result = self.github.create_branch(owner, repo, branch_name)
        if not branch_result:
            return
        
        # Add a comment to the issue
        self.github.add_issue_comment(
            owner,
            repo,
            issue_number,
            f"I've created a branch `{branch_name}` for this issue. Working on an automated fix..."
        )
        
        # Here you would typically:
        # 1. Analyze the issue (perhaps with AI)
        # 2. Generate code changes
        # 3. Create the PR
        
        # For this example, we'll just create a simple PR with a placeholder file
        file_changes = [{
            "path": f"docs/issues/{issue_number}.md",
            "content": f"# Issue #{issue_number}: {title}\n\n{description}\n\n## Fix Status\n\nIn progress\n",
            "message": f"Create documentation for issue #{issue_number}"
        }]
        
        # Create a PR
        pr = automated_pr_workflow(
            self.github,
            owner,
            repo,
            f"issue-{issue_number}",
            file_changes
        )
        
        if pr:
            # Link the PR to the issue
            self.github.add_issue_comment(
                owner,
                repo,
                issue_number,
                f"Created PR #{pr['number']} to address this issue: {pr['html_url']}"
            )
```

## Best Practices for GitHub Integration

1. **Authentication Security**: Store API keys and tokens securely
2. **Idempotent Operations**: Design operations to be safely retryable
3. **Rate Limiting**: Implement backoff strategies to handle GitHub's rate limits
4. **Webhook Verification**: Validate webhook signatures to prevent spoofing
5. **Error Handling**: Properly handle and log GitHub API errors

## Common Use Cases

1. **Automated Code Reviews**: Trigger automated reviews when PRs are created
2. **Issue Triage**: Automatically categorize and assign incoming issues
3. **Deployment Automation**: Trigger deployments when specific branches are updated
4. **Documentation Generation**: Auto-update documentation when code changes
5. **AI-Assisted Coding**: Generate code fixes based on issue descriptions

## Advanced Configuration

For more complex scenarios, you can configure GitHub Apps through the DevOps MCP platform:

```python
def configure_github_app(name, webhook_url, permissions):
    """Configure a GitHub App through MCP"""
    data = {
        "name": name,
        "webhook_url": webhook_url,
        "permissions": permissions
    }
    
    response = requests.post(
        f"{MCP_BASE_URL}/config/github/app",
        headers=headers,
        data=json.dumps(data)
    )
    
    return response.json() if response.status_code == 200 else None
```

With this foundational knowledge, you can now integrate GitHub operations into your workflows using the DevOps MCP platform.
