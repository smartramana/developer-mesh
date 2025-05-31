# Monitoring and Observability Guide

## Overview
This guide covers the complete observability stack for DevOps MCP, including metrics, logging, tracing, and alerting.

## Quick Start

### 1. Deploy Monitoring Stack
```bash
# Deploy Prometheus, Grafana, and Jaeger
kubectl apply -f kubernetes/monitoring/

# Or use Docker Compose
docker-compose -f docker-compose.monitoring.yml up -d
```

### 2. Access Dashboards
- Grafana: http://localhost:3000 (admin/admin)
- Prometheus: http://localhost:9090
- Jaeger: http://localhost:16686

## Metrics Collection

### Available Metrics

#### HTTP Metrics
- `http_requests_total` - Total HTTP requests by method, path, status
- `http_request_duration_seconds` - Request latency histogram
- `http_requests_in_flight` - Current in-flight requests

#### Business Metrics
- `mcp_contexts_created_total` - Total contexts created
- `mcp_tool_executions_total` - Tool execution count by tool name
- `mcp_auth_attempts_total` - Authentication attempts by method
- `mcp_api_keys_active` - Number of active API keys

#### System Metrics
- `go_goroutines` - Number of goroutines
- `go_memstats_alloc_bytes` - Memory allocation
- `process_cpu_seconds_total` - CPU usage

### Prometheus Configuration
```yaml
# prometheus/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'mcp-server'
    static_configs:
      - targets: ['mcp-server:8080']
    relabel_configs:
      - source_labels: [__address__]
        target_label: instance
        regex: '([^:]+)(?::\d+)?'
        replacement: '${1}'

  - job_name: 'rest-api'
    static_configs:
      - targets: ['rest-api:8080']

  - job_name: 'worker'
    static_configs:
      - targets: ['worker:8080']
```

### Custom Metrics Implementation
```go
// Example: Adding custom metrics
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    contextsCreated = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_contexts_created_total",
            Help: "Total number of contexts created",
        },
        []string{"tenant_id", "type"},
    )
    
    contextSize = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name: "mcp_context_size_bytes",
            Help: "Size of contexts in bytes",
            Buckets: prometheus.ExponentialBuckets(1024, 2, 10), // 1KB to 1MB
        },
    )
)

// Use in your code
func CreateContext(ctx context.Context, content string) error {
    contextsCreated.WithLabelValues(tenantID, "manual").Inc()
    contextSize.Observe(float64(len(content)))
    // ... rest of implementation
}
```

## Logging Configuration

### Structured Logging
```yaml
# config.yaml
monitoring:
  logging:
    level: info
    format: json
    output: stdout
    fields:
      - service
      - version
      - environment
      - trace_id
```

### Log Aggregation with Loki
```yaml
# loki/config.yaml
auth_enabled: false

server:
  http_listen_port: 3100

ingester:
  lifecycler:
    address: 127.0.0.1
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1

schema_config:
  configs:
    - from: 2020-10-24
      store: boltdb-shipper
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 24h
```

### Promtail Configuration
```yaml
# promtail/config.yaml
clients:
  - url: http://loki:3100/loki/api/v1/push

positions:
  filename: /tmp/positions.yaml

scrape_configs:
  - job_name: mcp-services
    static_configs:
      - targets:
          - localhost
        labels:
          job: mcp-services
          __path__: /var/log/mcp/*.log
    pipeline_stages:
      - json:
          expressions:
            level: level
            service: service
            trace_id: trace_id
      - labels:
          level:
          service:
```

## Distributed Tracing

### Jaeger Integration
```go
// Initialize tracing
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracing() (*trace.TracerProvider, error) {
    exporter, err := jaeger.New(
        jaeger.WithCollectorEndpoint(
            jaeger.WithEndpoint("http://jaeger:14268/api/traces"),
        ),
    )
    if err != nil {
        return nil, err
    }

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String("mcp-server"),
            semconv.ServiceVersionKey.String("1.0.0"),
        )),
    )
    
    otel.SetTracerProvider(tp)
    return tp, nil
}

// Use in handlers
func HandleRequest(ctx context.Context) {
    tracer := otel.Tracer("mcp-server")
    ctx, span := tracer.Start(ctx, "HandleRequest")
    defer span.End()
    
    // Add attributes
    span.SetAttributes(
        attribute.String("tenant.id", tenantID),
        attribute.Int("request.size", len(body)),
    )
}
```

## Grafana Dashboards

### MCP Overview Dashboard
```json
{
  "dashboard": {
    "title": "MCP Overview",
    "panels": [
      {
        "title": "Request Rate",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
        "targets": [{
          "expr": "sum(rate(http_requests_total[5m])) by (service)"
        }]
      },
      {
        "title": "Error Rate",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
        "targets": [{
          "expr": "sum(rate(http_requests_total{status=~\"5..\"}[5m])) by (service) / sum(rate(http_requests_total[5m])) by (service)"
        }]
      },
      {
        "title": "P95 Latency",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8},
        "targets": [{
          "expr": "histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (service, le))"
        }]
      },
      {
        "title": "Active Contexts",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8},
        "targets": [{
          "expr": "sum(mcp_contexts_active) by (tenant_id)"
        }]
      }
    ]
  }
}
```

### Import Dashboard
```bash
# Import via API
curl -X POST http://admin:admin@localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @dashboards/mcp-overview.json
```

## Alerting Rules

