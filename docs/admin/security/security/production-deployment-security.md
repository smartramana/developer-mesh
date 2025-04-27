# Production Deployment Security Guide

This document outlines essential security considerations for deploying the MCP Server in production environments.

## Secure Port Configuration

### Development vs. Production Ports

In the default configuration templates and examples, the MCP server is configured to use port 8080:

```yaml
api:
  listen_address: ":8080"
```

While this is suitable for development environments, it is **not recommended for production deployments** for the following reasons:

1. Port 8080 is an unencrypted HTTP port
2. Production services should use HTTPS (TLS/SSL) for all communications
3. Using standard ports helps with firewall configurations and security audits

### Recommended Production Configuration

For production deployments, you should configure the MCP server to use port 443 (the standard HTTPS port) with proper TLS certificates:

```yaml
api:
  listen_address: ":443"
  tls_cert_file: "/path/to/cert.pem"
  tls_key_file: "/path/to/key.pem"
```

### Deployment Options

There are two main approaches to enabling HTTPS for the MCP server in production:

#### 1. Direct TLS Termination by MCP Server

Configure the MCP server to handle TLS directly:

```yaml
api:
  listen_address: ":443"
  tls_cert_file: "/path/to/cert.pem"
  tls_key_file: "/path/to/key.pem"
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 60s
```

This approach requires:
- Valid TLS certificates
- Proper certificate renewal process
- Direct access to port 443 (may require running as root or using capabilities)

#### 2. Reverse Proxy Configuration (Recommended)

A more common approach is to use a reverse proxy like Nginx, Apache, or a cloud load balancer:

1. Configure the MCP server to listen on an internal port:
   ```yaml
   api:
     listen_address: ":8080"
   ```

2. Set up a reverse proxy to handle TLS termination:

   **Nginx Example:**
   ```nginx
   server {
       listen 443 ssl;
       server_name mcp.example.com;

       ssl_certificate /path/to/cert.pem;
       ssl_certificate_key /path/to/key.pem;
       ssl_protocols TLSv1.2 TLSv1.3;
       ssl_ciphers HIGH:!aNULL:!MD5;

       location / {
           proxy_pass http://localhost:8080;
           proxy_set_header Host $host;
           proxy_set_header X-Real-IP $remote_addr;
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_set_header X-Forwarded-Proto $scheme;
       }
   }
   ```

   **Kubernetes Ingress Example:**
   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: Ingress
   metadata:
     name: mcp-ingress
     annotations:
       kubernetes.io/ingress.class: "nginx"
       cert-manager.io/cluster-issuer: "letsencrypt-prod"
   spec:
     tls:
     - hosts:
       - mcp.example.com
       secretName: mcp-tls
     rules:
     - host: mcp.example.com
       http:
         paths:
         - path: /
           pathType: Prefix
           backend:
             service:
               name: mcp-server
               port:
                 number: 8080
   ```

This approach has several advantages:
- Specialized tools for TLS management
- Automatic certificate renewal (when using cert-manager)
- Additional security features (WAF, rate limiting, etc.)
- No need to run the MCP server with elevated privileges

## HTTP to HTTPS Redirection

Always ensure that HTTP requests are redirected to HTTPS:

**Nginx Example:**
```nginx
server {
    listen 80;
    server_name mcp.example.com;
    return 301 https://$host$request_uri;
}
```

**Kubernetes Ingress Example:**
```yaml
metadata:
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
```

## TLS Configuration Best Practices

1. **Use Modern TLS Versions**:
   - Enable only TLS 1.2 and TLS 1.3
   - Disable older SSL and TLS versions (SSL 3.0, TLS 1.0, TLS 1.1)

2. **Strong Cipher Suites**:
   - Use strong, modern cipher suites
   - Disable weak ciphers and algorithms

3. **HTTP Security Headers**:
   - Strict-Transport-Security (HSTS)
   - Content-Security-Policy
   - X-Content-Type-Options
   - X-Frame-Options

4. **Certificate Management**:
   - Use certificates from trusted CAs
   - Implement automated certificate renewal
   - Monitor certificate expiration

## Internal Network Security

Even when using a reverse proxy for TLS termination, it's important to secure internal communications:

1. **Service Mesh**:
   - Consider using a service mesh like Istio or Linkerd for internal mTLS
   - Encrypt all pod-to-pod communication

2. **Network Policies**:
   - Restrict pod-to-pod communication with Kubernetes Network Policies
   - Allow only necessary connections

3. **Private Subnets**:
   - Deploy services in private subnets/networks
   - Use VPC/VNET for isolation

## Security Checklist for Production

Before deploying to production, ensure:

1. **TLS Configuration**:
   - HTTPS enabled on port 443
   - Modern TLS versions only (1.2+)
   - Strong cipher suites
   - Valid certificates from trusted CAs

2. **Authentication**:
   - Secure API keys or JWT tokens
   - Strong, randomly generated secrets
   - Regular credential rotation

3. **Network Security**:
   - Proper firewall rules
   - Network policies limiting access
   - No direct exposure of internal services

4. **Monitoring**:
   - TLS certificate monitoring
   - Security event logging
   - Regular security scanning

## Automated Security Scanning

Implement regular security scanning of your deployment:

1. **TLS/SSL Scanning**:
   ```bash
   # Using ssllabs-scan
   ssllabs-scan --grade -quiet mcp.example.com
   
   # Or using testssl.sh
   testssl.sh mcp.example.com
   ```

2. **Container Scanning**:
   ```bash
   # Using Trivy
   trivy image your-registry/mcp-server:latest
   ```

3. **Kubernetes Security Scanning**:
   ```bash
   # Using kube-bench
   kube-bench
   
   # Using kubesec
   kubesec scan deployment.yaml
   ```

By following these guidelines, you can ensure that your MCP Server deployment is secure and follows industry best practices for production environments.