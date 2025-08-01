#!/bin/bash

# Script to add config.yaml to the deployment workflow
# This fixes the MCP server startup issue

set -e

echo "🔧 Adding config.yaml to deployment workflow..."

# Create a sed script to update the deployment workflow
cat > /tmp/add-config-yaml.sed << 'EOF'
# Add config.yaml to the download list
/curl -sL.*auth.production.yaml/a\
              'curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.yaml -o configs/config.yaml',

# Update the verification loop to include config.yaml
s/for file in docker-compose.production.yml configs\/config.base.yaml configs\/config.production.yaml configs\/config.rest-api.yaml configs\/auth.production.yaml; do/for file in docker-compose.production.yml configs\/config.base.yaml configs\/config.production.yaml configs\/config.rest-api.yaml configs\/auth.production.yaml configs\/config.yaml; do/
EOF

echo "✅ Created sed script to update deployment workflow"

# Create a temporary emergency fix for the current deployment
cat > /tmp/emergency-config-fix.sh << 'EOF'
#!/bin/bash
# Emergency fix to run on EC2 to get services running

set -e

echo "🚨 Running emergency config fix..."

cd /home/ec2-user/developer-mesh

# Download the missing config.yaml
echo "Downloading config.yaml..."
curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.yaml -o configs/config.yaml

if [ ! -f configs/config.yaml ] || [ ! -s configs/config.yaml ]; then
    echo "ERROR: Failed to download config.yaml"
    exit 1
fi

echo "✓ config.yaml downloaded ($(wc -c < configs/config.yaml) bytes)"

# Restart services
echo "Restarting services..."
docker-compose down
docker-compose up -d

# Wait and check status
sleep 10
docker-compose ps

echo "✅ Emergency fix complete"
EOF

chmod +x /tmp/emergency-config-fix.sh

echo ""
echo "📝 Instructions:"
echo ""
echo "1. To apply the fix to the deployment workflow:"
echo "   sed -i.bak -f /tmp/add-config-yaml.sed .github/workflows/deploy-production-v2.yml"
echo ""
echo "2. To fix the current deployment immediately:"
echo "   scp -i ~/.ssh/nat-instance.pem /tmp/emergency-config-fix.sh ec2-user@54.86.185.227:~/"
echo "   ssh -i ~/.ssh/nat-instance.pem ec2-user@54.86.185.227 './emergency-config-fix.sh'"
echo ""
echo "3. The deployment workflow needs these changes:"
echo "   - Add: curl -sL .../configs/config.yaml -o configs/config.yaml"
echo "   - Update verification loop to include configs/config.yaml"
echo ""
echo "This will fix the 'open configs/config.yaml: no such file or directory' error."