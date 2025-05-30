#!/bin/bash
# Validate OpenAPI/Swagger specification

echo "Validating OpenAPI specification..."

# Check if swagger-cli is installed
if ! command -v swagger-cli &> /dev/null; then
    echo "swagger-cli not found. Installing..."
    npm install -g @apidevtools/swagger-cli
fi

# Validate the main OpenAPI spec
echo "Validating main OpenAPI spec..."
swagger-cli validate docs/swagger/openapi.yaml

# Check validation result
if [ $? -eq 0 ]; then
    echo "✅ OpenAPI specification is valid!"
    
    # Bundle the spec into a single file
    echo "Bundling specification..."
    swagger-cli bundle docs/swagger/openapi.yaml -o docs/swagger/openapi-bundled.yaml
    
    echo "✅ Bundled specification created at docs/swagger/openapi-bundled.yaml"
else
    echo "❌ OpenAPI specification validation failed!"
    exit 1
fi