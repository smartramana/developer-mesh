# syntax=docker/dockerfile:1
# Multi-stage build for optimal image size and security

# Build arguments for metadata
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE

# Build stage
FROM golang:1.24.4-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy entire project (Go workspace requires full context)
COPY . .

# Enable go workspace
ENV GOWORK=/app/go.work
# Set up Go workspace and build
WORKDIR /app
# Re-declare build args for use in this stage
ARG VERSION
ARG COMMIT_SHA
ARG BUILD_DATE

# Build the application
WORKDIR /app/apps/rest-api
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static' -X main.Version=${VERSION} -X main.GitCommit=${COMMIT_SHA} -X main.BuildDate=${BUILD_DATE}" \
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

# Add metadata labels
ARG VERSION
ARG COMMIT_SHA
ARG BUILD_DATE
LABEL org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_DATE}"

# Run the application
ENTRYPOINT ["/app/rest-api"]
