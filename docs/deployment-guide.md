# MCP Server Deployment Guide

This guide provides detailed instructions for deploying the MCP Server in production environments.

## Deployment Options

The MCP Server can be deployed in various environments:

1. **Docker-based deployment**
   - Docker Compose (for small deployments)
   - Kubernetes (recommended for production)

2. **Virtual machines or bare metal**
   - Systemd service
   - Reverse proxy with Nginx/Apache

3. **Cloud-specific deployments**
   - AWS ECS/EKS
   - Google Cloud Run/GKE
   - Azure Container Instances/AKS

This guide will focus on Kubernetes deployment as the recommended approach for production environments.

## Pre-deployment Checklist

Before deploying the MCP Server, ensure you have:

1. **External service credentials**
   - GitHub token
   - Harness token and account ID
   - SonarQube token
   - Artifactory credentials
   - Xray credentials

2. **Database and cache**
   - PostgreSQL database (version 12+)
   - Redis cache (version 6+)

3. **Domain and TLS certificates**
   - Domain name for MCP Server
   - TLS certificates for secure communication

4. **Monitoring infrastructure**
   - Prometheus for metrics collection
   - Grafana for dashboards
   - Log aggregation system

## Kubernetes Deployment

### Prerequisites

- Kubernetes cluster (version 1.19+)
- kubectl configured to access your cluster
- Helm (optional, for package management)

### Step 1: Create Namespace

Create a dedicated namespace for MCP Server:

```bash
kubectl create namespace mcp
```

### Step 2: Create ConfigMap and Secrets

Create a ConfigMap for non-sensitive configuration:

```bash
cat <<EOF > mcp-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mcp-config
  namespace: mcp
data:
  config.yaml: |
    api:
      listen_address: ":8080"
      read_timeout: 30s
      write_timeout: 30s
      idle_timeout: 90s
      base_path: "/api/v1"
      enable_cors: true
      
      # API rate limiting
      rate_limit:
        enabled: true
        limit: 100
        burst: 150
        expiration: 1h
      
    database:
      host: "postgres"
      port: 5432
      username: "postgres"
      database: "mcp"
      ssl_mode: "disable"
      max_open_conns: 25
      max_idle_conns: 5
      conn_max_lifetime: 5m
      
    cache:
      type: "redis"
      address: "redis:6379"
      database: 0
      max_retries: 3
      pool_size: 10
      
    engine:
      event_buffer_size: 1000
      concurrency_limit: 10
      event_timeout: 30s
      
    metrics:
      enabled: true
      type: "prometheus"
EOF

kubectl apply -f mcp-config.yaml
```

Create Secrets for sensitive configuration:

```bash
kubectl create secret generic mcp-secrets \
  --namespace mcp \
  --from-literal=DATABASE_PASSWORD=your-db-password \
  --from-literal=GITHUB_API_TOKEN=your-github-token \
  --from-literal=GITHUB_WEBHOOK_SECRET=your-github-webhook-secret \
  --from-literal=HARNESS_API_TOKEN=your-harness-token \
  --from-literal=HARNESS_ACCOUNT_ID=your-harness-account \
  --from-literal=HARNESS_WEBHOOK_SECRET=your-harness-webhook-secret \
  --from-literal=SONARQUBE_TOKEN=your-sonarqube-token \
  --from-literal=SONARQUBE_WEBHOOK_SECRET=your-sonarqube-webhook-secret \
  --from-literal=ARTIFACTORY_USERNAME=your-artifactory-username \
  --from-literal=ARTIFACTORY_PASSWORD=your-artifactory-password \
  --from-literal=ARTIFACTORY_API_KEY=your-artifactory-api-key \
  --from-literal=ARTIFACTORY_WEBHOOK_SECRET=your-artifactory-webhook-secret \
  --from-literal=XRAY_USERNAME=your-xray-username \
  --from-literal=XRAY_PASSWORD=your-xray-password \
  --from-literal=XRAY_API_KEY=your-xray-api-key \
  --from-literal=XRAY_WEBHOOK_SECRET=your-xray-webhook-secret
```

For TLS certificate (if not using an ingress controller with automatic TLS):

```bash
kubectl create secret tls mcp-tls \
  --namespace mcp \
  --cert=/path/to/tls.crt \
  --key=/path/to/tls.key
```

### Step 3: Deploy Database and Cache (If Needed)

If you don't have existing database and cache services, deploy them in the cluster:

#### PostgreSQL Deployment

```bash
cat <<EOF > postgres.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  namespace: mcp
spec:
  serviceName: postgres
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:15-alpine
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_USER
          value: postgres
        - name: POSTGRES_DB
          value: mcp
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mcp-secrets
              key: DATABASE_PASSWORD
        volumeMounts:
        - name: postgres-data
          mountPath: /var/lib/postgresql/data
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
  volumeClaimTemplates:
  - metadata:
      name: postgres-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: mcp
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
EOF

kubectl apply -f postgres.yaml
```

#### Redis Deployment

