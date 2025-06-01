# Security Guide

## Overview
This guide covers security best practices, implementation guidelines, and compliance requirements for DevOps MCP.

## Quick Security Checklist

- [ ] Enable TLS 1.3 for all connections
- [ ] Configure authentication (JWT/API Keys)
- [ ] Enable audit logging
- [ ] Set up network policies
- [ ] Configure secrets management
- [ ] Enable rate limiting
- [ ] Set up WAF rules
- [ ] Configure RBAC
- [ ] Enable monitoring/alerting
- [ ] Review security scanning

## Security Architecture

### Defense in Depth

```
┌─────────────────────────────────────────────────┐
│                   WAF/CDN                       │
├─────────────────────────────────────────────────┤
│                Load Balancer                    │
│              (TLS Termination)                  │
├─────────────────────────────────────────────────┤
│                  Ingress                        │
│            (Network Policies)                   │
├─────────────────────────────────────────────────┤
│    ┌─────────┐  ┌─────────┐  ┌─────────┐     │
│    │   MCP   │  │  REST   │  │ Worker  │     │
│    │ Server  │  │   API   │  │         │     │
│    └────┬────┘  └────┬────┘  └────┬────┘     │
│         │            │             │           │
├─────────┴────────────┴─────────────┴───────────┤
│              Service Mesh                       │
│           (mTLS, Policies)                     │
├─────────────────────────────────────────────────┤
│    ┌─────────┐  ┌─────────┐  ┌─────────┐     │
│    │Postgres │  │  Redis  │  │   S3    │     │
│    │(Encrypt)│  │  (TLS)  │  │  (SSE)  │     │
│    └─────────┘  └─────────┘  └─────────┘     │
└─────────────────────────────────────────────────┘
```

## Network Security

### Firewall Rules

```yaml
# kubernetes/network-policies/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: mcp-server-ingress
spec:
  podSelector:
    matchLabels:
      app: mcp-server
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: ingress-nginx
      ports:
        - protocol: TCP
          port: 8080
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: postgres
      ports:
        - protocol: TCP
          port: 5432
    - to:
        - podSelector:
            matchLabels:
              app: redis
      ports:
        - protocol: TCP
          port: 6379
```

### TLS Configuration

```yaml
# kubernetes/tls/tls-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tls-config
data:
  tls.conf: |
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    ssl_session_timeout 1d;
    ssl_session_cache shared:SSL:50m;
    ssl_stapling on;
    ssl_stapling_verify on;
```

## Data Security

### Encryption at Rest

```yaml
# PostgreSQL Encryption
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-encryption
data:
  postgresql.conf: |
    # Enable data checksums
    data_checksums = on
    
    # Use encrypted connections
    ssl = on
    ssl_cert_file = '/etc/ssl/certs/server.crt'
    ssl_key_file = '/etc/ssl/private/server.key'
    ssl_ca_file = '/etc/ssl/certs/ca.crt'
    
    # Force encrypted connections
    ssl_min_protocol_version = 'TLSv1.2'
```

### Encryption in Transit

```go
// TLS Configuration for Services
func NewTLSConfig() *tls.Config {
    return &tls.Config{
        MinVersion:               tls.VersionTLS12,
        PreferServerCipherSuites: true,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
        },
        CurvePreferences: []tls.CurveID{
            tls.X25519,
            tls.CurveP256,
        },
    }
}
```

## Secrets Management

### Kubernetes Secrets

```yaml
# Use Sealed Secrets for GitOps
apiVersion: bitnami.com/v1alpha1
kind: SealedSecret
metadata:
  name: mcp-secrets
spec:
  encryptedData:
    db-password: AgA... # Encrypted value
    jwt-secret: AgB...  # Encrypted value
    api-keys: AgC...    # Encrypted value
```

### HashiCorp Vault Integration

```yaml
# kubernetes/vault/vault-injector.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: mcp-server
  annotations:
    vault.hashicorp.com/agent-inject: "true"
    vault.hashicorp.com/role: "mcp-server"
    vault.hashicorp.com/agent-inject-secret-db: "database/creds/mcp"
    vault.hashicorp.com/agent-inject-template-db: |
      {{- with secret "database/creds/mcp" -}}
      export DB_USERNAME="{{ .Data.username }}"
      export DB_PASSWORD="{{ .Data.password }}"
      {{- end }}
```

### Environment Variable Security

```go
// Secure environment variable handling
func GetSecureEnv(key string, required bool) (string, error) {
    value := os.Getenv(key)
    if value == "" && required {
        return "", fmt.Errorf("required environment variable %s not set", key)
    }
    
    // Clear from environment after reading
    os.Unsetenv(key)
    
    return value, nil
}
```

