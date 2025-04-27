# MCP Server Monitoring Guide

This guide provides detailed information on monitoring the MCP Server, including available metrics, health checks, log analysis, and recommended alerting strategies.

## Monitoring Architecture

The MCP Server is designed with observability in mind and provides multiple ways to monitor its health and performance:

1. **Health Checks**: HTTP endpoints for basic health verification
2. **Metrics**: Prometheus metrics for detailed performance monitoring
3. **Logs**: Structured logging for event tracking and debugging
4. **Tracing**: Distributed tracing for request flow analysis (if configured)

## Health Checks

### Health Endpoint

The MCP Server provides a `/health` endpoint that returns the health status of all components:

```
GET /health
```

Example response:

```json
{
  "status": "healthy",
  "components": {
    "engine": "healthy",
    "github": "healthy",
    "harness": "healthy",
    "sonarqube": "healthy",
    "artifactory": "healthy",
    "xray": "healthy"
  }
}
```

If any component is unhealthy, the status will be "unhealthy" and the HTTP status code will be 503 (Service Unavailable).

### Component Health Status

Each component reports its health status based on:

- **Engine**: Overall health of the core processing engine
- **Adapters**: Connection status to external systems (GitHub, Harness, etc.)
- **Database**: Connection to the PostgreSQL database
- **Cache**: Connection to the Redis cache

### Health Check Configuration

You can use the health endpoint in load balancers, Kubernetes liveness probes, and monitoring systems:

#### Kubernetes Example

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
```

#### Docker Compose Example

```yaml
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
  interval: 30s
  timeout: 5s
  retries: 3
  start_period: 10s
```

## Metrics

### Prometheus Integration

The MCP Server exports metrics in Prometheus format at the `/metrics` endpoint:

```
GET /metrics
```

This endpoint requires authentication with an API key.

### Key Metrics

#### API Metrics

- `mcp_api_requests_total{method, path, status_code}`: Total number of API requests
- `mcp_api_request_duration_seconds{method, path}`: API request duration histogram
- `mcp_api_errors_total{method, path, error_type}`: Total number of API errors

#### Event Metrics

- `mcp_events_total{source, type}`: Total number of events by source and type
- `mcp_event_processing_duration_seconds{source, type}`: Event processing duration histogram
- `mcp_event_errors_total{source, type, error_type}`: Total number of event processing errors

#### Adapter Metrics

- `mcp_adapter_api_calls_total{adapter, operation}`: Total number of API calls to external systems
- `mcp_adapter_api_call_duration_seconds{adapter, operation}`: API call duration histogram
- `mcp_adapter_errors_total{adapter, operation, error_type}`: Total number of adapter errors

#### System Metrics

- `mcp_goroutines`: Number of goroutines
- `mcp_memory_usage_bytes`: Memory usage
- `mcp_event_queue_length`: Number of events waiting to be processed
- `mcp_database_connections`: Number of active database connections
- `mcp_cache_operations_total{operation}`: Total number of cache operations

### Grafana Dashboards

The Docker Compose setup includes Grafana with pre-configured dashboards:

1. **MCP Overview**: High-level system metrics
2. **API Performance**: Detailed API performance metrics
3. **Event Processing**: Event processing metrics
4. **External Systems**: Metrics for communication with external systems

Grafana is accessible at `http://localhost:3000` with default credentials (admin/admin).

### Metrics Configuration

You can configure metrics collection in the `metrics` section of the configuration file:

```yaml
metrics:
  enabled: true
  type: "prometheus"
  endpoint: "localhost:9090"
  push_gateway: ""
  push_interval: 10s
```

## Logging

### Log Levels

The MCP Server uses the following log levels:

- **DEBUG**: Detailed debugging information
- **INFO**: General information about system operation
- **WARN**: Warnings that don't affect normal operation
- **ERROR**: Errors that affect specific operations
- **FATAL**: Critical errors that require immediate attention

### Log Format

Logs are output in a structured format:

```
[2023-04-29T12:34:56Z] [INFO] [component=api] [request_id=abcd1234] Request received: GET /api/v1/github/repos
```

### Log Collection

In production environments, you should collect logs using a log aggregation system like ELK Stack, Graylog, or cloud-based solutions.

#### Example: Fluentd Configuration

```
<source>
  @type tail
  path /var/log/mcp-server.log
  pos_file /var/log/mcp-server.log.pos
  tag mcp.server
  <parse>
    @type regexp
    expression /^\[(?<time>[^\]]*)\] \[(?<level>[^\]]*)\] \[(?<component>[^\]]*)\] (?<message>.*)$/
    time_format %Y-%m-%dT%H:%M:%SZ
  </parse>
</source>
```

### Log Storage

For production, consider:

- Storing logs for at least 30 days
- Implementing log rotation
- Using log compression for storage efficiency
- Setting up retention policies based on compliance requirements

## Tracing

### Distributed Tracing

The MCP Server supports distributed tracing using OpenTelemetry (if configured). This allows you to trace requests across multiple components.

### Trace Sampling

To reduce overhead, tracing uses sampling:

```yaml
tracing:
  enabled: true
  type: "jaeger"
  endpoint: "localhost:6831"
  service_name: "mcp-server"
  sample_ratio: 0.1  # Sample 10% of requests
```

