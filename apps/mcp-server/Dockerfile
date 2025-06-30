# syntax=docker/dockerfile:1
# Simple, reliable Dockerfile for mcp-server

# Build arguments
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE

# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /workspace

# Copy go.work files first to enable workspace
COPY go.work go.work.sum ./

# Copy everything - simple and reliable
COPY . .

# Build from workspace root (required for Go workspaces)
# Set GOWORK explicitly to the workspace file
ENV GOWORK=/workspace/go.work

# Download all dependencies first
RUN go mod download -x

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.GitCommit=${COMMIT_SHA} -X main.BuildDate=${BUILD_DATE}" \
    -o /tmp/mcp-server ./apps/mcp-server/cmd/server

# Final stage
FROM gcr.io/distroless/static:nonroot

# Copy binary
COPY --from=builder /tmp/mcp-server /app/mcp-server

# Copy configuration files
COPY --chown=nonroot:nonroot apps/mcp-server/configs /app/configs

# Set working directory
WORKDIR /app

# Use non-root user
USER nonroot

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/mcp-server", "-health-check"]

# Minimal environment variables
ENV PORT=8080 \
    GIN_MODE=release

# Metadata labels
ARG VERSION
ARG COMMIT_SHA  
ARG BUILD_DATE
LABEL org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_DATE}"

# Run the application
ENTRYPOINT ["/app/mcp-server"]