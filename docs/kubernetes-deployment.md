# Kubernetes Deployment Guide

This guide provides detailed instructions for deploying the MCP Server on Kubernetes, with a focus on Amazon EKS.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Kubernetes Resources](#kubernetes-resources)
- [IAM Roles for Service Accounts (IRSA)](#iam-roles-for-service-accounts-irsa)
- [Deployment Steps](#deployment-steps)
- [Configuration](#configuration)
- [Scaling](#scaling)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)

## Prerequisites

Before deploying the MCP Server on Kubernetes, ensure you have:

- A Kubernetes cluster (e.g., Amazon EKS, GKE, AKS, or a self-managed cluster)
- `kubectl` configured to access your cluster
- For AWS deployments:
  - AWS CLI installed and configured
  - IAM permissions to create and manage IAM roles
  - `eksctl` installed (for AWS EKS)

## Kubernetes Resources

The MCP Server repository includes the following Kubernetes manifests in the `kubernetes/` directory:

- `namespace.yaml` - Creates the `mcp-server` namespace
- `serviceaccount.yaml` - Creates a service account with IRSA annotations
- `configmap.yaml` - Contains configuration settings
- `secret.yaml` - Contains sensitive information (e.g., API keys)
- `deployment.yaml` - Deploys the MCP Server application
- `service.yaml` - Exposes the application
- `ingress.yaml` - Configures ingress (optional)
- `hpa.yaml` - Horizontal Pod Autoscaler for scaling (optional)

## IAM Roles for Service Accounts (IRSA)

For AWS deployments, the MCP Server uses IAM Roles for Service Accounts (IRSA) to securely access AWS services without the need for hardcoded credentials.

### Setting Up IRSA on EKS

1. Ensure your EKS cluster has OIDC provider enabled:

```bash
eksctl utils associate-iam-oidc-provider --cluster your-cluster-name --approve
```

2. Create an IAM role for the MCP Server:

```bash
# Create a role with permissions for S3, RDS, and ElastiCache
eksctl create iamserviceaccount \
  --name mcp-server \
  --namespace mcp-server \
  --cluster your-cluster-name \
  --attach-policy-arn arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess \
  --attach-policy-arn arn:aws:iam::aws:policy/AmazonRDSDataFullAccess \
  --approve
```

3. For more fine-grained control, use the policy templates provided in the `docs/aws/` directory.

For detailed instructions, see the [AWS IRSA Setup Guide](aws/aws-irsa-setup.md).

## Deployment Steps

### 1. Create the Namespace

```bash
kubectl apply -f kubernetes/namespace.yaml
```

### 2. Create Config and Secrets

```bash
# Create a secret for sensitive information
kubectl create secret generic mcp-server-secrets \
  --namespace mcp-server \
  --from-literal=DB_PASSWORD='your-password' \
  --from-literal=JWT_SECRET='your-jwt-secret' \
  --from-literal=API_KEY='your-api-key'

# Apply the configmap
kubectl apply -f kubernetes/configmap.yaml
```

### 3. Deploy the Application

```bash
# Create the service account
kubectl apply -f kubernetes/serviceaccount.yaml

# Deploy the application
kubectl apply -f kubernetes/deployment.yaml

# Create the service
kubectl apply -f kubernetes/service.yaml

# (Optional) Apply ingress configuration
kubectl apply -f kubernetes/ingress.yaml
```

### 4. Verify the Deployment

```bash
# Check if pods are running
kubectl get pods -n mcp-server

# Check the service
kubectl get svc -n mcp-server

# Check logs
kubectl logs -n mcp-server deployment/mcp-server
```

## Configuration

The MCP Server can be configured via environment variables in the deployment manifest:

```yaml
env:
  - name: MCP_ENVIRONMENT
    value: "production"
  - name: MCP_API_LISTEN_ADDRESS
    value: ":8080"
  - name: MCP_DATABASE_HOST
    value: "your-rds-instance.region.rds.amazonaws.com"
  # Other environment variables...
```

For AWS service integration with IRSA:

```yaml
env:
  - name: MCP_AWS_RDS_USE_IAM_AUTH
    value: "true"
  - name: MCP_AWS_S3_USE_IAM_AUTH
    value: "true"
  - name: MCP_AWS_ELASTICACHE_USE_IAM_AUTH
    value: "true"
  # Other AWS-related configuration...
```

## Scaling

### Horizontal Pod Autoscaler

You can use Horizontal Pod Autoscaler (HPA) to automatically scale the number of pods based on CPU or memory usage:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: mcp-server
  namespace: mcp-server
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: mcp-server
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 75
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

Apply with:

```bash
kubectl apply -f kubernetes/hpa.yaml
```

### Resource Requests and Limits

Configure appropriate resource requests and limits in the deployment manifest:

```yaml
resources:
  requests:
    cpu: "100m"
    memory: "256Mi"
  limits:
    cpu: "1000m"
    memory: "1Gi"
```

## Monitoring

### Prometheus Integration

The MCP Server exposes metrics at the `/metrics` endpoint. To collect these metrics with Prometheus:

1. Install Prometheus and Grafana in your cluster (e.g., using the Prometheus Operator or Helm charts)
2. Create a ServiceMonitor or PodMonitor (if using the Prometheus Operator):

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: mcp-server
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: mcp-server
  namespaceSelector:
    matchNames:
    - mcp-server
  endpoints:
  - port: http
    path: /metrics
    interval: 15s
```

3. Import the provided Grafana dashboards (available in the `monitoring/grafana/` directory)

## Troubleshooting

### Common Issues

1. **Pods failing to start** - Check logs with `kubectl logs -n mcp-server <pod-name>`
2. **IRSA not working** - Verify that the service account annotations are correct and the OIDC provider is properly configured
3. **Connection issues to AWS services** - Check network policies and security groups
4. **Database connection failures** - Verify database credentials and network access
5. **High resource usage** - Adjust resource requests and limits or scale horizontally

### Debugging

```bash
# Get detailed pod information
kubectl describe pod -n mcp-server <pod-name>

# Check events
kubectl get events -n mcp-server

# Exec into a pod for debugging
kubectl exec -it -n mcp-server <pod-name> -- /bin/sh

# Check connectivity to services
kubectl exec -it -n mcp-server <pod-name> -- curl -v <service-url>
```

For more troubleshooting information, see the [Troubleshooting Guide](troubleshooting-guide.md).
