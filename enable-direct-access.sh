#!/bin/bash
# Temporarily enable direct access for testing

echo "Enabling direct access from your IP for testing..."

# Allow direct PostgreSQL access
aws ec2 authorize-security-group-ingress \
  --group-id sg-0de6028f4ccb89795 \
  --protocol tcp \
  --port 5432 \
  --cidr 72.135.248.191/32 \
  --region us-east-1 \
  2>/dev/null || echo "PostgreSQL rule already exists"

# Allow direct Redis access  
aws ec2 authorize-security-group-ingress \
  --group-id sg-00303bd7b592ecc63 \
  --protocol tcp \
  --port 6379 \
  --cidr 72.135.248.191/32 \
  --region us-east-1 \
  2>/dev/null || echo "Redis rule already exists"

echo "Direct access enabled for testing!"
echo "Remember to run ./disable-direct-access.sh when done testing"