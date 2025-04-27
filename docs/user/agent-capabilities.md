# AI Agent Capabilities

This document provides a comprehensive overview of what AI agents can accomplish with the MCP (Model Context Protocol) server.

## DevOps Tool Integration (With Safety Restrictions)

### GitHub Operations
- Create and update issues
- Close and comment on issues
- Create, review, and merge pull requests
- Query repository information
- **Archive repositories** (but cannot delete them)
- Search code and issues
- Receive and process GitHub webhooks

## Advanced Agent Capabilities

### Tool Operations
- Perform direct GitHub operations
- Get detailed information about GitHub resources
- Execute actions on repositories and issues
- Query GitHub data

### Event Handling
- Process webhooks from GitHub
- Respond to real-time events
- Track task progress through events

### Safety-Aware Operations
- Automatically prevent dangerous operations
- Get clear error messages about restricted operations
- List all allowed operations for each tool
- Understand the safety restrictions in place

### Multi-Turn Workflows
- Execute multi-step GitHub workflows
- Track long-running operations

## API Endpoints for AI Agents

AI agents can interact with the MCP server through a RESTful API:

### Tool API Endpoints
- Execute Tool Action: `POST /api/v1/tools/:tool/actions/:action`
- Query Tool Data: `POST /api/v1/tools/:tool/query`
- List Available Tools: `GET /api/v1/tools`
- List Allowed Actions: `GET /api/v1/tools/:tool/actions`

### Webhook Endpoints
- GitHub: `POST /webhook/github`

## Client Library

The MCP server provides a Go client library (`pkg/client`) that AI agents can use to interact with the server. The client library provides methods for all of the above operations with appropriate error handling and safety checks.

## Safety Guarantees

Our safety implementation ensures that AI agents cannot accidentally:

1. Delete GitHub repositories (but they can archive them)

This safety guarantee protects against potentially destructive operations while still allowing agents to perform useful tasks.
