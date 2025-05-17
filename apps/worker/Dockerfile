# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy workspace settings
COPY go.work go.work.sum* ./

# Create module directory structure
RUN mkdir -p apps/worker pkg/common pkg/models pkg/embedding pkg/chunking pkg/storage

# Copy go mod files for all modules
COPY apps/worker/go.mod apps/worker/go.sum* ./apps/worker/
COPY pkg/common/go.mod pkg/common/go.sum* ./pkg/common/
COPY pkg/models/go.mod pkg/models/go.sum* ./pkg/models/
COPY pkg/embedding/go.mod pkg/embedding/go.sum* ./pkg/embedding/
COPY pkg/chunking/go.mod pkg/chunking/go.sum* ./pkg/chunking/
COPY pkg/storage/go.mod pkg/storage/go.sum* ./pkg/storage/

# Copy source code for all modules
COPY apps/worker/ ./apps/worker/
COPY pkg/common/ ./pkg/common/
COPY pkg/models/ ./pkg/models/
COPY pkg/embedding/ ./pkg/embedding/
COPY pkg/chunking/ ./pkg/chunking/
COPY pkg/storage/ ./pkg/storage/

# Build the application
WORKDIR /app/apps/worker
RUN CGO_ENABLED=0 GOOS=linux go build -v -o worker ./cmd/worker

# Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/apps/worker/worker .

# Copy configs directory
COPY apps/worker/configs ./configs

# Expose any required ports (if needed)
# EXPOSE 8080

# Run the application
ENTRYPOINT ["/app/worker"]
