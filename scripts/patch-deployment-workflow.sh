#!/bin/bash

# Script to fix the deployment workflow issues

cat << 'EOF'
The current deployment workflow has several issues that need to be fixed:

1. Container Cleanup Issue:
   - Old containers with the same names are not being properly removed
   - This causes "container name already in use" errors

2. Missing Docker Login:
   - The deployment script doesn't log into GitHub Container Registry
   - This can cause image pull failures

3. Variable Escaping:
   - The $container variable in the loop isn't properly escaped
   - Should be \$container to work inside the base64 encoded script

4. Error Handling:
   - The deployment doesn't fail when containers don't start
   - Smoke tests pass even with failed containers

To fix the deployment workflow, update the "Deploy Services (Simple)" step in deploy-production-v2.yml:

1. Add container cleanup before docker-compose down:
   docker stop mcp-server rest-api worker 2>/dev/null || true
   docker rm mcp-server rest-api worker 2>/dev/null || true

2. Add Docker login before pulling:
   echo "${{ secrets.GITHUB_TOKEN }}" | docker login ghcr.io -u ${{ github.actor }} --password-stdin

3. Fix variable escaping in the status check loop:
   Change: if docker ps | grep -q "$container"; then
   To: if docker ps | grep -q "\$container"; then

4. Add proper exit code handling:
   After checking container status, add:
   CONTAINERS_RUNNING=true
   for container in mcp-server rest-api worker; do
     if ! docker ps | grep -q "\$container"; then
       CONTAINERS_RUNNING=false
     fi
   done
   if [ "\$CONTAINERS_RUNNING" = "false" ]; then
     echo "ERROR: Not all containers are running"
     exit 1
   fi

5. Fix the smoke test to properly fail:
   - Track overall test status
   - Exit with error code 1 if any test fails
   - Add diagnostic output on failure

These changes will ensure:
- Containers start properly
- Deployment fails if containers don't start
- Smoke tests accurately reflect deployment status
EOF