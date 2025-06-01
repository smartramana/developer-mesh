# syntax=docker/dockerfile:1
# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy entire project (Go workspace requires full context)
COPY . .

# Download dependencies and build with cache mounts
WORKDIR /app/apps/rest-api
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo \
    -o rest-api ./cmd/api

# Runtime stage - using distroless for security
FROM gcr.io/distroless/static:nonroot

# Copy binary from builder stage
COPY --from=builder --chown=nonroot:nonroot /app/apps/rest-api/rest-api /app/rest-api

# Copy configs with proper permissions
COPY --from=builder --chown=nonroot:nonroot /app/apps/rest-api/configs /app/configs

# Copy migrations if needed
COPY --from=builder --chown=nonroot:nonroot /app/apps/rest-api/migrations/sql /app/migrations

# Set working directory
WORKDIR /app

# Use non-root user
USER nonroot

# Expose port
EXPOSE 8080

# Set environment variables
ENV MCP_CONFIG_FILE=/app/configs/config.docker.yaml
ENV GIN_MODE=release
ENV PORT=8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/rest-api", "-health-check"]

# Run the application
ENTRYPOINT ["/app/rest-api"]
