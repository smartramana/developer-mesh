# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy the full source code for proper dependency resolution
COPY . .

# We'll use the existing go.work file which should have all the right modules defined
# If it doesn't exist or has issues, uncomment the following block and configure as needed
#RUN if [ ! -f go.work ]; then \
#    echo "go 1.24.3" > go.work && \
#    echo "use (" >> go.work && \
#    echo "    ./apps/mcp-server" >> go.work && \
#    echo "    ./pkg/common" >> go.work && \
#    echo "    ./pkg/database" >> go.work && \
#    echo "    ./pkg/models" >> go.work && \
#    echo "    ./pkg/mcp" >> go.work && \
#    echo ")" >> go.work; \
#fi

# Build the application
WORKDIR /app/apps/mcp-server
RUN CGO_ENABLED=0 GOOS=linux go build -v -o mcp-server ./cmd/server

# Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl bash

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/apps/mcp-server/mcp-server .

# Copy configs directory
COPY apps/mcp-server/configs /app/configs

# Copy scripts directory
COPY scripts /app/scripts

# Create config directory and make health check executable
RUN mkdir -p /app/configs && chmod +x /app/scripts/health-check.sh

# Expose port
EXPOSE 8080

# Set environment variables
ENV MCP_CONFIG_FILE=/app/configs/config.yaml

# Create a startup wrapper script to ensure Redis connectivity
RUN echo '#!/bin/sh

# Force Redis connection variables
export REDIS_ADDR="redis:6379"
export REDIS_ADDRESS="redis:6379"
export REDIS_HOST="redis"
export REDIS_PORT="6379"
export REDIS_URL="redis://redis:6379"

# Print Redis connection info
echo "Using Redis at: $REDIS_ADDR"

# Start the application
exec ./mcp-server "$@"
' > /app/start.sh && chmod +x /app/start.sh

# Run the application through the wrapper script
CMD ["/app/start.sh"]