```bash
cat <<EOF > redis.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: mcp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "200m"
---
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: mcp
spec:
  selector:
    app: redis
  ports:
  - port: 6379
    targetPort: 6379
EOF

kubectl apply -f redis.yaml
```

### Step 4: Deploy MCP Server

Create a deployment for the MCP Server:

```bash
cat <<EOF > mcp-server.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-server
  namespace: mcp
spec:
  replicas: 2
  selector:
    matchLabels:
      app: mcp-server
  template:
    metadata:
      labels:
        app: mcp-server
    spec:
      containers:
      - name: mcp-server
        image: your-registry/mcp-server:latest
        ports:
        - containerPort: 8080
        env:
        - name: MCP_CONFIG_FILE
          value: "/app/configs/config.yaml"
        - name: MCP_DATABASE_USERNAME
          value: "postgres"
        - name: MCP_DATABASE_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mcp-secrets
              key: DATABASE_PASSWORD
        - name: MCP_ENGINE_GITHUB_API_TOKEN
          valueFrom:
            secretKeyRef:
              name: mcp-secrets
              key: GITHUB_API_TOKEN
        - name: MCP_ENGINE_GITHUB_WEBHOOK_SECRET
          valueFrom:
            secretKeyRef:
              name: mcp-secrets
              key: GITHUB_WEBHOOK_SECRET
        # Add other environment variables for external services
        volumeMounts:
        - name: config-volume
          mountPath: /app/configs
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config-volume
        configMap:
          name: mcp-config
---
apiVersion: v1
kind: Service
metadata:
  name: mcp-server
  namespace: mcp
spec:
  selector:
    app: mcp-server
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
EOF

kubectl apply -f mcp-server.yaml
```

### Step 5: Create Ingress

Set up an ingress to expose the MCP Server:

```bash
cat <<EOF > mcp-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: mcp-ingress
  namespace: mcp
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
              number: 80
EOF

kubectl apply -f mcp-ingress.yaml
```

### Step 6: Set Up Monitoring

Deploy Prometheus and Grafana for monitoring:

```bash
# Using Helm (recommended)
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false

# Create a ServiceMonitor for MCP Server
cat <<EOF > mcp-service-monitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: mcp-service-monitor
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: mcp-server
  namespaceSelector:
    matchNames:
    - mcp
  endpoints:
  - port: http
    path: /metrics
    interval: 15s
    bearerTokenSecret:
      name: mcp-metrics-token
      key: token
EOF

kubectl apply -f mcp-service-monitor.yaml
```

## Scaling Considerations

### Horizontal Scaling

The MCP Server is designed to scale horizontally:

- Increase the number of replicas in the Deployment
- Ensure database and cache can handle increased load
- Configure proper session affinity if needed

Example scaling command:

```bash
kubectl scale deployment mcp-server -n mcp --replicas=4
```

### Vertical Scaling

For higher load scenarios, increase resources:

```bash
# Edit the deployment
kubectl edit deployment mcp-server -n mcp
```

Update resource limits:

```yaml
resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "1000m"
```

### Database Scaling

For high-load scenarios, consider:

- Using a managed database service (RDS, Cloud SQL)
- Setting up PostgreSQL replication (primary/replica)
- Implementing connection pooling (PgBouncer)

### Cache Scaling

For high-throughput deployments:

- Use Redis cluster mode for distributed caching
- Configure proper cache eviction policies
- Monitor cache hit/miss rates

## High Availability Configuration

### Multi-Zone Deployment

For high availability, deploy across multiple zones:

```yaml
spec:
  topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: topology.kubernetes.io/zone
    whenUnsatisfiable: DoNotSchedule
    labelSelector:
      matchLabels:
        app: mcp-server
```

### Database HA

For database high availability:

- Use a managed PostgreSQL service with HA
- Set up PostgreSQL streaming replication
- Implement automatic failover

### Cache HA

For cache high availability:

- Use Redis Sentinel or Redis Cluster
- Deploy across multiple zones
- Configure proper persistence and backup

## Security Considerations

### Network Security

1. **API Authentication**:
   - Ensure all API endpoints require authentication
   - Use secure API keys or JWT tokens
   - Implement proper RBAC

2. **TLS Everywhere**:
   - Enable TLS for all services
   - Use modern TLS versions (1.2+)
   - Configure proper cipher suites

3. **Network Policies**:
   - Restrict pod-to-pod communication
   - Implement service mesh for mTLS

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: mcp-network-policy
  namespace: mcp
spec:
  podSelector:
    matchLabels:
      app: mcp-server
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: ingress-nginx
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
  - to:
    - ipBlock:
        cidr: 0.0.0.0/0
        except:
        - 10.0.0.0/8
        - 172.16.0.0/12
        - 192.168.0.0/16
    ports:
    - protocol: TCP
      port: 443
```

### Secret Management

1. **Kubernetes Secrets**:
   - Use Kubernetes secrets for sensitive data
   - Consider encryption at rest for secrets

2. **External Secret Management**:
   - Consider HashiCorp Vault or AWS Secrets Manager
   - Use a Kubernetes operator for secret injection

3. **Rotate Credentials**:
   - Implement regular credential rotation
   - Use short-lived API tokens when possible

## Update and Rollback Strategy

### Rolling Updates

Configure Kubernetes to perform rolling updates:

```yaml
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 25%
      maxSurge: 25%
