# GitHub Actions Workflows

This directory contains automated workflows for the DevOps MCP project.

## Workflows

### CI (ci.yml)
[![CI](https://github.com/S-Corkum/devops-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/S-Corkum/devops-mcp/actions/workflows/ci.yml)

Runs on every push to `main` and feature branches, and on pull requests.

- **Lint**: Runs golangci-lint
- **Test**: Runs unit tests with coverage
- **Integration Test**: Runs integration tests
- **Build**: Builds all applications
- **Docker Build**: Builds Docker images
- **Functional Test**: Runs end-to-end functional tests

### Release (release.yml)
[![Release](https://github.com/S-Corkum/devops-mcp/actions/workflows/release.yml/badge.svg)](https://github.com/S-Corkum/devops-mcp/actions/workflows/release.yml)

Triggered when a version tag (v*.*.*) is pushed.

- Creates GitHub release with changelog
- Builds binaries for multiple platforms
- Builds and pushes Docker images to GitHub Container Registry

### Security (security.yml)
[![Security](https://github.com/S-Corkum/devops-mcp/actions/workflows/security.yml/badge.svg)](https://github.com/S-Corkum/devops-mcp/actions/workflows/security.yml)

Runs on push, PR, and weekly schedule.

- **Gosec**: Go security scanner
- **Nancy**: Checks for vulnerable dependencies
- **Trivy**: Container vulnerability scanner
- **CodeQL**: GitHub's semantic code analysis

### Dependencies (dependencies.yml)
[![Dependencies](https://github.com/S-Corkum/devops-mcp/actions/workflows/dependencies.yml/badge.svg)](https://github.com/S-Corkum/devops-mcp/actions/workflows/dependencies.yml)

Runs weekly or on manual trigger.

- Updates all Go dependencies
- Creates PR with changes

### PR Checks (pr-checks.yml)

Runs on all pull requests.

- **PR Title**: Enforces conventional commit format
- **PR Size**: Labels PRs by size
- **Branch Name**: Validates branch naming convention
- **Commit Messages**: Validates commit message format

## Branch Protection

For trunk-based development, configure the following branch protection rules for `main`:

1. Require pull request reviews before merging
2. Require status checks to pass before merging:
   - CI / Lint
   - CI / Test
   - CI / Integration Test
   - CI / Build
   - Security / Security Scan
   - PR Checks / PR Title Check
3. Require branches to be up to date before merging
4. Include administrators in restrictions

## Secrets Required

- `GITHUB_TOKEN`: Automatically provided by GitHub Actions
- `AWS_ACCESS_KEY_ID`: (Optional) For AWS deployments
- `AWS_SECRET_ACCESS_KEY`: (Optional) For AWS deployments
- `SLACK_WEBHOOK`: (Optional) For notifications