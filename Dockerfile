# Build stage
FROM golang:1.24.2-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download
RUN go mod tidy

# Copy source code
COPY . .

# Create necessary directories if they don't exist
RUN mkdir -p cmd/server internal/adapters internal/api internal/cache internal/config internal/core internal/database internal/metrics pkg/mcp

# Build the application with verbose output to show any issues
RUN CGO_ENABLED=0 GOOS=linux go build -v -a -installsuffix cgo -o mcp-server ./cmd/server

# Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl bash

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/mcp-server .

# Copy config template
COPY --from=builder /app/configs/config.yaml /app/configs/config.yaml

# Copy scripts directory
COPY scripts /app/scripts

# Create config directory
RUN mkdir -p /app/configs && chmod +x /app/scripts/health-check.sh

# Expose port
EXPOSE 8080

# Set environment variables
ENV MCP_CONFIG_FILE=/app/configs/config.yaml

# Run the application
CMD ["./mcp-server"]