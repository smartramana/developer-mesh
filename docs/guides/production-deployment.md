# Production Deployment Guide

> **Purpose**: Complete guide for deploying DevOps MCP platform to AWS production environment
> **Audience**: DevOps engineers and system administrators deploying to production
> **Scope**: AWS infrastructure, deployment process, monitoring, scaling, and maintenance

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Architecture Overview](#architecture-overview)
3. [AWS Infrastructure Setup](#aws-infrastructure-setup)
4. [Service Deployment](#service-deployment)
5. [Configuration Management](#configuration-management)
6. [Security Hardening](#security-hardening)
7. [Monitoring and Alerts](#monitoring-and-alerts)
8. [Scaling Strategy](#scaling-strategy)
9. [Disaster Recovery](#disaster-recovery)
10. [Maintenance Procedures](#maintenance-procedures)

## Prerequisites

### Required Tools

```bash
# AWS CLI v2
aws --version  # Should be 2.x or higher

# Terraform (for infrastructure as code)
terraform --version  # 1.5+ recommended

# Docker and Docker Compose
docker --version     # 24.0+
docker-compose --version  # 2.20+

# kubectl (for EKS deployment)
kubectl version --client  # 1.28+

# Go (for building from source)
go version  # 1.24.3 required
```

### AWS Account Setup

```bash
# Configure AWS credentials
aws configure --profile production
# AWS Access Key ID: [your-key]
# AWS Secret Access Key: [your-secret]
# Default region: us-east-1
# Default output format: json

# Verify access
aws sts get-caller-identity --profile production
```

### Required AWS Services

- **EC2**: Application servers
- **RDS PostgreSQL**: Primary database with pgvector
- **ElastiCache Redis**: Caching and session storage
- **S3**: Object storage for contexts
- **SQS**: Message queuing
- **Bedrock**: AI model access
- **ALB**: Load balancing
- **Route 53**: DNS management
- **CloudWatch**: Monitoring
- **VPC**: Network isolation

## Architecture Overview

### Production Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Route 53                                 │
│                    (mcp.yourdomain.com)                         │
└────────────────────────┬────────────────────────────────────────┘
                         │
┌────────────────────────┴────────────────────────────────────────┐
│                    CloudFront CDN                                │
│                  (Global Distribution)                           │
└────────────────────────┬────────────────────────────────────────┘
                         │
┌────────────────────────┴────────────────────────────────────────┐
│                Application Load Balancer                         │
│                    (Multi-AZ, SSL)                              │
└────────────────────────┬────────────────────────────────────────┘
                         │
        ┌────────────────┴────────────────┐
        │                                 │
┌───────┴──────────┐            ┌────────┴──────────┐
│   Auto Scaling   │            │   Auto Scaling    │
│   Group (AZ-1)   │            │   Group (AZ-2)    │
├──────────────────┤            ├───────────────────┤
│ MCP Server       │            │ MCP Server        │
│ REST API         │            │ REST API          │
│ Worker           │            │ Worker            │
└──────┬───────────┘            └────────┬──────────┘
       │                                  │
       └────────────┬─────────────────────┘
                    │
    ┌───────────────┴─────────────────────────┐
    │                                         │
┌───┴──────────┐  ┌──────────────┐  ┌───────┴──────┐
│RDS PostgreSQL│  │ ElastiCache  │  │     S3       │
│  (Multi-AZ)  │  │Redis Cluster │  │   Buckets    │
│  + pgvector  │  │  (Multi-AZ)  │  │              │
└──────────────┘  └──────────────┘  └──────────────┘
```

### Network Architecture

```yaml
VPC:
  CIDR: 10.0.0.0/16
  
  Public Subnets:
    - 10.0.1.0/24 (AZ-1) # ALB, NAT Gateway
    - 10.0.2.0/24 (AZ-2) # ALB, NAT Gateway
    
  Private Subnets:
    - 10.0.10.0/24 (AZ-1) # Application servers
    - 10.0.20.0/24 (AZ-2) # Application servers
    
  Database Subnets:
    - 10.0.100.0/24 (AZ-1) # RDS, ElastiCache
    - 10.0.200.0/24 (AZ-2) # RDS, ElastiCache
```

## AWS Infrastructure Setup

### 1. VPC and Networking

```bash
# Create VPC using Terraform
cd infrastructure/terraform/production

# Initialize Terraform
terraform init

# Plan infrastructure
terraform plan -var-file=production.tfvars

# Apply infrastructure
terraform apply -var-file=production.tfvars
```

**Terraform VPC Module** (`infrastructure/terraform/modules/vpc/main.tf`):

```hcl
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"

  name = "mcp-production-vpc"
  cidr = "10.0.0.0/16"

  azs             = ["us-east-1a", "us-east-1b"]
  private_subnets = ["10.0.10.0/24", "10.0.20.0/24"]
  public_subnets  = ["10.0.1.0/24", "10.0.2.0/24"]
  database_subnets = ["10.0.100.0/24", "10.0.200.0/24"]

  enable_nat_gateway = true
  enable_vpn_gateway = false
  enable_dns_hostnames = true
  enable_dns_support = true

  tags = {
    Environment = "production"
    Project = "devops-mcp"
  }
}
```

### 2. RDS PostgreSQL Setup

```hcl
resource "aws_db_instance" "mcp_postgres" {
  identifier = "mcp-production-db"
  
  engine         = "postgres"
  engine_version = "15.4"
  instance_class = "db.r6g.xlarge"  # 4 vCPU, 32 GB RAM
  
  allocated_storage     = 100
  storage_type         = "gp3"
  storage_encrypted    = true
  kms_key_id          = aws_kms_key.rds.arn
  
  db_name  = "mcp"
  username = "mcp_admin"
  password = random_password.db_password.result
  
  vpc_security_group_ids = [aws_security_group.rds.id]
  db_subnet_group_name   = aws_db_subnet_group.main.name
  
  backup_retention_period = 30
  backup_window          = "03:00-04:00"
  maintenance_window     = "sun:04:00-sun:05:00"
  
  multi_az               = true
  deletion_protection    = true
  skip_final_snapshot    = false
  
  enabled_cloudwatch_logs_exports = ["postgresql"]
  
  tags = {
    Name = "mcp-production-db"
    Environment = "production"
  }
}

# Enable pgvector extension
resource "null_resource" "enable_pgvector" {
  depends_on = [aws_db_instance.mcp_postgres]
  
  provisioner "local-exec" {
    command = <<-EOT
      PGPASSWORD=${random_password.db_password.result} psql \
        -h ${aws_db_instance.mcp_postgres.endpoint} \
        -U mcp_admin \
        -d mcp \
        -c "CREATE EXTENSION IF NOT EXISTS vector;"
    EOT
  }
}
```

### 3. ElastiCache Redis Setup

```hcl
resource "aws_elasticache_replication_group" "mcp_redis" {
  replication_group_id       = "mcp-production-redis"
  replication_group_description = "MCP Production Redis Cluster"
  
  engine               = "redis"
  engine_version       = "7.1"
  node_type           = "cache.r7g.large"
  number_cache_clusters = 2  # Multi-AZ
  
  parameter_group_name = aws_elasticache_parameter_group.redis.name
  subnet_group_name    = aws_elasticache_subnet_group.main.name
  security_group_ids   = [aws_security_group.redis.id]
  
  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token_enabled        = true
  auth_token                = random_password.redis_auth_token.result
  
  automatic_failover_enabled = true
  multi_az_enabled          = true
  
  snapshot_retention_limit = 7
  snapshot_window         = "03:00-05:00"
  
  tags = {
    Name = "mcp-production-redis"
    Environment = "production"
  }
}
```

### 4. S3 Buckets

```hcl
resource "aws_s3_bucket" "contexts" {
  bucket = "mcp-production-contexts"
  
  tags = {
    Name = "MCP Context Storage"
    Environment = "production"
  }
}

resource "aws_s3_bucket_versioning" "contexts" {
  bucket = aws_s3_bucket.contexts.id
  
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "contexts" {
  bucket = aws_s3_bucket.contexts.id
  
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "contexts" {
  bucket = aws_s3_bucket.contexts.id
  
  rule {
    id     = "archive-old-contexts"
    status = "Enabled"
    
    transition {
      days          = 30
      storage_class = "STANDARD_IA"
    }
    
    transition {
      days          = 90
      storage_class = "GLACIER"
    }
  }
}
```

### 5. SQS Queues

```hcl
resource "aws_sqs_queue" "task_queue" {
  name = "mcp-production-tasks"
  
  delay_seconds             = 0
  max_message_size         = 262144  # 256 KB
  message_retention_seconds = 1209600 # 14 days
  receive_wait_time_seconds = 20      # Long polling
  
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq.arn
    maxReceiveCount     = 3
  })
  
  kms_master_key_id = "alias/aws/sqs"
  
  tags = {
    Name = "mcp-production-task-queue"
    Environment = "production"
  }
}

resource "aws_sqs_queue" "dlq" {
  name = "mcp-production-tasks-dlq"
  
  message_retention_seconds = 1209600 # 14 days
  
  tags = {
    Name = "mcp-production-dlq"
    Environment = "production"
  }
}
```

## Service Deployment

### 1. Container Registry Setup

```bash
# Create ECR repositories
aws ecr create-repository --repository-name mcp/server --region us-east-1
aws ecr create-repository --repository-name mcp/api --region us-east-1
aws ecr create-repository --repository-name mcp/worker --region us-east-1

# Get login token
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin $ECR_REGISTRY

# Build and push images
make docker-build-all
make docker-push-all ECR_REGISTRY=$ECR_REGISTRY
```

### 2. ECS Task Definitions

**MCP Server Task** (`infrastructure/ecs/task-definitions/mcp-server.json`):

```json
{
  "family": "mcp-server",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "2048",
  "memory": "4096",
  "containerDefinitions": [
    {
      "name": "mcp-server",
      "image": "${ECR_REGISTRY}/mcp/server:latest",
      "essential": true,
      "portMappings": [
        {
          "containerPort": 8080,
          "protocol": "tcp"
        }
      ],
      "environment": [
        {"name": "ENV", "value": "production"},
        {"name": "LOG_LEVEL", "value": "info"},
        {"name": "METRICS_ENABLED", "value": "true"}
      ],
      "secrets": [
        {
          "name": "DATABASE_URL",
          "valueFrom": "arn:aws:secretsmanager:us-east-1:123456789012:secret:mcp/production/db-url"
        },
        {
          "name": "REDIS_URL",
          "valueFrom": "arn:aws:secretsmanager:us-east-1:123456789012:secret:mcp/production/redis-url"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/mcp-server",
          "awslogs-region": "us-east-1",
          "awslogs-stream-prefix": "ecs"
        }
      },
      "healthCheck": {
        "command": ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"],
        "interval": 30,
        "timeout": 5,
        "retries": 3,
        "startPeriod": 60
      }
    }
  ]
}
```

### 3. ECS Service Deployment

```bash
# Create ECS cluster
aws ecs create-cluster --cluster-name mcp-production

# Register task definitions
aws ecs register-task-definition --cli-input-json file://infrastructure/ecs/task-definitions/mcp-server.json
aws ecs register-task-definition --cli-input-json file://infrastructure/ecs/task-definitions/rest-api.json
aws ecs register-task-definition --cli-input-json file://infrastructure/ecs/task-definitions/worker.json

# Create services
aws ecs create-service \
  --cluster mcp-production \
  --service-name mcp-server \
  --task-definition mcp-server:1 \
  --desired-count 3 \
  --launch-type FARGATE \
  --platform-version LATEST \
  --network-configuration "awsvpcConfiguration={subnets=[subnet-xxx,subnet-yyy],securityGroups=[sg-xxx]}" \
  --load-balancers "targetGroupArn=arn:aws:elasticloadbalancing:...,containerName=mcp-server,containerPort=8080"
```

### 4. Auto Scaling Configuration

```hcl
resource "aws_appautoscaling_target" "mcp_server" {
  max_capacity       = 10
  min_capacity       = 2
  resource_id        = "service/mcp-production/mcp-server"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "cpu_scaling" {
  name               = "mcp-server-cpu-scaling"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.mcp_server.resource_id
  scalable_dimension = aws_appautoscaling_target.mcp_server.scalable_dimension
  service_namespace  = aws_appautoscaling_target.mcp_server.service_namespace
  
  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    
    target_value = 70.0
    scale_in_cooldown  = 300
    scale_out_cooldown = 60
  }
}
```

## Configuration Management

### 1. Environment Variables

```bash
# Production environment file (.env.production)
ENV=production
LOG_LEVEL=info
LOG_FORMAT=json

# Service URLs
MCP_SERVER_URL=wss://mcp.yourdomain.com/ws
REST_API_URL=https://api.yourdomain.com

# AWS Services
AWS_REGION=us-east-1
S3_BUCKET=mcp-production-contexts
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123456789012/mcp-production-tasks

# Bedrock Configuration
BEDROCK_ENABLED=true
BEDROCK_SESSION_LIMIT=1.00
GLOBAL_COST_LIMIT=100.0

# Performance Settings
MAX_CONNECTIONS=1000
CONNECTION_TIMEOUT=30s
REQUEST_TIMEOUT=300s
WORKER_CONCURRENCY=10

# Security
JWT_SECRET_NAME=mcp/production/jwt-secret
API_KEY_SECRET_NAME=mcp/production/api-keys
```

### 2. Secrets Management

```bash
# Store secrets in AWS Secrets Manager
aws secretsmanager create-secret \
  --name mcp/production/db-url \
  --secret-string "postgresql://user:pass@rds-endpoint:5432/mcp?sslmode=require"

aws secretsmanager create-secret \
  --name mcp/production/redis-url \
  --secret-string "rediss://default:auth-token@redis-endpoint:6379"

aws secretsmanager create-secret \
  --name mcp/production/jwt-secret \
  --secret-string "$(openssl rand -hex 32)"

# Rotate secrets
aws secretsmanager rotate-secret \
  --secret-id mcp/production/db-password \
  --rotation-lambda-arn arn:aws:lambda:us-east-1:123456789012:function:SecretsManagerRotation
```

### 3. Parameter Store

```bash
# Store configuration in Parameter Store
aws ssm put-parameter \
  --name /mcp/production/config/max-connections \
  --value "1000" \
  --type String

aws ssm put-parameter \
  --name /mcp/production/config/bedrock-models \
  --value '["amazon.titan-embed-text-v1","cohere.embed-english-v3"]' \
  --type StringList
```

## Security Hardening

### 1. Network Security

```hcl
# ALB Security Group
resource "aws_security_group" "alb" {
  name_prefix = "mcp-alb-"
  vpc_id      = module.vpc.vpc_id
  
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  
  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# Application Security Group
resource "aws_security_group" "app" {
  name_prefix = "mcp-app-"
  vpc_id      = module.vpc.vpc_id
  
  ingress {
    from_port       = 8080
    to_port         = 8081
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }
  
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
```

### 2. IAM Roles and Policies

```hcl
# ECS Task Role
resource "aws_iam_role" "ecs_task_role" {
  name = "mcp-production-ecs-task-role"
  
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
      }
    ]
  })
}

# Task Role Policy
resource "aws_iam_role_policy" "ecs_task_policy" {
  name = "mcp-production-ecs-task-policy"
  role = aws_iam_role.ecs_task_role.id
  
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject"
        ]
        Resource = "${aws_s3_bucket.contexts.arn}/*"
      },
      {
        Effect = "Allow"
        Action = [
          "sqs:SendMessage",
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage"
        ]
        Resource = aws_sqs_queue.task_queue.arn
      },
      {
        Effect = "Allow"
        Action = [
          "bedrock:InvokeModel"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue"
        ]
        Resource = "arn:aws:secretsmanager:us-east-1:*:secret:mcp/production/*"
      }
    ]
  })
}
```

### 3. SSL/TLS Configuration

```bash
# Request SSL certificate
aws acm request-certificate \
  --domain-name mcp.yourdomain.com \
  --subject-alternative-names "*.mcp.yourdomain.com" \
  --validation-method DNS

# Configure ALB listener
aws elbv2 create-listener \
  --load-balancer-arn $ALB_ARN \
  --protocol HTTPS \
  --port 443 \
  --certificates CertificateArn=$CERT_ARN \
  --ssl-policy ELBSecurityPolicy-TLS13-1-2-2021-06 \
  --default-actions Type=forward,TargetGroupArn=$TARGET_GROUP_ARN
```

### 4. WAF Configuration

```hcl
resource "aws_wafv2_web_acl" "main" {
  name  = "mcp-production-waf"
  scope = "REGIONAL"
  
  default_action {
    allow {}
  }
  
  rule {
    name     = "RateLimitRule"
    priority = 1
    
    action {
      block {}
    }
    
    statement {
      rate_based_statement {
        limit              = 2000
        aggregate_key_type = "IP"
      }
    }
    
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name               = "RateLimitRule"
      sampled_requests_enabled   = true
    }
  }
  
  rule {
    name     = "CommonRuleSet"
    priority = 2
    
    override_action {
      none {}
    }
    
    statement {
      managed_rule_group_statement {
        vendor_name = "AWS"
        name        = "AWSManagedRulesCommonRuleSet"
      }
    }
    
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name               = "CommonRuleSet"
      sampled_requests_enabled   = true
    }
  }
}
```

## Monitoring and Alerts

### 1. CloudWatch Dashboards

```json
{
  "widgets": [
    {
      "type": "metric",
      "properties": {
        "metrics": [
          ["AWS/ECS", "CPUUtilization", "ServiceName", "mcp-server"],
          [".", "MemoryUtilization", ".", "."]
        ],
        "period": 300,
        "stat": "Average",
        "region": "us-east-1",
        "title": "ECS Service Metrics"
      }
    },
    {
      "type": "metric",
      "properties": {
        "metrics": [
          ["AWS/ApplicationELB", "TargetResponseTime", "LoadBalancer", "mcp-alb"],
          [".", "RequestCount", ".", "."],
          [".", "HTTPCode_Target_4XX_Count", ".", "."],
          [".", "HTTPCode_Target_5XX_Count", ".", "."]
        ],
        "period": 300,
        "stat": "Sum",
        "region": "us-east-1",
        "title": "ALB Metrics"
      }
    }
  ]
}
```

### 2. CloudWatch Alarms

```hcl
resource "aws_cloudwatch_metric_alarm" "high_cpu" {
  alarm_name          = "mcp-production-high-cpu"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name        = "CPUUtilization"
  namespace          = "AWS/ECS"
  period             = 300
  statistic          = "Average"
  threshold          = 80
  alarm_description  = "Triggers when CPU utilization is above 80%"
  
  dimensions = {
    ServiceName = "mcp-server"
    ClusterName = "mcp-production"
  }
  
  alarm_actions = [aws_sns_topic.alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "high_error_rate" {
  alarm_name          = "mcp-production-high-error-rate"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  threshold          = 10
  
  metric_query {
    id          = "error_rate"
    expression  = "(m1/m2)*100"
    label       = "Error Rate"
    return_data = true
  }
  
  metric_query {
    id = "m1"
    metric {
      metric_name = "HTTPCode_Target_5XX_Count"
      namespace   = "AWS/ApplicationELB"
      period      = 300
      stat        = "Sum"
      dimensions = {
        LoadBalancer = aws_lb.main.arn_suffix
      }
    }
  }
  
  metric_query {
    id = "m2"
    metric {
      metric_name = "RequestCount"
      namespace   = "AWS/ApplicationELB"
      period      = 300
      stat        = "Sum"
      dimensions = {
        LoadBalancer = aws_lb.main.arn_suffix
      }
    }
  }
}
```

### 3. Application Metrics

```go
// Custom CloudWatch metrics
func initMetrics() {
    // Create CloudWatch client
    sess := session.Must(session.NewSession())
    svc := cloudwatch.New(sess)
    
    // Task processing metrics
    go func() {
        ticker := time.NewTicker(60 * time.Second)
        for range ticker.C {
            // Send custom metrics
            svc.PutMetricData(&cloudwatch.PutMetricDataInput{
                Namespace: aws.String("MCP/Production"),
                MetricData: []*cloudwatch.MetricDatum{
                    {
                        MetricName: aws.String("TasksProcessed"),
                        Value:      aws.Float64(float64(tasksProcessed)),
                        Unit:       aws.String("Count"),
                        Dimensions: []*cloudwatch.Dimension{
                            {
                                Name:  aws.String("Environment"),
                                Value: aws.String("production"),
                            },
                        },
                    },
                    {
                        MetricName: aws.String("ActiveAgents"),
                        Value:      aws.Float64(float64(activeAgents)),
                        Unit:       aws.String("Count"),
                    },
                },
            })
        }
    }()
}
```

### 4. Log Aggregation

```bash
# Configure CloudWatch Logs Insights queries

# Error analysis query
fields @timestamp, @message
| filter @message like /ERROR/
| stats count() by bin(5m)

# Request latency analysis
fields @timestamp, duration
| filter @type = "REQUEST"
| stats avg(duration), max(duration), min(duration) by bin(5m)

# Agent activity monitoring
fields @timestamp, agent_id, action
| filter @type = "AGENT_EVENT"
| stats count() by agent_id, action
```

## Scaling Strategy

### 1. Horizontal Scaling

```yaml
# Kubernetes HPA configuration
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: mcp-server-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: mcp-server
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  - type: Pods
    pods:
      metric:
        name: websocket_connections_per_pod
      target:
        type: AverageValue
        averageValue: "100"
```

### 2. Database Scaling

```bash
# Read replica configuration
aws rds create-db-instance-read-replica \
  --db-instance-identifier mcp-production-read-1 \
  --source-db-instance-identifier mcp-production-db \
  --availability-zone us-east-1b

# Connection pooling with PgBouncer
[databases]
mcp = host=rds-endpoint.amazonaws.com port=5432 dbname=mcp

[pgbouncer]
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 25
min_pool_size = 10
reserve_pool_size = 5
```

### 3. Cache Scaling

```bash
# Redis cluster scaling
aws elasticache modify-replication-group \
  --replication-group-id mcp-production-redis \
  --cache-node-type cache.r7g.xlarge \
  --apply-immediately

# Add read replicas
aws elasticache increase-replica-count \
  --replication-group-id mcp-production-redis \
  --new-replica-count 3 \
  --apply-immediately
```

## Disaster Recovery

### 1. Backup Strategy

```bash
# Automated RDS backups
aws rds modify-db-instance \
  --db-instance-identifier mcp-production-db \
  --backup-retention-period 30 \
  --preferred-backup-window "03:00-04:00"

# S3 cross-region replication
aws s3api put-bucket-replication \
  --bucket mcp-production-contexts \
  --replication-configuration file://s3-replication.json

# ElastiCache snapshots
aws elasticache create-snapshot \
  --replication-group-id mcp-production-redis \
  --snapshot-name mcp-redis-$(date +%Y%m%d-%H%M%S)
```

### 2. Recovery Procedures

```bash
# RDS recovery from snapshot
aws rds restore-db-instance-from-db-snapshot \
  --db-instance-identifier mcp-production-db-restored \
  --db-snapshot-identifier mcp-production-snapshot-20241225

# Redis recovery
aws elasticache create-replication-group \
  --replication-group-id mcp-production-redis-restored \
  --replication-group-description "Restored Redis cluster" \
  --snapshot-name mcp-redis-20241225-120000

# Application failover
kubectl set image deployment/mcp-server \
  mcp-server=$ECR_REGISTRY/mcp/server:rollback-version \
  --record
```

### 3. Multi-Region Setup

```hcl
# Secondary region resources
module "dr_region" {
  source = "./modules/dr-region"
  
  providers = {
    aws = aws.us_west_2
  }
  
  primary_region = "us-east-1"
  enable_replication = true
  
  rds_snapshot_identifier = aws_db_snapshot_copy.dr_snapshot.id
  s3_replica_bucket = "mcp-dr-contexts"
}
```

## Maintenance Procedures

### 1. Rolling Updates

```bash
# ECS rolling update
aws ecs update-service \
  --cluster mcp-production \
  --service mcp-server \
  --task-definition mcp-server:new-version \
  --desired-count 3 \
  --deployment-configuration "maximumPercent=200,minimumHealthyPercent=100"

# Monitor deployment
aws ecs describe-services \
  --cluster mcp-production \
  --services mcp-server \
  --query 'services[0].deployments'
```

### 2. Database Maintenance

```sql
-- Vacuum and analyze tables
VACUUM ANALYZE agents;
VACUUM ANALYZE tasks;
VACUUM ANALYZE contexts;

-- Update statistics
ANALYZE;

-- Check for bloat
SELECT 
  schemaname,
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename) - pg_relation_size(schemaname||'.'||tablename)) AS external_size
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

