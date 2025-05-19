# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy the full source code for proper dependency resolution
COPY . .

# We'll use the existing go.work file which should have all the right modules defined
# If it doesn't exist or has issues, uncomment the following block and configure as needed
#RUN if [ ! -f go.work ]; then \
#    echo "go 1.24.3" > go.work && \
#    echo "use (" >> go.work && \
#    echo "    ./apps/rest-api" >> go.work && \
#    echo "    ./pkg/common" >> go.work && \
#    echo "    ./pkg/database" >> go.work && \
#    echo "    ./pkg/models" >> go.work && \
#    echo ")" >> go.work; \
#fi

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

# Set environment variables for Redis
ENV REDIS_ADDR="redis:6379"
ENV REDIS_ADDRESS="redis:6379"
ENV REDIS_HOST="redis"
ENV REDIS_PORT="6379"
ENV REDIS_URL="redis://redis:6379"

# Run the application
CMD ["./rest-api"]
