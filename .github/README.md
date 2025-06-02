# GitHub Configuration

This directory contains GitHub-specific configurations for the DevOps MCP project.

## Contents

- `workflows/` - GitHub Actions CI/CD workflows
- `scripts/` - Helper scripts used by workflows
- `dependabot.yml` - Automated dependency updates configuration
- `commitlint.config.js` - Commit message linting rules

## Workflow Overview

All workflows are configured to work with Go 1.24 workspaces and include proper error handling following industry best practices.

Last validated: 2025-06-02