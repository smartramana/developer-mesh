#!/bin/bash

# Script to fix EC2 deployment directory structure issues
# Fixes circular symlinks and ensures proper directory setup

set -e

echo "🔧 Fixing EC2 deployment directory structure..."

# Create a deployment fix script to run via SSM
cat > /tmp/fix-deployment-dirs.sh << 'EOF'
#!/bin/bash
set -e

echo "Starting deployment directory fix..."

cd /home/ec2-user

# Stop any running containers first
cd developer-mesh 2>/dev/null && docker-compose down 2>/dev/null || true
cd /home/ec2-user

# Remove broken symlinks and create clean directory structure
echo "Cleaning up old symlinks..."
rm -rf developer-mesh.old 2>/dev/null || true
if [ -d developer-mesh ]; then
    mv developer-mesh developer-mesh.old
fi

# Create fresh developer-mesh directory
mkdir -p developer-mesh
cd developer-mesh

# Create proper directories
mkdir -p configs logs nginx

# Download latest docker-compose files
echo "Downloading docker-compose files..."
curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/docker-compose.production.yml -o docker-compose.production.yml
cp docker-compose.production.yml docker-compose.yml

# Download config files
echo "Downloading config files..."
curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.base.yaml -o configs/config.base.yaml
curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.production.yaml -o configs/config.production.yaml
curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.rest-api.yaml -o configs/config.rest-api.yaml
curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/auth.production.yaml -o configs/auth.production.yaml

# Copy nginx config if it exists in old location
if [ -f /home/ec2-user/devops-mcp/nginx/mcp.conf ]; then
    cp /home/ec2-user/devops-mcp/nginx/mcp.conf nginx/
elif [ -f /home/ec2-user/developer-mesh.old/nginx/mcp.conf ]; then
    cp /home/ec2-user/developer-mesh.old/nginx/mcp.conf nginx/
fi

# Copy .env if it exists
if [ -f /home/ec2-user/developer-mesh.old/.env ]; then
    cp /home/ec2-user/developer-mesh.old/.env .env
fi

# Verify structure
echo "Verifying directory structure..."
echo "Contents of developer-mesh:"
ls -la
echo ""
echo "Contents of configs:"
ls -la configs/
echo ""

# Fix permissions
chown -R ec2-user:ec2-user /home/ec2-user/developer-mesh

echo "Directory structure fix complete!"
EOF

echo "✅ Created deployment fix script"

# Create SSM command to fix the deployment
cat > /tmp/ssm-fix-command.json << 'EOF'
{
  "commands": [
    "#!/bin/bash",
    "set -e",
    "cd /home/ec2-user",
    "# Stop containers",
    "cd developer-mesh 2>/dev/null && docker-compose down 2>/dev/null || true",
    "cd /home/ec2-user",
    "# Backup and recreate directory",
    "rm -rf developer-mesh.backup",
    "[ -d developer-mesh ] && mv developer-mesh developer-mesh.backup",
    "mkdir -p developer-mesh",
    "cd developer-mesh",
    "# Create directories",
    "mkdir -p configs logs nginx",
    "# Download files",
    "curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/docker-compose.production.yml -o docker-compose.yml",
    "curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.base.yaml -o configs/config.base.yaml",
    "curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.production.yaml -o configs/config.production.yaml",
    "curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/config.rest-api.yaml -o configs/config.rest-api.yaml",
    "curl -sL https://raw.githubusercontent.com/developer-mesh/developer-mesh/main/configs/auth.production.yaml -o configs/auth.production.yaml",
    "# Restore .env if exists",
    "[ -f ../developer-mesh.backup/.env ] && cp ../developer-mesh.backup/.env .",
    "# Fix ownership",
    "chown -R ec2-user:ec2-user /home/ec2-user/developer-mesh",
    "echo 'Directory fix complete'",
    "ls -la",
    "ls -la configs/"
  ]
}
EOF

echo ""
echo "📝 The issue:"
echo "  - The developer-mesh/configs directory is a circular symlink pointing to itself"
echo "  - docker-compose.yml and docker-compose.production.yml are symlinks to non-existent devops-mcp directory"
echo "  - This causes the deployment to fail when trying to create/access files"
echo ""
echo "📋 Manual fix via SSH:"
echo "  scp -i ~/.ssh/nat-instance.pem /tmp/fix-deployment-dirs.sh ec2-user@IP:~/"
echo "  ssh -i ~/.ssh/nat-instance.pem ec2-user@IP 'chmod +x fix-deployment-dirs.sh && sudo ./fix-deployment-dirs.sh'"
echo ""
echo "📋 Fix via AWS SSM:"
echo "  aws ssm send-command \\"
echo "    --instance-ids \"INSTANCE_ID\" \\"
echo "    --document-name \"AWS-RunShellScript\" \\"
echo "    --parameters file:///tmp/ssm-fix-command.json"
echo ""
echo "This will create a clean developer-mesh directory with proper structure."