### Tracing Visualization

Traces can be visualized using Jaeger UI or Zipkin UI, depending on your configuration.

## Alerting

### Recommended Alerts

The following alerts are recommended for production deployments:

#### High Priority Alerts

- **Service Health**: Alert when the health check fails
- **High Error Rate**: Alert when error rate exceeds 5% over 5 minutes
- **API Latency**: Alert when P95 API latency exceeds thresholds
- **Event Processing Delay**: Alert when events are processing slowly

#### Medium Priority Alerts

- **External System Connection**: Alert when connection to external systems fails
- **Database Connection**: Alert when database connections are limited
- **Cache Connection**: Alert when cache connections are limited
- **High Resource Usage**: Alert on high CPU or memory usage

#### Low Priority Alerts

- **Rate Limit Reached**: Alert when API rate limits are frequently hit
- **Slow Queries**: Alert on slow database queries
- **High Event Volume**: Alert on unusual event volumes

### Alert Configuration Example (Prometheus Alertmanager)

```yaml
groups:
- name: mcp-alerts
  rules:
  - alert: HighErrorRate
    expr: rate(mcp_api_errors_total[5m]) / rate(mcp_api_requests_total[5m]) > 0.05
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "High API error rate"
      description: "Error rate is {{ $value | humanizePercentage }} over the last 5 minutes"
```

## Dashboard and Visualization

### Main Dashboards

#### 1. System Overview Dashboard

Key metrics:
- API request rate
- Event processing rate
- Error rates
- Resource usage
- Component health status

#### 2. API Performance Dashboard

Key metrics:
- Request rates by endpoint
- Response times
- Error rates by endpoint
- Rate limit usage

#### 3. Event Processing Dashboard

Key metrics:
- Event rates by source and type
- Processing latency
- Queue length
- Processing errors

#### 4. External Systems Dashboard

Key metrics:
- API call rates by system
- Response times
- Error rates
- Connectivity status

### Dashboard Configuration

The `configs/grafana/dashboards` directory contains pre-configured dashboards that are loaded into Grafana automatically.

## Performance Monitoring

### Key Performance Indicators (KPIs)

Monitor these KPIs to ensure optimal performance:

1. **API Response Time**: Should be <100ms for p95
2. **Event Processing Time**: Should be <500ms for p95
3. **External API Call Time**: Should be <1s for p95
4. **CPU Usage**: Should stay below 70% sustained
5. **Memory Usage**: Should stay below 80% of limit
6. **Event Queue Length**: Should stay near zero during normal operation
7. **Error Rate**: Should stay below 1% for production

### Performance Degradation Alerts

Set up alerts for performance degradation:

1. **Slow API Response**: Alert when p95 response time exceeds 300ms for 5 minutes
2. **Slow Event Processing**: Alert when p95 processing time exceeds 1.5s for 5 minutes
3. **Growing Event Queue**: Alert when queue length exceeds 100 for 5 minutes

## Capacity Planning

### Resource Requirements

Baseline resource requirements for different scales:

| Scale | Users | Events/min | CPU Cores | Memory | Database | Cache |
|-------|-------|------------|-----------|--------|----------|-------|
| Small | <10   | <100       | 2         | 2GB    | 2GB      | 1GB   |
| Medium| <50   | <1,000     | 4         | 4GB    | 4GB      | 2GB   |
| Large | <200  | <10,000    | 8+        | 8GB+   | 8GB+     | 4GB+  |

### Scaling Indicators

Watch these indicators to determine when to scale:

1. **CPU Utilization**: Above 70% sustained
2. **Memory Utilization**: Above 80% sustained
3. **Event Queue Growth**: Consistent backlog
4. **API Response Slowdown**: Consistent increase in response times

## Monitoring in Different Environments

### Local Development

For local development, use:
- Built-in health checks
- Docker Compose with included monitoring stack
- Local Grafana dashboards

### Testing/Staging

For testing environments:
- Lightweight monitoring with basic alerting
- Event sampling (store only a percentage of events)
- Performance testing with detailed metrics

### Production

For production environments:
- Full monitoring coverage with all recommended alerts
- Log aggregation and analysis
- Regular capacity planning reviews
- Automated scaling based on metrics

## Troubleshooting Common Issues

### High API Latency

If you observe high API latency:

1. Check database connection and query performance
2. Check external system response times
3. Review recent code or configuration changes
4. Check for resource constraints (CPU, memory)

### Growing Event Queue

If the event queue keeps growing:

1. Check event processor health
2. Increase the concurrency limit for event processing
3. Check for slow external system responses
4. Review event handler performance

### High Error Rates

If you see high error rates:

1. Check logs for specific error messages
2. Verify external system connectivity
3. Check for recent deployments or changes
4. Verify configuration is correct

### External System Connectivity

If adapters show unhealthy status:

1. Verify external system is available
2. Check API credentials and tokens
3. Review external system rate limits
4. Check network connectivity and firewall rules

## Further Reading

- [Prometheus Documentation](https://prometheus.io/docs/introduction/overview/)
- [Grafana Documentation](https://grafana.com/docs/grafana/latest/)
- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [ELK Stack Documentation](https://www.elastic.co/guide/index.html)