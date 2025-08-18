#!/bin/bash
# MCP Tool Usage Validation Script
# This script reminds developers and AI assistants to use MCP tools

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}🚨 MCP TOOL USAGE REMINDER 🚨${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo

echo -e "${GREEN}Before performing ANY operation, check:${NC}"
echo "1. Have you run 'tools/list' to see available MCP tools?"
echo "2. Is there an mcp__devmesh__* tool for your task?"
echo "3. Are you using MCP tools instead of CLI commands?"
echo

echo -e "${YELLOW}Common MCP Tool Mappings:${NC}"
echo "┌─────────────────────────┬──────────────────────────────┬─────────────────┐"
echo "│ Operation               │ Use This MCP Tool            │ NOT This        │"
echo "├─────────────────────────┼──────────────────────────────┼─────────────────┤"
echo "│ Create GitHub tag       │ mcp__devmesh__github_git     │ git tag         │"
echo "│ Create GitHub release   │ mcp__devmesh__github_repos   │ gh release      │"
echo "│ Manage GitHub issues    │ mcp__devmesh__github_issues  │ gh issue        │"
echo "│ Manage GitHub PRs       │ mcp__devmesh__github_pulls   │ gh pr           │"
echo "│ GitHub Actions          │ mcp__devmesh__github_actions │ gh workflow     │"
echo "│ Any GitHub operation    │ mcp__devmesh__github_*       │ gh CLI          │"
echo "└─────────────────────────┴──────────────────────────────┴─────────────────┘"
echo

echo -e "${RED}Remember:${NC}"
echo "• MCP tools are dynamically loaded - check frequently"
echo "• Tools may change between sessions"
echo "• Always prefer MCP over CLI for supported operations"
echo

echo -e "${GREEN}Quick Check Commands:${NC}"
echo "1. List all tools: tools/list"
echo "2. Filter GitHub tools: tools/list | grep github"
echo "3. Check specific tool: tools/call <tool_name> --help"
echo

echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

# Exit with 0 so this can be used as a reminder without breaking workflows
exit 0