## Access Control (RBAC)

### Kubernetes RBAC

```yaml
# kubernetes/rbac/roles.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: mcp-server-role
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get"]
    resourceNames: ["mcp-secrets", "mcp-tls"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: mcp-server-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: mcp-server-role
subjects:
  - kind: ServiceAccount
    name: mcp-server
```

### Application RBAC

```go
// Role-based access control implementation
type Permission struct {
    Resource string
    Action   string
}

type Role struct {
    Name        string
    Permissions []Permission
}

var DefaultRoles = map[string]Role{
    "admin": {
        Name: "admin",
        Permissions: []Permission{
            {Resource: "*", Action: "*"},
        },
    },
    "developer": {
        Name: "developer",
        Permissions: []Permission{
            {Resource: "contexts", Action: "read"},
            {Resource: "contexts", Action: "write"},
            {Resource: "tools", Action: "execute"},
        },
    },
    "viewer": {
        Name: "viewer",
        Permissions: []Permission{
            {Resource: "contexts", Action: "read"},
            {Resource: "tools", Action: "read"},
        },
    },
}

func CheckPermission(userRole string, resource string, action string) bool {
    role, exists := DefaultRoles[userRole]
    if !exists {
        return false
    }
    
    for _, perm := range role.Permissions {
        if (perm.Resource == "*" || perm.Resource == resource) &&
           (perm.Action == "*" || perm.Action == action) {
            return true
        }
    }
    
    return false
}
```

## Security Scanning

### Container Scanning

```yaml
# .github/workflows/security-scan.yml
name: Security Scan
on: [push, pull_request]

jobs:
  trivy-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Run Trivy vulnerability scanner
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: 'devops-mcp:${{ github.sha }}'
          format: 'sarif'
          output: 'trivy-results.sarif'
          severity: 'CRITICAL,HIGH'
      
      - name: Upload Trivy scan results
        uses: github/codeql-action/upload-sarif@v2
        with:
          sarif_file: 'trivy-results.sarif'
```

### Code Scanning

```yaml
# .github/workflows/codeql.yml
name: "CodeQL"
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Initialize CodeQL
        uses: github/codeql-action/init@v2
        with:
          languages: go
      
      - name: Autobuild
        uses: github/codeql-action/autobuild@v2
      
      - name: Perform CodeQL Analysis
        uses: github/codeql-action/analyze@v2
```

### Dependency Scanning

```bash
# Check for vulnerabilities
go list -json -deps | nancy sleuth

# Update dependencies
go get -u ./...
go mod tidy
go mod verify
```

## Audit Logging

### Structured Audit Logs

```go
type AuditLog struct {
    Timestamp   time.Time              `json:"timestamp"`
    UserID      string                 `json:"user_id"`
    TenantID    string                 `json:"tenant_id"`
    Action      string                 `json:"action"`
    Resource    string                 `json:"resource"`
    ResourceID  string                 `json:"resource_id"`
    Result      string                 `json:"result"`
    IP          string                 `json:"ip"`
    UserAgent   string                 `json:"user_agent"`
    Duration    time.Duration          `json:"duration_ms"`
    Error       string                 `json:"error,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

func LogAuditEvent(ctx context.Context, event AuditLog) {
    // Add trace ID
    if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
        event.Metadata["trace_id"] = span.SpanContext().TraceID().String()
    }
    
    // Log to structured logger
    logger.Info("audit_event",
        zap.String("user_id", event.UserID),
        zap.String("action", event.Action),
        zap.String("resource", event.Resource),
        zap.String("result", event.Result),
        zap.Any("metadata", event.Metadata),
    )
    
    // Send to SIEM
    siemClient.Send(event)
}
```

### Audit Log Retention

```yaml
# kubernetes/audit/audit-policy.yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: RequestResponse
    omitStages:
      - RequestReceived
    users: ["system:serviceaccount:mcp:*"]
    verbs: ["get", "list", "watch"]
    resources:
      - group: ""
        resources: ["secrets", "configmaps"]
    namespaces: ["mcp"]
  
  - level: Metadata
    omitStages:
      - RequestReceived
```

## Compliance

### SOC2 Requirements

1. **Access Control**
   - Multi-factor authentication
   - Regular access reviews
   - Least privilege principle
   - Session timeout

2. **Change Management**
   - Code review requirements
   - Automated testing
   - Deployment approvals
   - Rollback procedures

3. **Monitoring**
   - Real-time alerting
   - Log aggregation
   - Performance monitoring
   - Security event monitoring

### GDPR Compliance

```go
// Data privacy implementation
type PrivacyManager struct {
    encryptor Encryptor
}

