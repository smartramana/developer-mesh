# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /workspace

# Copy go workspace files
COPY go.work ./

# Handle go.work.sum in a way that won't fail if the file doesn't exist
# This is standard practice for Go workspaces in CI environments
RUN touch go.work.sum
COPY go.work.sum* ./ || true

# Copy all module files
COPY apps/mockserver/go.mod apps/mockserver/go.sum ./apps/mockserver/
COPY pkg/go.mod pkg/go.sum ./pkg/

# Download dependencies
RUN go mod download

# Copy source code
COPY apps/mockserver/ ./apps/mockserver/
COPY pkg/ ./pkg/

# Build the application
WORKDIR /workspace/apps/mockserver
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mockserver ./cmd/main.go

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /workspace/apps/mockserver/mockserver .

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8081

# Run the application
CMD ["./mockserver"]