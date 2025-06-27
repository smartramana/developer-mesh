#!/bin/bash
# Remove temporary direct access

echo "Removing direct access rules..."

# Remove direct PostgreSQL access
aws ec2 revoke-security-group-ingress \
  --group-id sg-0de6028f4ccb89795 \
  --protocol tcp \
  --port 5432 \
  --cidr 72.135.248.191/32 \
  --region us-east-1 \
  2>/dev/null || echo "PostgreSQL rule not found"

# Remove direct Redis access  
aws ec2 revoke-security-group-ingress \
  --group-id sg-00303bd7b592ecc63 \
  --protocol tcp \
  --port 6379 \
  --cidr 72.135.248.191/32 \
  --region us-east-1 \
  2>/dev/null || echo "Redis rule not found"

echo "Direct access disabled!"