#!/bin/bash
# Script to fix dirty migration state on production EC2 instance

set -e

echo "====================================="
echo "Fixing Production Migration State"
echo "====================================="

# Check for SSH key
if [ -z "$SSH_KEY_PATH" ]; then
    echo "Error: SSH_KEY_PATH environment variable not set"
    echo "Please run: export SSH_KEY_PATH=/path/to/your/key.pem"
    exit 1
fi

# Get EC2 IP from .env if available
if [ -f .env ]; then
    source .env
fi

EC2_IP="${EC2_INSTANCE_IP:-52.91.223.34}"

echo "Connecting to EC2 instance at $EC2_IP..."

# Create the fix script that will run on EC2
FIX_SCRIPT='#!/bin/bash
set -e

echo "Running on EC2 instance..."
cd /home/ec2-user/developer-mesh

# Source the .env file to get database credentials
if [ -f .env ]; then
    source .env
else
    echo "Error: .env file not found on EC2"
    exit 1
fi

echo ""
echo "Current migration status:"
PGPASSWORD="$DATABASE_PASSWORD" psql \
    -h "$DATABASE_HOST" \
    -p "${DATABASE_PORT:-5432}" \
    -U "${DATABASE_USER:-dbadmin}" \
    -d "${DATABASE_NAME:-devops_mcp}" \
    -c "SELECT version, dirty FROM schema_migrations ORDER BY version::int DESC LIMIT 5;"

echo ""
echo "Fixing dirty state for migration 21..."
PGPASSWORD="$DATABASE_PASSWORD" psql \
    -h "$DATABASE_HOST" \
    -p "${DATABASE_PORT:-5432}" \
    -U "${DATABASE_USER:-dbadmin}" \
    -d "${DATABASE_NAME:-devops_mcp}" \
    -c "UPDATE schema_migrations SET dirty = false WHERE version = '\''21'\'';"

echo ""
echo "Updated migration status:"
PGPASSWORD="$DATABASE_PASSWORD" psql \
    -h "$DATABASE_HOST" \
    -p "${DATABASE_PORT:-5432}" \
    -U "${DATABASE_USER:-dbadmin}" \
    -d "${DATABASE_NAME:-devops_mcp}" \
    -c "SELECT version, dirty FROM schema_migrations ORDER BY version::int DESC LIMIT 5;"

echo ""
echo "Restarting REST API to trigger migrations..."
docker restart rest-api

echo ""
echo "Waiting for REST API to start..."
sleep 10

echo ""
echo "REST API logs showing migration status:"
docker logs rest-api 2>&1 | grep -i "migration" | tail -20

echo ""
echo "Checking if migration 24 was applied:"
PGPASSWORD="$DATABASE_PASSWORD" psql \
    -h "$DATABASE_HOST" \
    -p "${DATABASE_PORT:-5432}" \
    -U "${DATABASE_USER:-dbadmin}" \
    -d "${DATABASE_NAME:-devops_mcp}" \
    -c "SELECT version, dirty FROM schema_migrations WHERE version IN ('\''22'\'', '\''23'\'', '\''24'\'') ORDER BY version::int;"

echo ""
echo "Checking agents.model_id column type:"
PGPASSWORD="$DATABASE_PASSWORD" psql \
    -h "$DATABASE_HOST" \
    -p "${DATABASE_PORT:-5432}" \
    -U "${DATABASE_USER:-dbadmin}" \
    -d "${DATABASE_NAME:-devops_mcp}" \
    -c "SELECT column_name, data_type, character_maximum_length FROM information_schema.columns WHERE table_schema = '\''public'\'' AND table_name = '\''agents'\'' AND column_name = '\''model_id'\'';"
'

# Execute the script on EC2
ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no ec2-user@"$EC2_IP" "$FIX_SCRIPT"

echo ""
echo "====================================="
echo "Migration fix complete!"
echo "====================================="