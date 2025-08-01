#!/bin/bash

# Comprehensive deployment fix script
# This fixes all identified issues with the EC2 deployment

set -e

echo "🔧 Starting holistic deployment fix..."

# 1. Stop all running containers
echo "Stopping existing containers..."
cd /home/ec2-user/developer-mesh 2>/dev/null && docker-compose down 2>/dev/null || true
cd /home/ec2-user

# 2. Backup existing deployment
echo "Backing up existing deployment..."
if [ -d developer-mesh ]; then
    tar -czf developer-mesh-backup-$(date +%Y%m%d_%H%M%S).tar.gz developer-mesh 2>/dev/null || true
    rm -rf developer-mesh.old 2>/dev/null || true
    mv developer-mesh developer-mesh.old
fi

# 3. Create clean directory structure
echo "Creating clean directory structure..."
mkdir -p developer-mesh
cd developer-mesh
mkdir -p configs logs nginx

# 4. Download latest docker-compose file
echo "Downloading docker-compose.production.yml..."
curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/docker-compose.production.yml -o docker-compose.yml
if [ ! -s docker-compose.yml ]; then
    echo "ERROR: Failed to download docker-compose.yml"
    exit 1
fi

# 5. Download all config files
echo "Downloading configuration files..."
for config in config.base.yaml config.production.yaml config.rest-api.yaml auth.production.yaml; do
    curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/$config -o configs/$config
    if [ ! -s configs/$config ]; then
        echo "ERROR: Failed to download $config"
        exit 1
    fi
    echo "✓ Downloaded $config"
done

# 6. Create symlink for config.yaml pointing to config.production.yaml
echo "Creating config.yaml symlink..."
cd configs
ln -sf config.production.yaml config.yaml
cd ..

# 7. Restore .env from backup if exists
if [ -f ../developer-mesh.old/.env ]; then
    echo "Restoring .env file..."
    cp ../developer-mesh.old/.env .env
else
    echo "WARNING: No .env file found in backup"
fi

# 8. Configure GitHub Container Registry authentication
echo "Configuring Docker authentication..."
if [ -n "$GITHUB_TOKEN" ]; then
    echo "$GITHUB_TOKEN" | docker login ghcr.io -u github-actions --password-stdin
else
    echo "WARNING: GITHUB_TOKEN not set, skipping docker login"
fi

# 9. Update docker-compose.yml to fix the command
echo "Fixing docker-compose configuration..."
# Remove the problematic command line from mcp-server
sed -i '/command:.*sh -c.*ln -sf/d' docker-compose.yml

# 10. Set proper permissions
echo "Setting proper permissions..."
chown -R ec2-user:ec2-user /home/ec2-user/developer-mesh

# 11. Pull latest images
echo "Pulling latest Docker images..."
docker-compose pull || echo "WARNING: Some images may not have been pulled"

# 12. Start services
echo "Starting services..."
docker-compose up -d

# 13. Wait for services to stabilize
echo "Waiting for services to stabilize..."
sleep 15

# 14. Check service status
echo "Checking service status..."
docker-compose ps

# 15. Show logs for any failed services
for service in mcp-server rest-api worker; do
    if ! docker ps | grep -q $service; then
        echo "❌ $service is not running. Recent logs:"
        docker logs $service --tail 20 2>&1 || true
    else
        echo "✓ $service is running"
    fi
done

# 16. Final verification
echo ""
echo "Deployment fix completed!"
echo "Directory structure:"
ls -la
echo ""
echo "Configs directory:"
ls -la configs/
echo ""
echo "Container status:"
docker ps

# 17. Test health endpoints
echo ""
echo "Testing health endpoints..."
sleep 5
if curl -f -s http://localhost:8080/health > /dev/null 2>&1; then
    echo "✓ MCP Server health check passed"
else
    echo "❌ MCP Server health check failed"
fi

if curl -f -s http://localhost:8081/health > /dev/null 2>&1; then
    echo "✓ REST API health check passed"
else
    echo "❌ REST API health check failed"
fi