func (pm *PrivacyManager) AnonymizeUser(userID string) error {
    // Replace PII with anonymized data
    return pm.db.Transaction(func(tx *sql.Tx) error {
        _, err := tx.Exec(`
            UPDATE users 
            SET email = concat('anon-', id, '@example.com'),
                name = concat('User-', id),
                phone = NULL,
                address = NULL
            WHERE id = $1
        `, userID)
        return err
    })
}

func (pm *PrivacyManager) ExportUserData(userID string) ([]byte, error) {
    // Export all user data for GDPR requests
    var data UserDataExport
    
    // Collect from all tables
    if err := pm.collectUserData(userID, &data); err != nil {
        return nil, err
    }
    
    return json.Marshal(data)
}
```

### HIPAA Compliance

- Encryption of PHI at rest and in transit
- Access controls and audit logs
- Business Associate Agreements (BAAs)
- Regular security assessments
- Incident response procedures

## Security Headers

```go
// Security headers middleware
func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-XSS-Protection", "1; mode=block")
        c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        c.Header("Content-Security-Policy", "default-src 'self'")
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
        
        c.Next()
    }
}
```

## Incident Response

### Response Plan

1. **Detection** (0-15 min)
   - Alert triggered
   - Initial assessment
   - Severity classification

2. **Containment** (15-60 min)
   - Isolate affected systems
   - Preserve evidence
   - Prevent spread

3. **Investigation** (1-4 hours)
   - Root cause analysis
   - Impact assessment
   - Timeline reconstruction

4. **Recovery** (4-24 hours)
   - Remove threat
   - Restore services
   - Verify integrity

5. **Post-Incident** (1-7 days)
   - Lessons learned
   - Update procedures
   - Improve defenses

### Emergency Contacts

```yaml
# kubernetes/configmaps/emergency-contacts.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: emergency-contacts
data:
  contacts.yaml: |
    on_call:
      primary: "+1-555-0123"
      secondary: "+1-555-0124"
      escalation: "+1-555-0125"
    
    security_team:
      email: "security@example.com"
      slack: "#security-incidents"
      pagerduty: "security-team"
```

## Security Best Practices

### Development

1. **Code Security**
   ```go
   // Use parameterized queries
   query := "SELECT * FROM users WHERE id = $1"
   rows, err := db.Query(query, userID)
   
   // Validate input
   if !isValidEmail(email) {
       return errors.New("invalid email format")
   }
   
   // Sanitize output
   safeName := html.EscapeString(user.Name)
   ```

2. **Dependency Management**
   ```bash
   # Regular updates
   go get -u ./...
   go mod tidy
   
   # Vulnerability scanning
   nancy sleuth < go.list
   ```

3. **Secret Handling**
   ```go
   // Never log secrets
   logger.Info("Connecting to database",
       zap.String("host", dbHost),
       zap.String("user", dbUser),
       // Never: zap.String("password", dbPass)
   )
   ```

### Deployment

1. **Image Security**
   ```dockerfile
   # Use minimal base images
   FROM gcr.io/distroless/static:nonroot
   
   # Run as non-root
   USER nonroot:nonroot
   
   # Copy only necessary files
   COPY --chown=nonroot:nonroot mcp-server /
   ```

2. **Pod Security**
   ```yaml
   securityContext:
     runAsNonRoot: true
     runAsUser: 65534
     fsGroup: 65534
     seccompProfile:
       type: RuntimeDefault
     capabilities:
       drop:
         - ALL
     readOnlyRootFilesystem: true
   ```

## Security Testing

### Penetration Testing

```bash
# API Security Testing
python3 -m pytest security_tests/

# OWASP ZAP Scanning
docker run -t owasp/zap2docker-stable zap-baseline.py \
  -t https://api.example.com \
  -r security-report.html
```

### Security Benchmarks

| Metric | Target | Current |
|--------|--------|---------|
| TLS Version | ≥ 1.2 | 1.3 |
| Auth Response Time | < 100ms | 45ms |
| Failed Auth Rate | < 1% | 0.3% |
| Encryption Coverage | 100% | 100% |
| Vulnerability Score | < 4.0 | 2.1 |

## Compliance Checklist

- [ ] All data encrypted at rest
- [ ] All connections use TLS 1.2+
- [ ] Authentication required for all endpoints
- [ ] Audit logging enabled
- [ ] Regular security scans scheduled
- [ ] Incident response plan documented
- [ ] Access reviews conducted quarterly
- [ ] Security training completed
- [ ] Penetration testing performed
- [ ] Compliance audit passed