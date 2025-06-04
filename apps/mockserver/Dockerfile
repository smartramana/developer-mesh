# syntax=docker/dockerfile:1
# Multi-stage build for optimal image size and security

# Build arguments for metadata
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE

# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy only the mockserver app since it's self-contained
COPY apps/mockserver/ ./

# Download dependencies and build with cache mounts
# Re-declare build args for use in this stage
ARG VERSION
ARG COMMIT_SHA
ARG BUILD_DATE

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static' -X main.Version=${VERSION} -X main.GitCommit=${COMMIT_SHA} -X main.BuildDate=${BUILD_DATE}" \
    -a -installsuffix cgo \
    -o mockserver ./cmd/main.go

# Runtime stage - using distroless for security
FROM gcr.io/distroless/static:nonroot

# Copy timezone data and SSL certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder
COPY --from=builder /build/mockserver /app/mockserver

# Set working directory
WORKDIR /app

# Use non-root user
USER nonroot

# Expose port
EXPOSE 8081

# Health check is handled by docker-compose or Kubernetes
# The mockserver provides a /health endpoint on the configured port

# Environment variables
ENV GIN_MODE=release \
    PORT=8081

# Add metadata labels
ARG VERSION
ARG COMMIT_SHA
ARG BUILD_DATE
LABEL org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_DATE}"

# Run the application
ENTRYPOINT ["/app/mockserver"]