#!/bin/bash

# Restore application module files properly
# This creates clean go.mod files for each application

set -e

echo "=== Restoring Application Module Files ==="
echo ""

# Restore mcp-server
echo "Restoring apps/mcp-server/go.mod..."
cat > apps/mcp-server/go.mod << 'EOF'
module github.com/S-Corkum/devops-mcp/apps/mcp-server

go 1.24.2

require (
	github.com/S-Corkum/devops-mcp v0.0.0
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/gin-gonic/gin v1.9.1
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.7.4
	github.com/jmoiron/sqlx v1.3.5
	github.com/lib/pq v1.10.9
	github.com/sony/gobreaker v1.0.0
	github.com/stretchr/testify v1.10.0
	github.com/swaggo/files v1.0.1
	github.com/swaggo/gin-swagger v1.6.0
	github.com/xeipuuv/gojsonschema v1.2.0
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/trace v1.35.0
	go.uber.org/goleak v1.3.0
	golang.org/x/time v0.8.0
)
EOF

# Restore rest-api
echo "Restoring apps/rest-api/go.mod..."
cat > apps/rest-api/go.mod << 'EOF'
module github.com/S-Corkum/devops-mcp/apps/rest-api

go 1.24.2

require (
	github.com/S-Corkum/devops-mcp v0.0.0
	github.com/S-Corkum/devops-mcp/apps/mcp-server v0.0.0
	github.com/gin-gonic/gin v1.9.1
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/google/uuid v1.6.0
	github.com/jmoiron/sqlx v1.4.0
	github.com/lib/pq v1.10.9
	github.com/stretchr/testify v1.10.0
	github.com/swaggo/files v1.0.1
	github.com/swaggo/gin-swagger v1.6.0
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/trace v1.35.0
	go.uber.org/zap v1.27.0
	golang.org/x/time v0.8.0
)
EOF

# Restore worker
echo "Restoring apps/worker/go.mod..."
cat > apps/worker/go.mod << 'EOF'
module github.com/S-Corkum/devops-mcp/apps/worker

go 1.24.2

require (
	github.com/S-Corkum/devops-mcp v0.0.0
	github.com/aws/aws-sdk-go-v2 v1.36.3
	github.com/aws/aws-sdk-go-v2/config v1.29.14
	github.com/aws/aws-sdk-go-v2/service/sqs v1.39.5
	github.com/go-redis/redis/v8 v8.11.5
	github.com/google/uuid v1.6.0
	github.com/stretchr/testify v1.10.0
	go.uber.org/zap v1.27.0
)
EOF

# Restore mockserver
echo "Restoring apps/mockserver/go.mod..."
cat > apps/mockserver/go.mod << 'EOF'
module github.com/S-Corkum/devops-mcp/apps/mockserver

go 1.24.2

require (
	github.com/gorilla/mux v1.8.1
	github.com/stretchr/testify v1.10.0
	github.com/tidwall/gjson v1.18.0
)
EOF

# Restore test/functional
echo "Restoring test/functional/go.mod..."
cat > test/functional/go.mod << 'EOF'
module github.com/S-Corkum/devops-mcp/test/functional

go 1.24.2

require (
	github.com/S-Corkum/devops-mcp v0.0.0
	github.com/gorilla/mux v1.8.1
	github.com/onsi/ginkgo/v2 v2.22.2
	github.com/onsi/gomega v1.36.2
	github.com/stretchr/testify v1.10.0
)
EOF

# Restore pkg/tests/integration
echo "Restoring pkg/tests/integration/go.mod..."
cat > pkg/tests/integration/go.mod << 'EOF'
module github.com/S-Corkum/devops-mcp/pkg/tests/integration

go 1.24.2

require (
	github.com/S-Corkum/devops-mcp v0.0.0
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/google/uuid v1.6.0
	github.com/jmoiron/sqlx v1.4.0
	github.com/lib/pq v1.10.9
	github.com/stretchr/testify v1.10.0
	go.uber.org/goleak v1.3.0
)
EOF

# Keep the mcp-server test integration module
echo "Checking apps/mcp-server/tests/integration/go.mod..."
if [ ! -f "apps/mcp-server/tests/integration/go.mod" ]; then
    cat > apps/mcp-server/tests/integration/go.mod << 'EOF'
module github.com/S-Corkum/devops-mcp/apps/mcp-server/tests/integration

go 1.24.2
EOF
fi

echo ""
echo "=== Module restoration complete ==="
echo ""
echo "Now let's try building with the workspace..."