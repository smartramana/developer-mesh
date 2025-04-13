# Changelog

## 2025-04-13: Fix health check functionality and update quick start guide

### Bug Fixes
- Fixed the GitHub health check in mock mode to properly detect "healthy (mock)" status
- Added mock_responses and mock_url configuration parameters to GitHub adapter in config.yaml
- Enhanced mockserver to handle GitHub health check endpoints
- Added health-check.sh script for Docker healthcheck that correctly interprets mock health statuses
- Updated Docker configuration to use the health check script and added necessary dependencies

### Documentation Updates
- Updated quick start guide to reflect actual endpoint availability and requirements
- Fixed incorrect webhook test information 
- Added explicit configuration instructions for GitHub mock mode in development environments
- Improved API usage examples

### Changes
1. Added missing mock configuration to configs/config.yaml
2. Updated the GitHub adapter in internal/adapters/github/github.go to properly handle mock mode
3. Enhanced the mockserver in cmd/mockserver/main.go to expose health endpoints
4. Created a health-check.sh script for Docker container health checks
5. Modified docker-compose.yml to use the health check script
6. Updated Dockerfile to install required dependencies and copy scripts
7. Fixed documentation in docs/quick-start-guide.md
