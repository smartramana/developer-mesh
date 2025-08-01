#!/bin/bash

# Script to fix MCP server config deployment issue
# The MCP server is failing to start because it can't find configs/config.yaml

set -e

echo "🔧 Fixing MCP server config deployment issue..."

# Create a patch for the deployment workflow
cat > /tmp/config-deployment-fix.patch << 'EOF'
--- a/.github/workflows/deploy-production-v2.yml
+++ b/.github/workflows/deploy-production-v2.yml
@@ -170,11 +170,13 @@ jobs:
               'echo "Downloading config files..."',
               'mkdir -p configs',
               'curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.base.yaml -o configs/config.base.yaml',
               'curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.production.yaml -o configs/config.production.yaml',
               'curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.rest-api.yaml -o configs/config.rest-api.yaml',
               'curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/auth.production.yaml -o configs/auth.production.yaml',
+              'curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.yaml -o configs/config.yaml',
               'echo "Verifying downloaded files..."',
-              'for file in docker-compose.production.yml configs/config.base.yaml configs/config.production.yaml configs/config.rest-api.yaml configs/auth.production.yaml; do',
+              'for file in docker-compose.production.yml configs/config.base.yaml configs/config.production.yaml configs/config.rest-api.yaml configs/auth.production.yaml configs/config.yaml; do',
               '  if [ ! -f "\$file" ] || [ ! -s "\$file" ]; then',
               '    echo "ERROR: File \$file is missing or empty"',
               '    exit 1',
EOF

echo "✅ Created patch file for deployment workflow"

# Create a manual fix script for EC2
cat > /tmp/fix-ec2-configs.sh << 'EOF'
#!/bin/bash
# Manual fix script to run on EC2 instance

set -e

echo "Fixing config directory on EC2..."

cd ~/developer-mesh

# Stop containers
docker-compose down || true

# Fix the configs directory
# Remove broken symlink if it exists
if [ -L configs ]; then
    echo "Removing broken symlink..."
    rm -f configs
fi

# Create actual configs directory
mkdir -p configs

# Copy config files from the old location if they exist
if [ -d /home/ec2-user/devops-mcp/configs ]; then
    echo "Copying config files from old location..."
    cp -r /home/ec2-user/devops-mcp/configs/* configs/ 2>/dev/null || true
fi

# Download missing config.yaml if needed
if [ ! -f configs/config.yaml ]; then
    echo "Downloading config.yaml..."
    curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.yaml -o configs/config.yaml
fi

# Ensure all required config files exist
for file in config.base.yaml config.production.yaml config.rest-api.yaml auth.production.yaml config.yaml; do
    if [ ! -f configs/$file ]; then
        echo "Downloading missing $file..."
        curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/$file -o configs/$file
    fi
done

echo "Config files:"
ls -la configs/

# Start containers
echo "Starting containers..."
docker-compose up -d

# Check status
sleep 5
docker-compose ps

echo "Fix complete!"
EOF

chmod +x /tmp/fix-ec2-configs.sh

echo ""
echo "📝 Summary of the issue:"
echo ""
echo "The MCP server is failing to start with error:"
echo "  'open configs/config.yaml: no such file or directory'"
echo ""
echo "Root causes:"
echo "1. The deployment workflow is not downloading config.yaml"
echo "2. The configs directory is a symlink pointing to the old devops-mcp directory"
echo "3. The docker-compose command tries to create a symlink but fails"
echo ""
echo "Solutions:"
echo "1. Update the deployment workflow to download config.yaml"
echo "2. Fix the configs directory to contain actual files, not symlinks"
echo "3. Ensure all required config files are present"
echo ""
echo "Files created:"
echo "  - /tmp/config-deployment-fix.patch - Patch for the deployment workflow"
echo "  - /tmp/fix-ec2-configs.sh - Manual fix script for EC2"
echo ""
echo "To apply the manual fix on EC2:"
echo "  scp -i ~/.ssh/nat-instance.pem /tmp/fix-ec2-configs.sh ec2-user@54.86.185.227:~/"
echo "  ssh -i ~/.ssh/nat-instance.pem ec2-user@54.86.185.227 'chmod +x fix-ec2-configs.sh && ./fix-ec2-configs.sh'"