### 3. Security Updates

```bash
# Update base images
docker pull public.ecr.aws/lambda/go:1.24.3
docker build --no-cache -t mcp/server:latest .

# Rotate secrets
aws secretsmanager rotate-secret \
  --secret-id mcp/production/jwt-secret \
  --rotation-lambda-arn $ROTATION_LAMBDA_ARN

# Update security groups
aws ec2 revoke-security-group-ingress \
  --group-id sg-xxx \
  --ip-permissions file://old-rules.json

aws ec2 authorize-security-group-ingress \
  --group-id sg-xxx \
  --ip-permissions file://new-rules.json
```

### 4. Performance Tuning

```bash
# RDS parameter group optimization
aws rds modify-db-parameter-group \
  --db-parameter-group-name mcp-postgres15 \
  --parameters "ParameterName=shared_buffers,ParameterValue=8GB,ApplyMethod=pending-reboot" \
               "ParameterName=effective_cache_size,ParameterValue=24GB,ApplyMethod=immediate" \
               "ParameterName=work_mem,ParameterValue=64MB,ApplyMethod=immediate"

# Redis optimization
CONFIG SET maxmemory-policy allkeys-lru
CONFIG SET timeout 300
CONFIG SET tcp-keepalive 60
```

## Troubleshooting

### Common Issues

