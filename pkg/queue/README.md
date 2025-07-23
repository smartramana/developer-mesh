# Queue Package

> **Purpose**: AWS SQS integration for event processing in the Developer Mesh platform
> **Status**: Basic Implementation
> **Dependencies**: AWS SQS, LocalStack support, JSON serialization

## Overview

The queue package provides SQS integration for processing webhook events and other asynchronous tasks. It supports both production AWS SQS and LocalStack for local development, with a mock implementation for testing.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Queue Architecture                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Event Producers ──► SQS Client ──► AWS SQS ──► Workers    │
│                          │                                   │
│                          ├── Production SQS                 │
│                          ├── LocalStack                     │
│                          └── Mock Queue                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. SQSAdapter Interface

```go
// SQSAdapter interface represents the high-level operations needed by the worker
type SQSAdapter interface {
    // EnqueueEvent sends a message to SQS
    EnqueueEvent(ctx context.Context, event SQSEvent) error
    
    // ReceiveEvents receives messages from SQS
    ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]SQSEvent, []string, error)
    
    // DeleteMessage deletes a message from SQS
    DeleteMessage(ctx context.Context, receiptHandle string) error
}
```

### 2. Event Definition

```go
// SQSEvent represents an event in the SQS queue with auth context
type SQSEvent struct {
    DeliveryID  string          `json:"delivery_id"`
    EventType   string          `json:"event_type"`
    RepoName    string          `json:"repo_name"`
    SenderName  string          `json:"sender_name"`
    Payload     json.RawMessage `json:"payload"`
    AuthContext *EventAuthContext `json:"auth_context,omitempty"`
}

// EventAuthContext contains authentication context for queue events
type EventAuthContext struct {
    TenantID       string                 `json:"tenant_id"`
    PrincipalID    string                 `json:"principal_id"`
    PrincipalType  string                 `json:"principal_type"`
    InstallationID *int64                 `json:"installation_id,omitempty"`
    AppID          *int64                 `json:"app_id,omitempty"`
    Permissions    []string               `json:"permissions,omitempty"`
    Metadata       map[string]interface{} `json:"metadata,omitempty"`
}
```

### 3. SQS Client Implementation

```go
// SQSClient provides basic SQS operations
type SQSClient struct {
    Client   SQSAPI
    QueueURL string
}

// SQSClientAdapter adapts between production SQS, LocalStack, and mock implementations
type SQSClientAdapter struct {
    client   SQSReceiverDeleter
    queueURL string
    config   *SQSAdapterConfig
}

// SQSAdapterConfig holds configuration for the SQS adapter
type SQSAdapterConfig struct {
    MockMode      bool   `json:"mock_mode"`
    UseLocalStack bool   `json:"use_localstack"`
    Region        string `json:"region"`
    QueueName     string `json:"queue_name"`
    QueueURL      string `json:"queue_url"`
    Endpoint      string `json:"endpoint"`
    AccessKey     string `json:"access_key"`
    SecretKey     string `json:"secret_key"`
}
```

## Implementation Types

### 1. Production SQS Client

```go
// Production AWS SQS client
func NewSQSClient(ctx context.Context) (*SQSClient, error) {
    cfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        return nil, err
    }
    client := sqs.NewFromConfig(cfg)
    queueURL := os.Getenv("SQS_QUEUE_URL")
    return &SQSClient{Client: client, QueueURL: queueURL}, nil
}
```

### 2. LocalStack Support

```go
// LocalStack SQS client for development
func createLocalStackSQSClient(ctx context.Context, config *SQSAdapterConfig) (*sqs.Client, error) {
    // Custom HTTP client to allow insecure connections for LocalStack
    customTransport := http.DefaultTransport.(*http.Transport).Clone()
    customTransport.TLSClientConfig = &tls.Config{
        MinVersion:         tls.VersionTLS12,
        InsecureSkipVerify: true, // LocalStack development only
    }
    
    // Create AWS config with LocalStack endpoint
    cfg, err := awsconfig.LoadDefaultConfig(ctx,
        awsconfig.WithHTTPClient(customHTTPClient),
        awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
            config.AccessKey, config.SecretKey, "")),
        awsconfig.WithRegion(config.Region),
    )
    
    // Create SQS client with custom endpoint
    sqsClient := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
        o.BaseEndpoint = aws.String(config.Endpoint)
    })
    return sqsClient, nil
}
```

### 3. Mock Implementation

```go
// MockSQSClient for testing
type MockSQSClient struct {
    messages []*types.Message
}

func NewMockSQSClient() *SQSClientAdapter {
    mockClient := &MockSQSClient{
        messages: []*types.Message{},
    }
    
    return &SQSClientAdapter{
        client:   mockClient,
        queueURL: "mock-queue-url",
        config:   &SQSAdapterConfig{MockMode: true},
    }
}
```

## Basic Operations

### 1. Sending Events

