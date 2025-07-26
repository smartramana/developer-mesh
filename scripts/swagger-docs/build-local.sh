#!/bin/bash
set -euo pipefail

# This script builds the Swagger documentation locally for testing
# Actual deployment happens via GitHub Actions

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build/swagger-docs"
OPENAPI_SPEC="$PROJECT_ROOT/docs/swagger/openapi.yaml"
SWAGGER_UI_VERSION="5.27.0"
SWAGGER_UI_DIST_URL="https://unpkg.com/swagger-ui-dist@${SWAGGER_UI_VERSION}/"

echo -e "${GREEN}Building Swagger documentation locally...${NC}"

# Validate OpenAPI spec exists
if [ ! -f "$OPENAPI_SPEC" ]; then
    echo -e "${RED}Error: OpenAPI spec not found at $OPENAPI_SPEC${NC}"
    exit 1
fi

# Create build directory
echo "Creating build directory..."
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Download Swagger UI files
echo "Downloading Swagger UI v${SWAGGER_UI_VERSION}..."
cd "$BUILD_DIR"

# Download essential Swagger UI files from unpkg
FILES=(
    "swagger-ui.css"
    "swagger-ui-bundle.js"
    "swagger-ui-standalone-preset.js"
    "favicon-16x16.png"
    "favicon-32x32.png"
)

for file in "${FILES[@]}"; do
    echo "  Downloading $file..."
    curl -s -L -o "$file" "${SWAGGER_UI_DIST_URL}${file}"
done

# Copy OpenAPI spec
echo "Copying OpenAPI specification..."
cp "$OPENAPI_SPEC" "$BUILD_DIR/openapi.yaml"

# Also copy all referenced files from docs/swagger
echo "Copying referenced OpenAPI files..."
cp -r "$PROJECT_ROOT/docs/swagger/." "$BUILD_DIR/swagger/"

# Create custom index.html with our configuration
echo "Customizing Swagger UI configuration..."
cat > "$BUILD_DIR/index.html" << 'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Developer Mesh API Documentation</title>
    <link rel="stylesheet" type="text/css" href="./swagger-ui.css" />
    <link rel="icon" type="image/png" href="./favicon-32x32.png" sizes="32x32" />
    <link rel="icon" type="image/png" href="./favicon-16x16.png" sizes="16x16" />
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin: 0;
            background: #fafafa;
        }
        .swagger-ui .topbar {
            background-color: #1a1a1a;
        }
        .swagger-ui .topbar .download-url-wrapper .download-url-button {
            background: #4CAF50;
            color: white;
        }
        .swagger-ui .topbar .download-url-wrapper .download-url-button:hover {
            background: #45a049;
        }
        /* Hide the confusing URL input field and only show the title */
        .swagger-ui .topbar .download-url-wrapper {
            display: none !important;
        }
        /* Add a custom header message */
        .swagger-ui .topbar::after {
            content: "Developer Mesh API Documentation";
            color: white;
            font-size: 20px;
            font-weight: 600;
            position: absolute;
            left: 50%;
            top: 50%;
            transform: translate(-50%, -50%);
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="./swagger-ui-bundle.js" charset="UTF-8"></script>
    <script src="./swagger-ui-standalone-preset.js" charset="UTF-8"></script>
    <script>
        window.onload = function() {
            window.ui = SwaggerUIBundle({
                url: "./swagger/openapi.yaml",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout",
                defaultModelsExpandDepth: 1,
                defaultModelExpandDepth: 1,
                docExpansion: "list",
                filter: true,
                showExtensions: true,
                showCommonExtensions: true,
                tryItOutEnabled: true,
                supportedSubmitMethods: ['get', 'post', 'put', 'delete', 'patch'],
                onComplete: function() {
                    console.log("Swagger UI loaded successfully");
                },
                validatorUrl: null
            });
        };
    </script>
</body>
</html>
EOF

echo -e "${GREEN}✓ Build completed successfully!${NC}"
echo -e "${YELLOW}Build output: $BUILD_DIR${NC}"
echo ""
echo "To test locally:"
echo "  cd $BUILD_DIR && python3 -m http.server 8000"
echo "  Then visit: http://localhost:8000"
echo ""
echo -e "${YELLOW}Note: Deployment happens automatically via GitHub Actions${NC}"
echo "      when changes are pushed to the main branch in docs/swagger/**"