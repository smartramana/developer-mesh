#!/bin/bash
# Create database in RDS through SSH tunnel

set -e

# Load environment variables
source .env

# Get RDS password from Parameter Store
echo "Retrieving RDS password from Parameter Store..."
DB_PASSWORD=$(aws ssm get-parameter --name "/devops-mcp/rds/password" --with-decryption --query 'Parameter.Value' --output text --region us-east-1)

if [ -z "$DB_PASSWORD" ]; then
    echo "Failed to retrieve password from Parameter Store"
    exit 1
fi

export PGPASSWORD="$DB_PASSWORD"

# Connection parameters (through SSH tunnel)
HOST="localhost"
PORT="5432"
USER="postgres"

echo "Checking if database devops_mcp_dev exists..."

# Check if database exists
DB_EXISTS=$(echo "SELECT 1 FROM pg_database WHERE datname = 'devops_mcp_dev';" | psql -h $HOST -p $PORT -U $USER -d postgres -tAq 2>/dev/null || echo "0")

if [ "$DB_EXISTS" != "1" ]; then
    echo "Creating database devops_mcp_dev..."
    psql -h $HOST -p $PORT -U $USER -d postgres -c "CREATE DATABASE devops_mcp_dev;"
    echo "Database created successfully!"
else
    echo "Database devops_mcp_dev already exists"
fi

echo "Applying schema..."
psql -h $HOST -p $PORT -U $USER -d devops_mcp_dev < scripts/db/init.sql

echo "Database setup complete!"