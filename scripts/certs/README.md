# TLS Certificate Management

This directory contains scripts for managing TLS certificates in the Developer Mesh project.

## Development Certificates

### Quick Start

```bash
# Generate development certificates
make dev-certs

# Source environment variables
source certs/dev/env-exports.sh

# Or add to your .env file
cat certs/dev/env-additions.txt >> .env
```

### What Gets Generated

The `generate-dev-certs.sh` script creates:

1. **Root CA Certificate** (`ca-cert.pem`, `ca-key.pem`)
   - Self-signed certificate authority for development
   - Used to sign all other certificates

2. **Server Certificate** (`server-cert.pem`, `server-key.pem`)
   - For HTTPS on API servers
   - Includes SANs for localhost, 127.0.0.1, Docker hostnames

3. **Client Certificate** (`client-cert.pem`, `client-key.pem`)
   - For database connections requiring client certificates
   - Can be used for mutual TLS (mTLS)

4. **Redis Certificate** (`redis-cert.pem`, `redis-key.pem`)
   - For Redis/ElastiCache connections
   - Configured to work with localhost (SSH tunnel)

### Configuration

After generating certificates, you can enable TLS in different components:

#### API Server HTTPS
```env
API_TLS_ENABLED=true
TLS_CERT_FILE=./certs/dev/server-cert.pem
TLS_KEY_FILE=./certs/dev/server-key.pem
```

#### Database TLS
```env
DATABASE_TLS_ENABLED=true
DATABASE_SSL_MODE=require  # or verify-ca, verify-full
DATABASE_TLS_CERT=./certs/dev/client-cert.pem
DATABASE_TLS_KEY=./certs/dev/client-key.pem
DATABASE_TLS_CA=./certs/dev/ca-cert.pem
```

#### Redis/ElastiCache TLS
```env
# For ElastiCache through SSH tunnel, keep insecure_skip_verify: true
REDIS_CA_CERT=./certs/dev/ca-cert.pem
```

### AWS ElastiCache with SSH Tunnel

When using ElastiCache through an SSH tunnel in development:

1. The SSH tunnel terminates TLS at the bastion host
2. Your local connection uses the tunnel (localhost:6379)
3. Keep `insecure_skip_verify: true` in development config
4. The generated certificates are compatible but not strictly necessary

### Testing TLS

```bash
# Test server certificate
openssl x509 -in certs/dev/server-cert.pem -text -noout

# Test connection with TLS
curl --cacert certs/dev/ca-cert.pem https://localhost:8081/health

# Test with client certificate (mutual TLS)
curl --cacert certs/dev/ca-cert.pem \
     --cert certs/dev/client-cert.pem \
     --key certs/dev/client-key.pem \
     https://localhost:8081/health
```

## Production Certificates

In production (Kubernetes), use [cert-manager.io](https://cert-manager.io/):

1. **cert-manager** handles certificate lifecycle
2. **Let's Encrypt** or internal CA for trusted certificates
3. **Automatic renewal** before expiration
4. **Secrets management** via Kubernetes secrets

Example cert-manager configuration:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: developer-mesh-tls
  namespace: developer-mesh
spec:
  secretName: developer-mesh-tls-secret
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  commonName: api.developer-mesh.com
  dnsNames:
  - api.developer-mesh.com
  - mcp.developer-mesh.com
```

## Security Notes

⚠️ **Development certificates are self-signed and NOT secure for production use!**

- Development certs are valid for 10 years (convenience)
- Production certs should be valid for 90 days or less
- Always use trusted CAs in production
- Enable certificate validation in production
- Use mutual TLS (mTLS) for service-to-service communication

## Troubleshooting

### Certificate Verification Failed
- Check certificate paths in environment variables
- Ensure CA certificate is trusted
- Verify certificate hasn't expired
- Check certificate common name matches hostname

### TLS Handshake Timeout
- Verify TLS versions match (client and server)
- Check cipher suite compatibility
- Ensure firewall allows TLS traffic
- Check for proxy interference

### Permission Denied
- Certificate files should be readable by the application
- Private keys should have 600 permissions
- CA certificates should have 644 permissions