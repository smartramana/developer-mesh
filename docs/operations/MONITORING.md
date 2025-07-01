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

#### WebSocket Metrics
- `mcp_websocket_connections_active` - Current active WebSocket connections
- `mcp_websocket_connections_total` - Total WebSocket connections established
- `mcp_websocket_messages_sent_total` - Messages sent by type and compression
- `mcp_websocket_messages_received_total` - Messages received by type
- `mcp_websocket_message_size_bytes` - Message size distribution
- `mcp_websocket_compression_ratio` - Compression effectiveness
- `mcp_websocket_protocol_version` - Protocol version in use
- `mcp_websocket_errors_total` - WebSocket errors by type
- `mcp_websocket_ping_latency_seconds` - Ping/pong latency

#### Agent Metrics
- `mcp_agents_registered` - Number of registered agents by type
- `mcp_agent_workload_current` - Current workload per agent
- `mcp_agent_tasks_completed_total` - Tasks completed by agent
- `mcp_agent_task_duration_seconds` - Task processing time by agent
- `mcp_agent_capabilities` - Agent capabilities matrix

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

### WebSocket Metrics Implementation
```go
// WebSocket-specific metrics
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Connection metrics
    wsConnectionsActive = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_websocket_connections_active",
            Help: "Number of active WebSocket connections",
        },
        []string{"protocol_version", "agent_type"},
    )
    
    wsConnectionsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_websocket_connections_total",
            Help: "Total WebSocket connections established",
        },
        []string{"protocol_version", "status"},
    )
    
    // Message metrics
    wsMessagesSent = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_websocket_messages_sent_total",
            Help: "Total messages sent via WebSocket",
        },
        []string{"message_type", "compressed", "protocol_version"},
    )
    
    wsMessagesReceived = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_websocket_messages_received_total",
            Help: "Total messages received via WebSocket",
        },
        []string{"message_type", "protocol_version"},
    )
    
    wsMessageSize = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "mcp_websocket_message_size_bytes",
            Help: "Size of WebSocket messages in bytes",
            Buckets: prometheus.ExponentialBuckets(128, 2, 12), // 128B to 256KB
        },
        []string{"direction", "message_type", "compressed"},
    )
    
    wsCompressionRatio = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name: "mcp_websocket_compression_ratio",
            Help: "Compression ratio for WebSocket messages",
            Buckets: prometheus.LinearBuckets(0, 0.1, 11), // 0% to 100%
        },
    )
    
    wsPingLatency = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name: "mcp_websocket_ping_latency_seconds",
            Help: "WebSocket ping/pong latency",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to 1s
        },
    )
)

// Track WebSocket connection
func TrackWSConnection(protocolVersion, agentType string, connected bool) {
    if connected {
        wsConnectionsActive.WithLabelValues(protocolVersion, agentType).Inc()
        wsConnectionsTotal.WithLabelValues(protocolVersion, "established").Inc()
    } else {
        wsConnectionsActive.WithLabelValues(protocolVersion, agentType).Dec()
        wsConnectionsTotal.WithLabelValues(protocolVersion, "closed").Inc()
    }
}

// Track WebSocket message
func TrackWSMessage(direction string, msg *WSMessage) {
    compressed := "false"
    if msg.Flags&FlagCompressed != 0 {
        compressed = "true"
    }
    
    labels := []string{msg.Type.String(), compressed, msg.Version.String()}
    
    if direction == "sent" {
        wsMessagesSent.WithLabelValues(labels...).Inc()
    } else {
        wsMessagesReceived.WithLabelValues(msg.Type.String(), msg.Version.String()).Inc()
    }
    
    wsMessageSize.WithLabelValues(direction, msg.Type.String(), compressed).
        Observe(float64(len(msg.Payload)))
    
    // Track compression ratio if compressed
    if compressed == "true" && msg.OriginalSize > 0 {
        ratio := float64(len(msg.Payload)) / float64(msg.OriginalSize)
        wsCompressionRatio.Observe(1 - ratio) // Savings percentage
    }
}

// Track ping latency
func TrackPingLatency(latency time.Duration) {
    wsPingLatency.Observe(latency.Seconds())
}
```