```

### Blue-Green Deployment

For zero-downtime updates, consider blue-green deployment:

1. Deploy new version alongside old version
2. Verify new version with health checks
3. Switch traffic to new version
4. Keep old version for quick rollback

### Canary Deployment

For gradual rollout, implement canary deployment:

1. Deploy new version with minimal replicas
2. Route a small percentage of traffic to new version
3. Monitor for errors and performance issues
4. Gradually increase traffic to new version

## Backup and Disaster Recovery

### Database Backup

Implement regular database backups:

```bash
# Create a CronJob for PostgreSQL backup
cat <<EOF > postgres-backup.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: postgres-backup
  namespace: mcp
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: postgres-backup
            image: postgres:15-alpine
            command:
            - /bin/sh
            - -c
            - |
              pg_dump -h postgres -U postgres -d mcp | gzip > /backup/mcp-\$(date +%Y%m%d%H%M%S).sql.gz
            env:
            - name: PGPASSWORD
              valueFrom:
                secretKeyRef:
                  name: mcp-secrets
                  key: DATABASE_PASSWORD
            volumeMounts:
            - name: backup-volume
              mountPath: /backup
          restartPolicy: OnFailure
          volumes:
          - name: backup-volume
            persistentVolumeClaim:
              claimName: postgres-backup-pvc
EOF

kubectl apply -f postgres-backup.yaml
```

### Disaster Recovery Plan

1. **Regular Backups**:
   - Database (daily)
   - Configuration (with changes)
   - Logs (retained for 30+ days)

2. **Recovery Testing**:
   - Test restore process regularly
   - Document recovery procedures
   - Train team on recovery process

3. **Recovery Time Objective (RTO)**:
   - Define acceptable downtime
   - Implement procedures to meet RTO
   - Test recovery time regularly

## Production Checklist

Before going to production, ensure:

1. **Security**:
   - All sensitive data is encrypted
   - All connections use TLS
   - Authentication is properly configured
   - Network policies are in place

2. **Performance**:
   - Load testing has been performed
   - Resource limits are properly set
   - Scaling strategy is defined
   - Database and cache are properly sized

3. **Monitoring**:
   - Metrics collection is configured
   - Alerting is set up
   - Logs are being collected
   - Dashboards are available

4. **Reliability**:
   - High availability is configured
   - Backup and restore is tested
   - Disaster recovery plan is documented
   - Failover procedures are tested

5. **Operations**:
   - Update and rollback procedures are documented
   - Troubleshooting guides are available
   - Monitoring dashboards are set up
   - On-call procedures are defined

## Cloud-Specific Deployments

### AWS Deployment

For deploying on AWS:

1. **EKS Cluster**:
   - Use managed node groups
   - Implement auto-scaling
   - Use private subnets

2. **RDS for PostgreSQL**:
   - Enable multi-AZ
   - Configure automatic backups
   - Set up parameter groups

3. **ElastiCache for Redis**:
   - Use Redis cluster mode
   - Enable automatic failover
   - Configure subnet groups

4. **ALB for Ingress**:
   - Configure TLS termination
   - Set up WAF rules
   - Implement proper health checks

### Google Cloud Deployment

For deploying on GCP:

1. **GKE Cluster**:
   - Use node auto-provisioning
   - Enable Workload Identity
   - Use regional clusters

2. **Cloud SQL for PostgreSQL**:
   - Configure high availability
   - Set up maintenance windows
   - Enable automated backups

3. **Memorystore for Redis**:
   - Use Redis version 6.x+
   - Configure proper tier
   - Enable read replicas

4. **Cloud Load Balancer**:
   - Configure SSL certificates
   - Set up IAP for secure access
   - Implement content-based routing

### Azure Deployment

For deploying on Azure:

1. **AKS Cluster**:
   - Use virtual node scaling
   - Implement pod security policies
   - Use availability zones

2. **Azure Database for PostgreSQL**:
   - Enable geo-redundant backups
   - Configure high availability
   - Set up server parameters

3. **Azure Cache for Redis**:
   - Use Premium tier for clustering
   - Enable data persistence
   - Configure firewall rules

4. **Application Gateway**:
   - Implement WAF policies
   - Configure TLS settings
   - Set up health probes

## Post-Deployment Verification

After deployment, verify:

1. **Health Check**:
   ```bash
   curl https://mcp.example.com/health
   ```

2. **API Access**:
   ```bash
   curl -H "Authorization: ApiKey your-api-key" https://mcp.example.com/api/v1/github/repos
   ```

3. **Webhook Reception**:
   - Configure a test webhook in GitHub
   - Trigger an event
   - Verify proper processing

4. **Metrics Collection**:
   - Check Prometheus metrics
   - Verify Grafana dashboards
   - Test alerting

5. **Log Collection**:
   - Verify logs are being collected
   - Check log format and content
   - Ensure sensitive data is not logged