### Prometheus Alert Rules
```yaml
# alerts/rules.yml
groups:
  - name: mcp_critical
    interval: 30s
    rules:
      - alert: HighErrorRate
        expr: |
          sum(rate(http_requests_total{status=~"5.."}[5m])) by (service) 
          / sum(rate(http_requests_total[5m])) by (service) > 0.05
        for: 5m
        labels:
          severity: critical
          team: platform
        annotations:
          summary: "High error rate on {{ $labels.service }}"
          description: "{{ $labels.service }} has {{ $value | humanizePercentage }} error rate"

      - alert: HighLatency
        expr: |
          histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (service, le)) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency on {{ $labels.service }}"
          description: "P99 latency is {{ $value }}s"

      - alert: MemoryLeak
        expr: |
          rate(go_memstats_alloc_bytes[5m]) > 0 and 
          predict_linear(go_memstats_alloc_bytes[1h], 4*3600) > 8e9
        for: 30m
        labels:
          severity: warning
        annotations:
          summary: "Potential memory leak in {{ $labels.service }}"

      - alert: GoroutineLeak
        expr: |
          rate(go_goroutines[5m]) > 0 and
          predict_linear(go_goroutines[1h], 4*3600) > 10000
        for: 30m
        labels:
          severity: warning
```

### AlertManager Configuration
```yaml
# alertmanager/config.yml
global:
  resolve_timeout: 5m
  slack_api_url: ${SLACK_WEBHOOK_URL}

route:
  group_by: ['alertname', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'default'
  routes:
    - match:
        severity: critical
      receiver: pagerduty
      continue: true
    - match:
        severity: warning
      receiver: slack

receivers:
  - name: 'default'
    # No-op

  - name: 'slack'
    slack_configs:
      - channel: '#mcp-alerts'
        title: 'MCP Alert'
        text: '{{ range .Alerts }}{{ .Annotations.summary }}\n{{ end }}'

  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: ${PAGERDUTY_SERVICE_KEY}
```

## SLI/SLO Definitions

### Service Level Indicators
```yaml
# SLI definitions
slis:
  availability:
    description: "Percentage of successful requests"
    query: |
      sum(rate(http_requests_total{status!~"5.."}[5m])) 
      / sum(rate(http_requests_total[5m]))
    
  latency:
    description: "95th percentile latency"
    query: |
      histogram_quantile(0.95, 
        sum(rate(http_request_duration_seconds_bucket[5m])) by (le)
      )
  
  error_rate:
    description: "Percentage of failed requests"
    query: |
      sum(rate(http_requests_total{status=~"5.."}[5m])) 
      / sum(rate(http_requests_total[5m]))
```

### Service Level Objectives
```yaml
# SLO targets
slos:
  - name: "API Availability"
    sli: availability
    target: 99.9
    window: 30d
    
  - name: "API Latency"
    sli: latency
    target_value: 0.5  # 500ms
    window: 7d
    
  - name: "Error Budget"
    sli: error_rate
    target: 0.1  # 0.1% error rate
    window: 30d
```

### SLO Dashboard
```json
{
  "panels": [
    {
      "title": "SLO Status",
      "targets": [{
        "expr": "1 - (sum(rate(http_requests_total{status=~\"5..\"}[30d])) / sum(rate(http_requests_total[30d])))"
      }],
      "thresholds": [
        {"value": 0.999, "color": "green"},
        {"value": 0.995, "color": "yellow"},
        {"value": 0, "color": "red"}
      ]
    }
  ]
}
```

## Performance Monitoring

### Database Queries
```sql
-- Slow query monitoring
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- View slow queries
SELECT 
  query,
  calls,
  mean_exec_time,
  max_exec_time,
  total_exec_time
FROM pg_stat_statements
WHERE mean_exec_time > 100  -- queries slower than 100ms
ORDER BY mean_exec_time DESC
LIMIT 20;
```

### Redis Monitoring
```bash
# Redis metrics to monitor
redis-cli INFO stats | grep -E "instantaneous_ops_per_sec|used_memory_human|connected_clients|blocked_clients"

# Slow log
redis-cli SLOWLOG GET 10
```

## Debugging Production Issues

### Enable Debug Logging
```bash
# Temporarily enable debug logging
curl -X PUT http://localhost:8080/admin/log-level \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"level": "debug"}'
```

### Trace Specific Request
```bash
# Add trace header
curl -H "X-Debug-Trace: true" \
     -H "X-Request-ID: debug-123" \
     http://localhost:8080/api/v1/contexts

# View trace in Jaeger
open http://localhost:16686/trace/debug-123
```

### Memory Profiling
```bash
# Get heap profile
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof -http=:8081 heap.prof

# Get goroutine profile
curl http://localhost:8080/debug/pprof/goroutine > goroutine.prof
```

## Best Practices

1. **Use structured logging** with consistent field names
2. **Add trace IDs** to all log entries
3. **Set up alerts** before issues occur
4. **Monitor SLOs** not just system metrics
5. **Use sampling** for high-volume tracing
6. **Implement custom metrics** for business KPIs
7. **Regular review** of dashboard effectiveness

## Troubleshooting

### High Memory Usage
1. Check for goroutine leaks: `/debug/pprof/goroutine`
2. Analyze heap profile: `/debug/pprof/heap`
3. Review recent deployments
4. Check for increased traffic

### High Latency
1. Check database slow queries
2. Review Redis latency
3. Check external API calls
4. Analyze trace data

### Missing Metrics
1. Verify Prometheus targets: http://localhost:9090/targets
2. Check service discovery
3. Verify metrics endpoint: `curl http://service:8080/metrics`