### Agent Metrics Implementation
```go
// Agent-specific metrics
var (
    agentsRegistered = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_agents_registered",
            Help: "Number of registered agents",
        },
        []string{"agent_type", "model", "status"},
    )
    
    agentWorkload = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_agent_workload_current",
            Help: "Current workload percentage per agent",
        },
        []string{"agent_id", "agent_type"},
    )
    
    agentTasksCompleted = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_agent_tasks_completed_total",
            Help: "Total tasks completed by agent",
        },
        []string{"agent_id", "agent_type", "task_type", "status"},
    )
    
    agentTaskDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "mcp_agent_task_duration_seconds",
            Help: "Task processing duration by agent",
            Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 100ms to 100s
        },
        []string{"agent_type", "task_type"},
    )
)

// Track agent registration
func TrackAgentRegistration(agent *Agent) {
    agentsRegistered.WithLabelValues(
        agent.Type, 
        agent.Model, 
        agent.Status.String(),
    ).Inc()
}

// Update agent workload
func UpdateAgentWorkload(agentID, agentType string, workload float64) {
    agentWorkload.WithLabelValues(agentID, agentType).Set(workload)
}

// Track task completion
func TrackTaskCompletion(agent *Agent, task *Task, duration time.Duration, status string) {
    agentTasksCompleted.WithLabelValues(
        agent.ID,
        agent.Type,
        task.Type,
        status,
    ).Inc()
    
    agentTaskDuration.WithLabelValues(
        agent.Type,
        task.Type,
    ).Observe(duration.Seconds())
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

// WebSocket tracing with custom attributes
func HandleWebSocketMessage(ctx context.Context, msg *Message) {
    tracer := otel.Tracer("mcp-websocket")
    ctx, span := tracer.Start(ctx, "HandleWebSocketMessage",
        trace.WithSpanKind(trace.SpanKindServer))
    defer span.End()
    
    // Custom WebSocket attributes
    span.SetAttributes(
        attribute.String("ws.connection_id", msg.ConnectionID),
        attribute.String("ws.message_type", msg.Type.String()),
        attribute.Int("ws.sequence_id", int(msg.SequenceID)),
        attribute.Int("ws.protocol_version", int(msg.Version)),
        attribute.Bool("ws.compressed", msg.IsCompressed()),
        attribute.Int("ws.message_size", len(msg.Payload)),
        attribute.String("ws.encoding", msg.Encoding),
    )
    
    // Agent-specific attributes
    if msg.AgentID != "" {
        span.SetAttributes(
            attribute.String("agent.id", msg.AgentID),
            attribute.String("agent.type", msg.AgentType),
            attribute.StringSlice("agent.capabilities", msg.Capabilities),
            attribute.Float64("agent.workload", msg.CurrentWorkload),
        )
    }
    
    // Task routing attributes
    if msg.TaskID != "" {
        span.SetAttributes(
            attribute.String("task.id", msg.TaskID),
            attribute.String("task.type", msg.TaskType),
            attribute.String("task.routing_strategy", msg.RoutingStrategy),
            attribute.Float64("task.priority", msg.Priority),
            attribute.Float64("task.estimated_cost", msg.EstimatedCost),
        )
    }
}

// Custom span attributes for different operations
var CustomSpanAttributes = struct {
    // WebSocket attributes
    WebSocket struct {
        ConnectionID    attribute.Key
        MessageType     attribute.Key
        ProtocolVersion attribute.Key
        Compressed      attribute.Key
        MessageSize     attribute.Key
        PingLatency     attribute.Key
    }
    
    // Agent attributes
    Agent struct {
        ID           attribute.Key
        Type         attribute.Key
        Model        attribute.Key
        Capabilities attribute.Key
        Workload     attribute.Key
        Status       attribute.Key
    }
    
    // Task attributes
    Task struct {
        ID               attribute.Key
        Type             attribute.Key
        RoutingStrategy  attribute.Key
        Priority         attribute.Key
        EstimatedCost    attribute.Key
        ActualCost       attribute.Key
        ProcessingTime   attribute.Key
    }
    
    // Cost tracking attributes
    Cost struct {
        Model         attribute.Key
        InputTokens   attribute.Key
        OutputTokens  attribute.Key
        TotalCost     attribute.Key
        SessionLimit  attribute.Key
        RemainingBudget attribute.Key
    }
}{
    WebSocket: struct {
        ConnectionID    attribute.Key
        MessageType     attribute.Key
        ProtocolVersion attribute.Key
        Compressed      attribute.Key
        MessageSize     attribute.Key
        PingLatency     attribute.Key
    }{
        ConnectionID:    attribute.Key("ws.connection_id"),
        MessageType:     attribute.Key("ws.message_type"),
        ProtocolVersion: attribute.Key("ws.protocol_version"),
        Compressed:      attribute.Key("ws.compressed"),
        MessageSize:     attribute.Key("ws.message_size"),
        PingLatency:     attribute.Key("ws.ping_latency_ms"),
    },
    Agent: struct {
        ID           attribute.Key
        Type         attribute.Key
        Model        attribute.Key
        Capabilities attribute.Key
        Workload     attribute.Key
        Status       attribute.Key
    }{
        ID:           attribute.Key("agent.id"),
        Type:         attribute.Key("agent.type"),
        Model:        attribute.Key("agent.model"),
        Capabilities: attribute.Key("agent.capabilities"),
        Workload:     attribute.Key("agent.workload"),
        Status:       attribute.Key("agent.status"),
    },
    Task: struct {
        ID               attribute.Key
        Type             attribute.Key
        RoutingStrategy  attribute.Key
        Priority         attribute.Key
        EstimatedCost    attribute.Key
        ActualCost       attribute.Key
        ProcessingTime   attribute.Key
    }{
        ID:              attribute.Key("task.id"),
        Type:            attribute.Key("task.type"),
        RoutingStrategy: attribute.Key("task.routing_strategy"),
        Priority:        attribute.Key("task.priority"),
        EstimatedCost:   attribute.Key("task.estimated_cost_usd"),
        ActualCost:      attribute.Key("task.actual_cost_usd"),
        ProcessingTime:  attribute.Key("task.processing_time_ms"),
    },
    Cost: struct {
        Model         attribute.Key
        InputTokens   attribute.Key
        OutputTokens  attribute.Key
        TotalCost     attribute.Key
        SessionLimit  attribute.Key
        RemainingBudget attribute.Key
    }{
        Model:           attribute.Key("cost.model"),
        InputTokens:     attribute.Key("cost.input_tokens"),
        OutputTokens:     attribute.Key("cost.output_tokens"),
        TotalCost:       attribute.Key("cost.total_usd"),
        SessionLimit:    attribute.Key("cost.session_limit_usd"),
        RemainingBudget: attribute.Key("cost.remaining_budget_usd"),
    },
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

### WebSocket Dashboard
```json
{
  "dashboard": {
    "title": "WebSocket Monitoring",
    "panels": [
      {
        "title": "Active WebSocket Connections",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
        "targets": [{
          "expr": "sum(mcp_websocket_connections_active) by (protocol_version, agent_type)"
        }]
      },
      {
        "title": "WebSocket Message Rate",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
        "targets": [{
          "expr": "sum(rate(mcp_websocket_messages_sent_total[5m])) by (message_type)",
          "legendFormat": "Sent - {{message_type}}"
        }, {
          "expr": "sum(rate(mcp_websocket_messages_received_total[5m])) by (message_type)",
          "legendFormat": "Received - {{message_type}}"
        }]
      },
      {
        "title": "Message Size Distribution",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8},
        "targets": [{
          "expr": "histogram_quantile(0.95, sum(rate(mcp_websocket_message_size_bytes_bucket[5m])) by (message_type, le))"
        }]
      },
      {
        "title": "Compression Effectiveness",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8},
        "targets": [{
          "expr": "histogram_quantile(0.5, sum(rate(mcp_websocket_compression_ratio_bucket[5m])) by (le)) * 100",
          "legendFormat": "Median Compression %"
        }]
      },
      {
        "title": "WebSocket Ping Latency",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 16},
        "targets": [{
          "expr": "histogram_quantile(0.99, sum(rate(mcp_websocket_ping_latency_seconds_bucket[5m])) by (le)) * 1000",
          "legendFormat": "P99 Latency (ms)"
        }]
      },
      {
        "title": "Protocol Version Distribution",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 16},
        "targets": [{
          "expr": "sum(mcp_websocket_connections_active) by (protocol_version)"
        }],
        "type": "piechart"
      }
    ]
  }
}
```

### Agent Performance Dashboard
```json
{
  "dashboard": {
    "title": "Agent Performance",
    "panels": [
      {
        "title": "Registered Agents",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
        "targets": [{
          "expr": "sum(mcp_agents_registered) by (agent_type, model)"
        }]
      },
      {
        "title": "Agent Workload Distribution",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
        "targets": [{
          "expr": "mcp_agent_workload_current",
          "legendFormat": "{{agent_id}} ({{agent_type}})"
        }]
      },
      {
        "title": "Task Completion Rate",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8},
        "targets": [{
          "expr": "sum(rate(mcp_agent_tasks_completed_total[5m])) by (agent_type, status)"
        }]
      },
      {
        "title": "Task Processing Time (P95)",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8},
        "targets": [{
          "expr": "histogram_quantile(0.95, sum(rate(mcp_agent_task_duration_seconds_bucket[5m])) by (agent_type, task_type, le))"
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