1. **High Database CPU**
   ```sql
   -- Find slow queries
   SELECT query, mean_exec_time, calls
   FROM pg_stat_statements
   ORDER BY mean_exec_time DESC
   LIMIT 10;
   ```

2. **WebSocket Connection Drops**
   ```bash
   # Check ALB timeout settings
   aws elbv2 modify-target-group-attributes \
     --target-group-arn $TG_ARN \
     --attributes Key=deregistration_delay.timeout_seconds,Value=30
   ```

3. **Memory Leaks**
   ```bash
   # Enable memory profiling
   kubectl exec -it mcp-server-xxx -- /bin/sh
   curl http://localhost:6060/debug/pprof/heap > heap.prof
   go tool pprof heap.prof
   ```

## Cost Optimization

### 1. Reserved Instances

```bash
# Purchase RDS reserved instances
aws rds purchase-reserved-db-instances-offering \
  --reserved-db-instances-offering-id offering-id \
  --reserved-db-instance-id mcp-production-ri \
  --instance-count 1

# ECS Fargate Savings Plans
# Configure through AWS Console for commitment discounts
```

### 2. Spot Instances for Workers

```hcl
resource "aws_ecs_capacity_provider" "spot" {
  name = "mcp-spot-capacity"
  
  auto_scaling_group_provider {
    auto_scaling_group_arn = aws_autoscaling_group.spot.arn
    
    managed_scaling {
      status                    = "ENABLED"
      target_capacity           = 100
      minimum_scaling_step_size = 1
      maximum_scaling_step_size = 10
    }
  }
}
```

## Next Steps

1. Review [Monitoring Guide](../monitoring/MONITORING.md) for detailed observability setup
2. Check [Security Best Practices](./security-best-practices.md) for additional hardening
3. See [Performance Tuning Guide](./performance-tuning-guide.md) for optimization
4. Explore [Cost Management Guide](./cost-management.md) for budget controls

## Resources

- [AWS Well-Architected Framework](https://aws.amazon.com/architecture/well-architected/)
- [ECS Best Practices Guide](https://docs.aws.amazon.com/AmazonECS/latest/bestpracticesguide/)
- [RDS Best Practices](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_BestPractices.html)
- [ElastiCache Best Practices](https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/BestPractices.html)