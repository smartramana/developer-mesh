#!/bin/bash
# Validates deployment configuration before deploying

set -e

echo "🔍 Validating deployment configuration..."

# Check required environment variables
REQUIRED_VARS=(
    "DATABASE_PASSWORD"
    "ADMIN_API_KEY"
    "DATABASE_HOST"
    "REDIS_ENDPOINT"
    "S3_BUCKET"
    "SQS_QUEUE_URL"
)

missing_vars=()
for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        missing_vars+=("$var")
    fi
done

if [ ${#missing_vars[@]} -gt 0 ]; then
    echo "❌ Missing required environment variables:"
    printf ' - %s\n' "${missing_vars[@]}"
    exit 1
fi

# Validate DATABASE_PASSWORD doesn't contain special characters
if [[ "$DATABASE_PASSWORD" =~ [^a-zA-Z0-9] ]]; then
    echo "❌ DATABASE_PASSWORD contains special characters. Use alphanumeric only."
    exit 1
fi

# Validate API key format (should be hex string)
if [[ ! "$ADMIN_API_KEY" =~ ^[a-f0-9]{64}$ ]]; then
    echo "❌ ADMIN_API_KEY should be a 64-character hex string"
    exit 1
fi

echo "✅ All validations passed"