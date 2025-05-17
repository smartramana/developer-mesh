# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy workspace settings
COPY go.work go.work.sum* ./

# Create module directory structure
RUN mkdir -p apps/mcp-server pkg/common pkg/models pkg/database pkg/mcp pkg/chunking pkg/embedding pkg/storage

# Copy go mod files for all modules
COPY apps/mcp-server/go.mod apps/mcp-server/go.sum* ./apps/mcp-server/
COPY pkg/common/go.mod pkg/common/go.sum* ./pkg/common/
COPY pkg/models/go.mod pkg/models/go.sum* ./pkg/models/
COPY pkg/database/go.mod pkg/database/go.sum* ./pkg/database/
COPY pkg/mcp/go.mod pkg/mcp/go.sum* ./pkg/mcp/
COPY pkg/chunking/go.mod pkg/chunking/go.sum* ./pkg/chunking/
COPY pkg/embedding/go.mod pkg/embedding/go.sum* ./pkg/embedding/
COPY pkg/storage/go.mod pkg/storage/go.sum* ./pkg/storage/

# Copy source code for all modules
COPY apps/mcp-server/ ./apps/mcp-server/
COPY pkg/common/ ./pkg/common/
COPY pkg/models/ ./pkg/models/
COPY pkg/database/ ./pkg/database/
COPY pkg/mcp/ ./pkg/mcp/
COPY pkg/chunking/ ./pkg/chunking/
COPY pkg/embedding/ ./pkg/embedding/
COPY pkg/storage/ ./pkg/storage/

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

# Run the application
CMD ["./mcp-server"]