```go
// Create SQS client
client, err := NewSQSClient(ctx)
if err != nil {
    return err
}

// Create event
event := SQSEvent{
    DeliveryID: "webhook-123",
    EventType:  "push",
    RepoName:   "owner/repo",
    SenderName: "user",
    Payload:    json.RawMessage(`{"ref": "refs/heads/main"}`),
    AuthContext: &EventAuthContext{
        TenantID:      "tenant-123",
        PrincipalID:   "user-456",
        PrincipalType: "user",
    },
}

// Send to SQS
err = client.EnqueueEvent(ctx, event)
```

### 2. Receiving Events

```go
// Receive up to 10 messages with 20s long polling
events, receiptHandles, err := client.ReceiveEvents(ctx, 10, 20)
if err != nil {
    return err
}

// Process events
for i, event := range events {
    // Process event
    processEvent(event)
    
    // Delete message after processing
    err = client.DeleteMessage(ctx, receiptHandles[i])
    if err != nil {
        log.Printf("Failed to delete message: %v", err)
    }
}
```

### 3. Using the Adapter

```go
// Load configuration
config := LoadSQSConfigFromEnv()

// Create adapter (supports production, LocalStack, mock)
adapter, err := NewSQSClientAdapter(ctx, config)
if err != nil {
    return err
}

// Use same interface regardless of implementation
err = adapter.EnqueueEvent(ctx, event)
events, handles, err := adapter.ReceiveEvents(ctx, 10, 20)
err = adapter.DeleteMessage(ctx, handle)

```

## Configuration

### Environment Variables

```bash
# SQS Configuration
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test
AWS_REGION=us-east-1

# LocalStack Configuration (development)
USE_LOCALSTACK=true
AWS_ENDPOINT_URL=http://localstack:4566
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
SQS_QUEUE_NAME=tasks

# Mock Mode (testing)
WORKER_MOCK_MODE=true
```

### Configuration Structure

```go
// LoadSQSConfigFromEnv loads SQS configuration from environment
func LoadSQSConfigFromEnv() *SQSAdapterConfig {
    config := &SQSAdapterConfig{
        MockMode:      os.Getenv("WORKER_MOCK_MODE") == "true",
        UseLocalStack: os.Getenv("USE_LOCALSTACK") == "true",
        Region:        getEnvWithDefault("AWS_REGION", "us-east-1"),
        QueueName:     getEnvWithDefault("SQS_QUEUE_NAME", "tasks"),
        QueueURL:      os.Getenv("SQS_QUEUE_URL"),
        Endpoint:      os.Getenv("AWS_ENDPOINT_URL"),
        AccessKey:     getEnvWithDefault("AWS_ACCESS_KEY_ID", "test"),
        SecretKey:     getEnvWithDefault("AWS_SECRET_ACCESS_KEY", "test"),
    }
    
    if config.UseLocalStack && config.Endpoint == "" {
        config.Endpoint = "http://localstack:4566"
    }
    
    return config
}
```

## Testing

### Using Mock Client

```go
// Create mock client for testing
mockClient := NewMockSQSClient()

// Use in tests
event := SQSEvent{
    DeliveryID: "test-123",
    EventType:  "push",
}

err := mockClient.EnqueueEvent(ctx, event)
events, handles, err := mockClient.ReceiveEvents(ctx, 10, 0)
```

### Integration Testing

```go
func TestSQSIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    ctx := context.Background()
    
    // Use real SQS or LocalStack
    config := &SQSAdapterConfig{
        UseLocalStack: true,
        Region:        "us-east-1",
        QueueName:     "test-queue",
        Endpoint:      "http://localhost:4566",
    }
    
    client, err := NewSQSClientAdapter(ctx, config)
    assert.NoError(t, err)
    
    // Test operations
    event := SQSEvent{
        DeliveryID: "test-123",
        EventType:  "push",
    }
    
    err = client.EnqueueEvent(ctx, event)
    assert.NoError(t, err)
    
    events, handles, err := client.ReceiveEvents(ctx, 1, 5)
    assert.NoError(t, err)
    assert.Len(t, events, 1)
    
    err = client.DeleteMessage(ctx, handles[0])
    assert.NoError(t, err)
}
```

## Implementation Status

**Implemented:**
- Basic SQS client with send/receive/delete operations
- SQS adapter supporting production AWS, LocalStack, and mock
- Event structure with authentication context
- Configuration loading from environment
- Mock implementation for testing

**Not Implemented:**
- Task/Worker abstractions (only webhook events)
- Priority queues or FIFO queues
- Retry mechanisms
- Dead letter queue handling
- Visibility timeout management
- Metrics and monitoring
- Batch operations
- Advanced error handling

## Best Practices

1. **Message Size**: Keep messages under 256KB for SQS
2. **Long Polling**: Use wait time to reduce API calls
3. **Idempotency**: Ensure event processing is idempotent
4. **Error Handling**: Delete messages only after successful processing
5. **Auth Context**: Include authentication context in events
6. **LocalStack**: Use for local development to avoid AWS costs

---

Package Version: 0.1.0 (Basic Implementation)
Last Updated: 2024-01-23