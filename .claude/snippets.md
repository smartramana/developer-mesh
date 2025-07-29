# Code Snippets for DevOps MCP

## Error Handling

### Wrap errors with context
```go
if err != nil {
    return nil, fmt.Errorf("failed to %s: %w", action, err)
}
```

### Handle deferred close
```go
defer func() {
    if err := resource.Close(); err != nil {
        s.logger.Warn("Failed to close resource", map[string]interface{}{
            "error": err.Error(),
        })
    }
}()
```

## Database Patterns

### Query with context
```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

var result Model
err := s.db.GetContext(ctx, &result, query, args...)
```

### Transaction pattern
```go
tx, err := s.db.BeginTxx(ctx, nil)
if err != nil {
    return fmt.Errorf("failed to begin transaction: %w", err)
}
defer func() {
    if err != nil {
        _ = tx.Rollback()
    }
}()

// Do work...

if err := tx.Commit(); err != nil {
    return fmt.Errorf("failed to commit transaction: %w", err)
}
```

## API Patterns

### Standard API handler
```go
func (api *API) HandleEndpoint(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
        return
    }

    // Handle request...
}
```

### Pagination
```go
page := 1
if p := c.Query("page"); p != "" {
    if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
        page = parsed
    }
}

limit := 20
if l := c.Query("limit"); l != "" {
    if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
        limit = parsed
    }
}

offset := (page - 1) * limit
```

## Testing Patterns

### Mock setup
```go
mockService := new(MockService)
mockService.On("Method", mock.Anything, "expectedArg").
    Return(&Result{ID: "123"}, nil)
```

### Table test
```go
tests := []struct {
    name    string
    input   InputType
    want    OutputType
    wantErr bool
}{
    {
        name:    "success case",
        input:   InputType{},
        want:    OutputType{},
        wantErr: false,
    },
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test implementation
    })
}
```

## Redis Patterns

### Set with TTL
```go
key := fmt.Sprintf("cache:%s:%s", namespace, id)
if err := s.cache.Set(ctx, key, value, 5*time.Minute); err != nil {
    s.logger.Warn("Failed to cache", map[string]interface{}{"error": err.Error()})
}
```

### Get with fallback
```go
var result Type
if err := s.cache.Get(ctx, key, &result); err != nil {
    // Cache miss - fetch from source
    result, err = s.fetchFromSource(ctx, id)
    if err != nil {
        return nil, err
    }
    // Cache for next time
    _ = s.cache.Set(ctx, key, result, ttl)
}
```

## Logging Patterns

### Structured logging
```go
s.logger.Info("Operation completed", map[string]interface{}{
    "tenant_id": tenantID,
    "tool_id":   toolID,
    "duration":  time.Since(start).Milliseconds(),
})
```

### Error logging
```go
s.logger.Error("Operation failed", map[string]interface{}{
    "tenant_id": tenantID,
    "error":     err.Error(),
    "stack":     fmt.Sprintf("%+v", err),
})
```

## Metrics Patterns

### Record histogram
```go
if s.metricsClient != nil {
    s.metricsClient.RecordHistogram(
        "operation.duration",
        float64(time.Since(start).Milliseconds()),
        map[string]string{
            "tenant_id": tenantID,
            "status":    status,
        },
    )
}
```

### Increment counter
```go
if s.metricsClient != nil {
    s.metricsClient.IncrementCounterWithLabels(
        "operation.count",
        1,
        map[string]string{
            "tenant_id": tenantID,
            "result":    result,
        },
    )
}
```