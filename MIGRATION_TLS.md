# TLS Configuration Migration Guide

## Breaking Changes

This update introduces breaking changes to TLS configuration. Please update your configurations before upgrading.

### API Configuration Changes

#### Before (Deprecated)
```yaml
api:
  tls_cert_file: /path/to/cert.pem
  tls_key_file: /path/to/key.pem
```

#### After (New Structure)
```yaml
api:
  tls:
    enabled: true
    cert_file: /path/to/cert.pem
    key_file: /path/to/key.pem
    min_version: "1.3"  # Optional, defaults to 1.3
```

### ElastiCache Configuration Changes

#### Before (Deprecated)
```yaml
cache:
  redis:
    use_tls: true
    insecure_skip_verify: true
```

#### After (New Structure)
```yaml
cache:
  redis:
    tls:
      enabled: true
      insecure_skip_verify: true
      min_version: "1.2"  # Optional
```

## Environment Variable Changes

### API Server

| Old Variable | New Variable | Notes |
|-------------|--------------|-------|
| `API_TLS_CERT_FILE` | `API_TLS_CERT_FILE` | Path now under `api.tls` in config |
| `API_TLS_KEY_FILE` | `API_TLS_KEY_FILE` | Path now under `api.tls` in config |
| N/A | `API_TLS_ENABLED` | New: explicitly enable/disable TLS |
| N/A | `API_TLS_MIN_VERSION` | New: set minimum TLS version |

### Cache/Redis

| Old Variable | New Variable | Notes |
|-------------|--------------|-------|
| `REDIS_USE_TLS` | `REDIS_TLS_ENABLED` | Now under `tls` sub-config |
| `REDIS_INSECURE_SKIP_VERIFY` | `REDIS_TLS_INSECURE_SKIP_VERIFY` | Now under `tls` sub-config |

## Quick Migration Steps

1. **Update Configuration Files**
   ```bash
   # Backup existing configs
   cp configs/config.*.yaml configs/backup/
   
   # Update YAML files with new structure
   # See examples above
   ```

2. **Update Environment Variables**
   ```bash
   # In your .env file or deployment scripts
   # Replace old variables with new ones
   ```

3. **Test Configuration**
   ```bash
   # Validate configuration loads correctly
   MCP_CONFIG_FILE=configs/config.development.yaml ./apps/mcp-server/mcp-server --validate-config
   ```

4. **Generate Certificates (if needed)**
   ```bash
   # For development/testing
   ./scripts/certs/generate-dev-certs.sh
   ```

## Default Behavior

- **TLS is DISABLED by default** in development
- Minimum TLS version is 1.3 when enabled
- Only secure cipher suites are allowed
- Perfect Forward Secrecy (PFS) is required

## Examples

### Development Configuration (TLS Disabled)
```yaml
api:
  listen_address: ":8080"
  tls:
    enabled: false  # Default
```

### Production Configuration (TLS Enabled)
```yaml
api:
  listen_address: ":443"
  tls:
    enabled: true
    cert_file: "/etc/certs/tls.crt"
    key_file: "/etc/certs/tls.key"
    min_version: "1.3"
    verify_certificates: true
    
    # Performance optimizations
    session_tickets: true
    session_cache_size: 1000
```

### ElastiCache with SSH Tunnel
```yaml
cache:
  redis:
    addr: "localhost:6379"
    tls:
      enabled: true
      insecure_skip_verify: true  # Required for SSH tunnel
```

## Troubleshooting

### Service Won't Start
- Check certificate file paths exist and are readable
- Verify certificate and key match
- Check file permissions (should be readable by service user)

### TLS Handshake Errors
- Ensure client supports TLS 1.2 minimum
- Check cipher suite compatibility
- Verify certificate validity

### Performance Issues
- Enable session resumption: `session_tickets: true`
- Increase session cache size for high traffic
- Consider TLS 1.3 for better performance

## Need Help?

- Check `TLS_TESTING.md` for testing guidance
- Review security settings with `gosec`
- Run functional tests: `make test-functional`