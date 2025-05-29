#!/bin/sh
# This script will be executed when the container starts to ensure proper Redis configuration

# Create a more direct Redis configuration
cat > /tmp/redis-config.yaml << EOL
redis:
  # Direct connection configuration - using TCP hostname
  addr: redis:6379
  host: redis
  port: 6379
  # Alternative formats
  url: redis://redis:6379
  password: ""
  db: 0
EOL

# Replace Redis configuration in the main config
sed -i '/redis:/,/logging:/s/^/# DISABLED: /' /app/configs/config.yaml
sed -i '/^# DISABLED: logging:/s/^# DISABLED: //' /app/configs/config.yaml
sed -i '/# DISABLED: redis:/r /tmp/redis-config.yaml' /app/configs/config.yaml

# Print the updated config
echo "=== UPDATED CONFIG ==="
grep -A 20 "redis:" /app/configs/config.yaml
echo "===================="

# Continue with regular startup
exec "$@"
