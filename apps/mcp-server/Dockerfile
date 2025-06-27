# syntax=docker/dockerfile:1
# Multi-stage build for optimal image size and security

# Build arguments for metadata
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE

# Build stage
FROM golang:1.24-alpine AS builder

# Install security updates and build dependencies
RUN apk update && apk upgrade && \
    apk add --no-cache git gcc musl-dev ca-certificates tzdata && \
    update-ca-certificates

# Create non-root user for build
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /build

# Copy entire project for Go workspace support
COPY . .

# Set up Go workspace and build
WORKDIR /build
# Re-declare build args for use in this stage
ARG VERSION
ARG COMMIT_SHA
ARG BUILD_DATE


# Build the application
WORKDIR /build/apps/mcp-server
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static' -X main.Version=${VERSION} -X main.GitCommit=${COMMIT_SHA} -X main.BuildDate=${BUILD_DATE}" \
    -a -installsuffix cgo \
    -o mcp-server ./cmd/server

# Create minimal runtime stage using distroless
FROM gcr.io/distroless/static:nonroot

# Copy timezone data and SSL certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder
COPY --from=builder /build/apps/mcp-server/mcp-server /app/mcp-server

# Copy configuration files
COPY --chown=nonroot:nonroot apps/mcp-server/configs /app/configs
COPY --chown=nonroot:nonroot configs /app/base-configs

# Set working directory
WORKDIR /app

# Use non-root user
USER nonroot

# Expose port (using standard port for consistency)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/mcp-server", "-health-check"]

# Environment variables
ENV MCP_CONFIG_FILE=/app/configs/config.docker.yaml \
    PORT=8080 \
    GIN_MODE=release \
    # Redis configuration
    REDIS_ADDR="redis:6379" \
    REDIS_ADDRESS="redis:6379" \
    REDIS_HOST="redis" \
    REDIS_PORT="6379" \
    REDIS_URL="redis://redis:6379" \
    # Observability
    OTEL_SERVICE_NAME="mcp-server" \
    OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector:4317"

# Add metadata labels
ARG VERSION
ARG COMMIT_SHA
ARG BUILD_DATE
LABEL org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_DATE}"

# Run the application
ENTRYPOINT ["/app/mcp-server"]