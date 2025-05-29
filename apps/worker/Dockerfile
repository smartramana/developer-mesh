# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Create a custom go.work file for building just this module
RUN echo 'go 1.24' > go.work && \
    echo 'use (' >> go.work && \
    echo '    ./apps/worker' >> go.work && \
    echo '    ./pkg/adapters' >> go.work && \
    echo '    ./pkg/common' >> go.work && \
    echo '    ./pkg/database' >> go.work && \
    echo '    ./pkg/events' >> go.work && \
    echo '    ./pkg/models' >> go.work && \
    echo '    ./pkg/embedding' >> go.work && \
    echo '    ./pkg/chunking' >> go.work && \
    echo '    ./pkg/storage' >> go.work && \
    echo '    ./pkg/queue' >> go.work && \
    echo '    ./pkg/observability' >> go.work && \
    echo '    ./pkg/repository' >> go.work && \
    echo '    ./pkg/worker' >> go.work && \
    echo ')' >> go.work

# Create module directory structure
RUN mkdir -p apps/worker pkg/adapters pkg/common pkg/database pkg/events pkg/models pkg/embedding pkg/chunking pkg/storage pkg/queue pkg/observability pkg/repository pkg/worker

# Copy go mod files for all modules
COPY apps/worker/go.mod apps/worker/go.sum* ./apps/worker/
COPY pkg/adapters/go.mod pkg/adapters/go.sum* ./pkg/adapters/
COPY pkg/common/go.mod pkg/common/go.sum* ./pkg/common/
COPY pkg/database/go.mod pkg/database/go.sum* ./pkg/database/
COPY pkg/events/go.mod pkg/events/go.sum* ./pkg/events/
COPY pkg/models/go.mod pkg/models/go.sum* ./pkg/models/
COPY pkg/embedding/go.mod pkg/embedding/go.sum* ./pkg/embedding/
COPY pkg/chunking/go.mod pkg/chunking/go.sum* ./pkg/chunking/
COPY pkg/storage/go.mod pkg/storage/go.sum* ./pkg/storage/
COPY pkg/queue/go.mod pkg/queue/go.sum* ./pkg/queue/
COPY pkg/observability/go.mod pkg/observability/go.sum* ./pkg/observability/
COPY pkg/repository/go.mod pkg/repository/go.sum* ./pkg/repository/
COPY pkg/worker/go.mod pkg/worker/go.sum* ./pkg/worker/

# Copy source code for all modules
COPY apps/worker/ ./apps/worker/
COPY pkg/adapters/ ./pkg/adapters/
COPY pkg/common/ ./pkg/common/
COPY pkg/database/ ./pkg/database/
COPY pkg/events/ ./pkg/events/
COPY pkg/models/ ./pkg/models/
COPY pkg/embedding/ ./pkg/embedding/
COPY pkg/chunking/ ./pkg/chunking/
COPY pkg/storage/ ./pkg/storage/
COPY pkg/queue/ ./pkg/queue/
COPY pkg/observability/ ./pkg/observability/
COPY pkg/repository/ ./pkg/repository/
COPY pkg/worker/ ./pkg/worker/

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

# Set environment variables for Redis and SQS
ENV REDIS_ADDR="redis:6379"
ENV REDIS_ADDRESS="redis:6379"
ENV REDIS_HOST="redis"
ENV REDIS_PORT="6379"
ENV REDIS_URL="redis://redis:6379"

# AWS/SQS settings
ENV USE_LOCALSTACK="true"
ENV AWS_ENDPOINT_URL="http://localstack:4566"
ENV AWS_REGION="us-east-1"
ENV SQS_QUEUE_NAME="tasks"
ENV AWS_ACCESS_KEY_ID="test"
ENV AWS_SECRET_ACCESS_KEY="test"

# Run the application
CMD ["/app/worker"]
