# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy workspace settings
COPY go.work go.work.sum* ./

# Create module directory structure
RUN mkdir -p apps/rest-api pkg/common pkg/models pkg/database pkg/embedding pkg/chunking pkg/storage pkg/migrations

# Copy go mod files for all modules
COPY apps/rest-api/go.mod apps/rest-api/go.sum* ./apps/rest-api/
COPY pkg/common/go.mod pkg/common/go.sum* ./pkg/common/
COPY pkg/models/go.mod pkg/models/go.sum* ./pkg/models/
COPY pkg/database/go.mod pkg/database/go.sum* ./pkg/database/
COPY pkg/embedding/go.mod pkg/embedding/go.sum* ./pkg/embedding/
COPY pkg/chunking/go.mod pkg/chunking/go.sum* ./pkg/chunking/
COPY pkg/storage/go.mod pkg/storage/go.sum* ./pkg/storage/
COPY pkg/migrations/go.mod pkg/migrations/go.sum* ./pkg/migrations/

# Copy source code for all modules
COPY apps/rest-api/ ./apps/rest-api/
COPY pkg/common/ ./pkg/common/
COPY pkg/models/ ./pkg/models/
COPY pkg/database/ ./pkg/database/
COPY pkg/embedding/ ./pkg/embedding/
COPY pkg/chunking/ ./pkg/chunking/
COPY pkg/storage/ ./pkg/storage/
COPY pkg/migrations/ ./pkg/migrations/

# Build the application
WORKDIR /app/apps/rest-api
RUN CGO_ENABLED=0 GOOS=linux go build -v -o rest-api ./cmd/api

# Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl bash

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/apps/rest-api/rest-api .

# Copy configs directory
COPY apps/rest-api/configs /app/configs

# Create config directory
RUN mkdir -p /app/configs

# Expose port
EXPOSE 8080

# Set environment variables
ENV API_CONFIG_FILE=/app/configs/config.yaml

# Run the application
CMD ["./rest-api"]
