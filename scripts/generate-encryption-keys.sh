#!/bin/bash
# Script to generate secure encryption keys for production use

set -euo pipefail

echo "=== DevOps MCP Encryption Key Generator ==="
echo ""
echo "This script generates secure encryption keys for production use."
echo "Each key should be at least 32 characters long."
echo ""

# Function to generate a secure key
generate_key() {
    # Generate 32 bytes of random data and encode as base64
    # This produces a 44-character string suitable for encryption
    openssl rand -base64 32 | tr -d '\n'
}

# Generate keys
CREDENTIAL_KEY=$(generate_key)
DEVMESH_KEY=$(generate_key)
MASTER_KEY=$(generate_key)
LEGACY_KEY=$(generate_key)

# Output in different formats
echo "=== Environment Variables ==="
echo "Add these to your .env file or set as environment variables:"
echo ""
echo "CREDENTIAL_ENCRYPTION_KEY=$CREDENTIAL_KEY"
echo "DEVMESH_ENCRYPTION_KEY=$DEVMESH_KEY"
echo "ENCRYPTION_MASTER_KEY=$MASTER_KEY"
echo "ENCRYPTION_KEY=$LEGACY_KEY"
echo ""

echo "=== Docker Compose ==="
echo "Add these to your docker-compose.yml environment section:"
echo ""
echo "      - CREDENTIAL_ENCRYPTION_KEY=$CREDENTIAL_KEY"
echo "      - DEVMESH_ENCRYPTION_KEY=$DEVMESH_KEY"
echo "      - ENCRYPTION_MASTER_KEY=$MASTER_KEY"
echo "      - ENCRYPTION_KEY=$LEGACY_KEY"
echo ""

echo "=== Kubernetes Secret ==="
echo "Create a Kubernetes secret with:"
echo ""
echo "kubectl create secret generic encryption-keys \\"
echo "  --from-literal=credential-encryption-key=$CREDENTIAL_KEY \\"
echo "  --from-literal=devmesh-encryption-key=$DEVMESH_KEY \\"
echo "  --from-literal=encryption-master-key=$MASTER_KEY \\"
echo "  --from-literal=encryption-key=$LEGACY_KEY"
echo ""

echo "=== AWS Systems Manager Parameter Store ==="
echo "Store in Parameter Store with:"
echo ""
echo "aws ssm put-parameter --name '/devops-mcp/encryption/credential-key' --value '$CREDENTIAL_KEY' --type 'SecureString'"
echo "aws ssm put-parameter --name '/devops-mcp/encryption/devmesh-key' --value '$DEVMESH_KEY' --type 'SecureString'"
echo "aws ssm put-parameter --name '/devops-mcp/encryption/master-key' --value '$MASTER_KEY' --type 'SecureString'"
echo "aws ssm put-parameter --name '/devops-mcp/encryption/legacy-key' --value '$LEGACY_KEY' --type 'SecureString'"
echo ""

echo "=== IMPORTANT SECURITY NOTES ==="
echo "1. Store these keys securely - they cannot be recovered if lost"
echo "2. Never commit these keys to version control"
echo "3. Use different keys for each environment (dev, staging, prod)"
echo "4. Rotate keys periodically according to your security policy"
echo "5. Ensure proper access controls on wherever you store these keys"
echo ""