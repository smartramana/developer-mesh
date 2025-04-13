# AI Agent Capabilities

This document provides a comprehensive overview of what AI agents can accomplish with the MCP (Model Context Protocol) server.

## Context Management Capabilities

### Conversation Context Management
- Create new conversation contexts with configurable parameters
- Store and retrieve complete conversation histories
- Track multi-turn conversations with context preservation
- Manage context windows with automatic truncation when they exceed limits
- Choose between different truncation strategies (oldest first, relevance-based, etc.)

### Memory Management
- Store conversation history across multiple sessions
- Maintain user-specific context
- Track conversation state over time
- Search within existing contexts for relevant information

### Context Operations
- Update existing contexts with new messages
- Delete contexts when no longer needed
- List all available contexts for an agent
- Filter contexts by session ID or other metadata
- Generate summaries of lengthy conversations

## DevOps Tool Integration (With Safety Restrictions)

### GitHub Operations
- Create and update issues
- Close and comment on issues
- Create, review, and merge pull requests
- Query repository information
- **Archive repositories** (but cannot delete them)
- Search code and issues
- Receive and process GitHub webhooks

### Harness CI/CD Operations
- Trigger pipeline executions
- Monitor build and deployment statuses
- Stop running pipelines
- Perform rollbacks of failed deployments
- Toggle non-production feature flags
- View pipeline configuration
- **Cannot modify production feature flags** (safety restricted)
- **Cannot delete pipelines or services** (safety restricted)

### Artifactory Operations (Read-Only)
- View and download artifacts
- Search for artifacts across repositories
- Get artifact metadata and properties
- View repository information
- Check storage statistics
- **Cannot upload, modify, or delete artifacts** (safety restricted)

### SonarQube Operations
- Trigger code analysis
- View quality gate status
- List code issues and violations
- Review code quality metrics
- Monitor project status
- **Cannot delete projects** (safety restricted)

### Xray Security Operations
- Scan artifacts for vulnerabilities
- View security vulnerability reports
- Check license compliance
- Monitor policy violations
- **Cannot modify security policies** (safety restricted)

## Advanced Agent Capabilities

### Contextual Tool Operations
- Perform tool operations with context tracking
- Automatically record all operations in conversation context
- Link conversation contexts to tool operations
- Maintain complete history of all tool interactions

### Event Handling
- Process webhooks from various DevOps tools
- Respond to real-time events
- Send notification events
- Track task progress through events

### Safety-Aware Operations
- Automatically prevent dangerous operations
- Get clear error messages about restricted operations
- List all allowed operations for each tool
- Understand the safety restrictions in place

### Multi-Turn Workflows
- Execute multi-step DevOps workflows
- Maintain context between workflow steps
- Resume interrupted workflows
- Track long-running operations

## API Endpoints for AI Agents

AI agents can interact with the MCP server through a RESTful API:

### Context API Endpoints
- Create Context: `POST /api/v1/contexts`
- Get Context: `GET /api/v1/contexts/:id`
- Update Context: `PUT /api/v1/contexts/:id`
- Delete Context: `DELETE /api/v1/contexts/:id`
- List Contexts: `GET /api/v1/contexts?agent_id=:agent_id&session_id=:session_id`
- Search Context: `POST /api/v1/contexts/:id/search`
- Summarize Context: `GET /api/v1/contexts/:id/summary`

### Tool API Endpoints
- Execute Tool Action: `POST /api/v1/tools/:tool/actions/:action?context_id=:context_id`
- Query Tool Data: `POST /api/v1/tools/:tool/query?context_id=:context_id`
- List Available Tools: `GET /api/v1/tools`
- List Allowed Actions: `GET /api/v1/tools/:tool/actions`

### Webhook Endpoints
- Agent Events: `POST /webhook/agent`
- GitHub: `POST /webhook/github`
- Harness: `POST /webhook/harness`
- SonarQube: `POST /webhook/sonarqube`
- Artifactory: `POST /webhook/artifactory`
- Xray: `POST /webhook/xray`

## Client Library

The MCP server provides a Go client library (`pkg/client`) that AI agents can use to interact with the server. The client library provides methods for all of the above operations with appropriate error handling and safety checks.

## Safety Guarantees

Our safety implementation ensures that AI agents cannot accidentally:

1. Delete GitHub repositories (but they can archive them)
2. Upload or delete artifacts in Artifactory (read-only access)
3. Delete or modify production feature flags in Harness
4. Delete projects in SonarQube
5. Modify security policies in Xray

These safety guarantees protect against potentially destructive operations while still allowing agents to perform useful tasks.
