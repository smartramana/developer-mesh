# MCP Server Configuration Guide

This guide provides detailed information about configuring the MCP Server for your environment.

## Configuration Methods

The MCP Server can be configured using:

1. YAML configuration file
2. Environment variables
3. Command-line flags (limited options)

## Configuration File

The default configuration file is located at `configs/config.yaml`. You can use the template at `configs/config.yaml.template` as a starting point.

### File Location

The server looks for the configuration file in the following locations:

1. Path specified by the `MCP_CONFIG_FILE` environment variable
2. `./configs/config.yaml` (relative to the current working directory)
3. `/etc/mcp/config.yaml` (system-wide configuration)

### Configuration Sections

The configuration file is organized into the following sections:

#### API Server Configuration

```yaml
api:
  listen_address: ":8080"          # Address and port to listen on
  read_timeout: 30s                # HTTP read timeout
  write_timeout: 30s               # HTTP write timeout
  idle_timeout: 90s                # HTTP idle timeout
  base_path: "/api/v1"             # Base path for API endpoints
  enable_cors: true                # Enable CORS support
  log_requests: true               # Log all HTTP requests
  
  # TLS configuration (strongly recommended for production)
  tls_cert_file: "/path/to/cert.pem"  # Path to TLS certificate file
  tls_key_file: "/path/to/key.pem"    # Path to TLS key file
  
  # API rate limiting
  rate_limit:
    enabled: true                  # Enable rate limiting
    limit: 100                     # Requests per hour per client
    burst: 150                     # Burst limit
    expiration: 1h                 # Expiration time for rate limit counters
  
  # Webhook configuration
  webhooks:
    github:
      enabled: true                # Enable GitHub webhooks
      secret: "${GITHUB_WEBHOOK_SECRET}"  # Secret for verifying webhook signatures
      path: "/github"              # Custom path for the webhook endpoint
    
  # Authentication configuration
  auth:
    jwt_secret: "your-jwt-secret"  # Secret for signing JWT tokens
    jwt_expiration: 24h            # JWT token expiration time
    api_keys:                      # List of valid API keys
      - "api-key-1"
      - "api-key-2"
    require_auth: true             # Require authentication for API endpoints
    allowed_user_roles:            # List of allowed user roles
      - "admin"
      - "operator"
    token_renewal_threshold: 1h    # Time before expiration to renew tokens
```

#### Database Configuration

```yaml
database:
  driver: "postgres"                # Database driver (postgres only for now)
  host: "localhost"                 # Database host
  port: 5432                        # Database port
  username: "postgres"              # Database username
  password: "postgres"              # Database password
  database: "mcp"                   # Database name
  ssl_mode: "disable"               # SSL mode (disable, require, verify-ca, verify-full)
  
  # Connection string (alternative to individual settings)
  dsn: "postgres://postgres:postgres@localhost:5432/mcp?sslmode=disable"
  
  # Connection pool settings
  max_open_conns: 25               # Maximum number of open connections
  max_idle_conns: 5                # Maximum number of idle connections
  conn_max_lifetime: 5m            # Maximum lifetime of a connection
```

#### Cache Configuration

```yaml
cache:
  type: "redis"                     # Cache type (redis only for now)
  address: "localhost:6379"         # Redis address
  password: ""                      # Redis password
  database: 0                       # Redis database number
  
  # Connection settings
  max_retries: 3                    # Maximum number of retries
  dial_timeout: 5s                  # Timeout for establishing new connections
  read_timeout: 3s                  # Timeout for socket reads
  write_timeout: 3s                 # Timeout for socket writes
  
  # Connection pool settings
  pool_size: 10                     # Maximum number of connections
  min_idle_conns: 2                 # Minimum number of idle connections
  pool_timeout: 4s                  # Timeout for getting connection from pool
```

#### Core Engine Configuration

```yaml
engine:
  event_buffer_size: 1000           # Size of the event buffer
  concurrency_limit: 5              # Maximum number of concurrent event processors
  event_timeout: 30s                # Timeout for processing events
  
  # GitHub Configuration
  github:
    api_token: "${GITHUB_API_TOKEN}"       # GitHub API token
    webhook_secret: "${GITHUB_WEBHOOK_SECRET}"  # GitHub webhook secret
    request_timeout: 10s                   # Timeout for API requests
    rate_limit_per_hour: 5000              # Maximum API requests per hour
    max_retries: 3                         # Maximum number of retries for API requests
    retry_delay: 1s                        # Delay between retries
    mock_responses: false                  # Use mock responses instead of real API
    mock_url: "http://localhost:8081/mock-github"  # URL for mock server
```

#### Metrics Configuration

```yaml
metrics:
  enabled: true                     # Enable metrics collection
  type: "prometheus"                # Metrics type (prometheus only for now)
  endpoint: "localhost:9090"        # Prometheus endpoint
  push_gateway: ""                  # Prometheus push gateway (optional)
  push_interval: 10s                # Interval for pushing metrics
```

## Environment Variables

All configuration options can be set using environment variables with the `MCP_` prefix, followed by the configuration path with underscores.

Examples:

- `MCP_API_LISTEN_ADDRESS=:8080` sets the API listen address
- `MCP_DATABASE_DSN=postgres://user:password@localhost:5432/mcp` sets the database connection string
- `MCP_ENGINE_GITHUB_API_TOKEN=your_token` sets the GitHub API token

For nested configuration, use underscores to separate levels:

- `MCP_API_RATE_LIMIT_ENABLED=true` sets the rate limiting enabled flag
- `MCP_ENGINE_GITHUB_REQUEST_TIMEOUT=10s` sets the GitHub request timeout

## Command-Line Flags

Some common configuration options can be set using command-line flags when starting the server:

```bash
./mcp-server --config /path/to/config.yaml --api-listen-address :8080 --log-level debug
```

Available flags:

- `--config`: Path to the configuration file
- `--api-listen-address`: Address and port to listen on
- `--log-level`: Logging level (debug, info, warn, error)
- `--version`: Show version information

## Configuration Priority

Configuration values are loaded in the following order, with later values overriding earlier ones:

1. Default values
2. Configuration file
3. Environment variables
4. Command-line flags

## Sensitive Information

Sensitive information (like API tokens and passwords) should be stored securely and not committed to version control. You can:

1. Use environment variables for sensitive values (recommended approach)
2. Use a secret management system (like HashiCorp Vault or AWS Secrets Manager)
3. Use Kubernetes secrets when deploying on Kubernetes

### Environment Variable Placeholders

The configuration files use environment variable placeholders like `${VARIABLE_NAME:-default}` which will:
- Use the value of the environment variable if it exists
- Fall back to the default value after the `:-` if the environment variable is not set

Example:
```yaml
database:
  password: "${MCP_DATABASE_PASSWORD:-postgres}"
```

This will use the value of `MCP_DATABASE_PASSWORD` if it's set, or fall back to "postgres" if it's not.

### Security Best Practices for Configuration

1. **Never commit secrets to version control** - Use the template files as a guide but keep your actual configuration with real secrets outside of version control
2. **Use environment variables** - Set sensitive values as environment variables rather than hardcoding them in configuration files
3. **Encrypt sensitive data at rest** - Enable S3 server-side encryption for stored context data
4. **Enable TLS for production** - Always use TLS certificates for production deployments
5. **Regularly rotate secrets** - Change API keys, passwords, and other secrets regularly
6. **Use least privilege principle** - Give services and users only the permissions they need

## Testing Configuration

To validate your configuration without starting the server:

```bash
./mcp-server --validate-config --config /path/to/config.yaml
```

## S3 Storage Configuration

The MCP Server now supports storing context data in Amazon S3 or S3-compatible storage services.

```yaml
storage:
  # Storage provider type: "local" or "s3"
  type: "s3"
  
  # S3 Storage Configuration
  s3:
    region: "us-west-2"                          # AWS region
    bucket: "mcp-contexts"                        # S3 bucket name
    endpoint: "http://localhost:4566"             # Optional: custom endpoint for S3-compatible services
    force_path_style: true                        # Optional: required for some S3-compatible services
    server_side_encryption: "AES256"              # Optional: enable server-side encryption
    kms_key_id: ""                                # Optional: KMS key ID for aws:kms encryption
    upload_part_size: 5242880                     # Upload multipart size (5MB)
    download_part_size: 5242880                   # Download multipart size (5MB)
    concurrency: 5                                # Number of concurrent upload/download parts
    request_timeout: 30s                          # Timeout for S3 operations
  
  # Context Storage Configuration
  context_storage:
    # Provider: "database" or "s3"
    provider: "s3"                                # Use S3 for context storage
    s3_path_prefix: "contexts"                    # Prefix for S3 keys
```

### S3 Security Best Practices

1. **Bucket Policy**: Configure your S3 bucket with appropriate policies that follow the principle of least privilege
2. **Encryption**: Enable server-side encryption for data at rest (AES256 or aws:kms)
3. **IAM Roles**: Use IAM roles instead of hardcoded access keys when possible
4. **Access Logging**: Enable S3 access logging for audit purposes
5. **Versioning**: Consider enabling bucket versioning to protect against accidental deletions
6. **Lifecycle Policies**: Implement lifecycle policies to manage context data retention

## Mock Mode

For development and testing, you can configure the GitHub adapter to use mock responses instead of connecting to the real GitHub API:

```yaml
engine:
  github:
    mock_responses: true
    mock_url: "http://localhost:8081/mock-github"
```

When running with Docker Compose, mock mode is enabled by default for the GitHub adapter.