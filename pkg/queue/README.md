# Queue Package

> **Purpose**: Distributed task queue system with AWS SQS integration for the DevOps MCP platform
> **Status**: Production Ready
> **Dependencies**: AWS SQS, task serialization, priority queuing, dead letter handling

## Overview

The queue package provides a scalable, distributed task queue system built on AWS SQS. It supports multiple queue types, priority-based processing, retry mechanisms, and comprehensive monitoring for asynchronous task execution.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Queue Architecture                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Producers ──► Queue Manager ──► SQS Queues ──► Consumers  │
│                      │               │                      │
│                      │               ├── Standard Queue     │
│                      │               ├── FIFO Queue        │
│                      │               └── DLQ               │
│                      │                                      │
│                 Visibility & Monitoring                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Queue Interface

```go
// Queue defines the core queue operations
type Queue interface {
    // Enqueue adds a task to the queue
    Enqueue(ctx context.Context, task *Task) error
    
    // EnqueueBatch adds multiple tasks
    EnqueueBatch(ctx context.Context, tasks []*Task) error
    
    // Dequeue retrieves and locks a task
    Dequeue(ctx context.Context) (*Task, error)
    
    // Complete marks a task as completed
    Complete(ctx context.Context, task *Task) error
    
    // Extend extends task visibility timeout
    Extend(ctx context.Context, task *Task, duration time.Duration) error
    
    // Release returns task to queue
    Release(ctx context.Context, task *Task) error
    
    // Stats returns queue statistics
    Stats(ctx context.Context) (*QueueStats, error)
}
```

### 2. Task Definition

```go
// Task represents a queued work item
type Task struct {
    ID              string                 `json:"id"`
    Type            string                 `json:"type"`
    Priority        int                    `json:"priority"`
    Payload         interface{}            `json:"payload"`
    Metadata        map[string]string      `json:"metadata"`
    CreatedAt       time.Time              `json:"created_at"`
    ScheduledFor    *time.Time             `json:"scheduled_for,omitempty"`
    MaxRetries      int                    `json:"max_retries"`
    RetryCount      int                    `json:"retry_count"`
    VisibilityTimeout time.Duration        `json:"visibility_timeout"`
    ReceiptHandle   string                 `json:"-"` // SQS receipt handle
}

// TaskResult represents task execution outcome
type TaskResult struct {
    TaskID    string        `json:"task_id"`
    Status    TaskStatus    `json:"status"`
    Result    interface{}   `json:"result,omitempty"`
    Error     string        `json:"error,omitempty"`
    Duration  time.Duration `json:"duration"`
    Timestamp time.Time     `json:"timestamp"`
}
```

### 3. Queue Manager

```go
// QueueManager manages multiple queues
type QueueManager struct {
    sqsClient    *sqs.Client
    queues       map[string]Queue
    config       *QueueConfig
    serializer   TaskSerializer
    metrics      *Metrics
}

// Example usage
manager := NewQueueManager(sqsClient, config)

// Register queues
manager.RegisterQueue("tasks", &SQSQueue{
    URL:      "https://sqs.us-east-1.amazonaws.com/123456/tasks",
    Type:     StandardQueue,
})

manager.RegisterQueue("priority-tasks", &SQSQueue{
    URL:      "https://sqs.us-east-1.amazonaws.com/123456/priority-tasks.fifo",
    Type:     FIFOQueue,
})
```

## Queue Types

### 1. Standard Queue

```go
// StandardQueue for high throughput, best-effort ordering
type StandardQueue struct {
    client        *sqs.Client
    queueURL      string
    visibilityTimeout int32
    maxReceiveCount   int32
}

func (q *StandardQueue) Enqueue(ctx context.Context, task *Task) error {
    // Serialize task
    body, err := json.Marshal(task)
    if err != nil {
        return fmt.Errorf("marshal task: %w", err)
    }
    
    // Send to SQS
    input := &sqs.SendMessageInput{
        QueueUrl:    &q.queueURL,
        MessageBody: aws.String(string(body)),
        MessageAttributes: map[string]types.MessageAttributeValue{
            "Type": {
                DataType:    aws.String("String"),
                StringValue: aws.String(task.Type),
            },
            "Priority": {
                DataType:    aws.String("Number"),
                StringValue: aws.String(strconv.Itoa(task.Priority)),
            },
        },
    }
    
    // Add delay if scheduled
    if task.ScheduledFor != nil {
        delay := int32(time.Until(*task.ScheduledFor).Seconds())
        if delay > 0 {
            input.DelaySeconds = delay
        }
    }
    
    _, err = q.client.SendMessage(ctx, input)
    return err
}
```

### 2. FIFO Queue

```go
// FIFOQueue for exactly-once processing and strict ordering
type FIFOQueue struct {
    client           *sqs.Client
    queueURL         string
    messageGroupID   string
    deduplicationID  func(*Task) string
}

func (q *FIFOQueue) EnqueueBatch(ctx context.Context, tasks []*Task) error {
    entries := make([]types.SendMessageBatchRequestEntry, 0, len(tasks))
    
    for i, task := range tasks {
        body, _ := json.Marshal(task)
        
        entry := types.SendMessageBatchRequestEntry{
            Id:                     aws.String(strconv.Itoa(i)),
            MessageBody:            aws.String(string(body)),
            MessageGroupId:         aws.String(q.messageGroupID),
            MessageDeduplicationId: aws.String(q.deduplicationID(task)),
        }
        
        entries = append(entries, entry)
    }
    
    // Batch send (max 10 messages)
    for i := 0; i < len(entries); i += 10 {
        end := min(i+10, len(entries))
        batch := entries[i:end]
        
        _, err := q.client.SendMessageBatch(ctx, &sqs.SendMessageBatchInput{
            QueueUrl: &q.queueURL,
            Entries:  batch,
        })
        if err != nil {
            return fmt.Errorf("send batch %d-%d: %w", i, end, err)
        }
    }
    
    return nil
}
```

### 3. Priority Queue

```go
// PriorityQueue manages multiple SQS queues by priority
type PriorityQueue struct {
    queues    map[int]Queue  // Priority -> Queue
    mu        sync.RWMutex
}

func (p *PriorityQueue) Enqueue(ctx context.Context, task *Task) error {
    p.mu.RLock()
    queue, exists := p.queues[task.Priority]
    p.mu.RUnlock()
    
    if !exists {
        // Use default queue for unknown priorities
        queue = p.queues[0]
    }
    
    return queue.Enqueue(ctx, task)
}

func (p *PriorityQueue) Dequeue(ctx context.Context) (*Task, error) {
    // Try high priority queues first
    priorities := []int{3, 2, 1, 0} // High to low
    
    for _, priority := range priorities {
        if queue, exists := p.queues[priority]; exists {
            task, err := queue.Dequeue(ctx)
            if err == nil {
                return task, nil
            }
            if !errors.Is(err, ErrNoTasks) {
                return nil, err
            }
        }
    }
    
    return nil, ErrNoTasks
}
```

## Task Processing

### 1. Worker Pool

```go
// WorkerPool processes tasks concurrently
type WorkerPool struct {
    queue       Queue
    handler     TaskHandler
    workerCount int
    metrics     *Metrics
    wg          sync.WaitGroup
    ctx         context.Context
    cancel      context.CancelFunc
}

// TaskHandler processes tasks
type TaskHandler interface {
    Handle(ctx context.Context, task *Task) (*TaskResult, error)
}

func (p *WorkerPool) Start() {
    p.ctx, p.cancel = context.WithCancel(context.Background())
    
    for i := 0; i < p.workerCount; i++ {
        p.wg.Add(1)
        go p.worker(i)
    }
}

func (p *WorkerPool) worker(id int) {
    defer p.wg.Done()
    
    for {
        select {
        case <-p.ctx.Done():
            return
        default:
            // Get task from queue
            task, err := p.queue.Dequeue(p.ctx)
            if err != nil {
                if errors.Is(err, ErrNoTasks) {
                    time.Sleep(1 * time.Second)
                    continue
                }
                logger.Error("dequeue failed", "error", err)
                continue
            }
            
            // Process task
            p.processTask(task)
        }
    }
}

func (p *WorkerPool) processTask(task *Task) {
    ctx, span := tracer.Start(p.ctx, "queue.processTask",
        trace.WithAttributes(
            attribute.String("task.id", task.ID),
            attribute.String("task.type", task.Type),
        ),
    )
    defer span.End()
    
    start := time.Now()
    
    // Handle task with timeout
    taskCtx, cancel := context.WithTimeout(ctx, task.VisibilityTimeout)
    defer cancel()
    
    result, err := p.handler.Handle(taskCtx, task)
    duration := time.Since(start)
    
    if err != nil {
        p.handleError(ctx, task, err)
        return
    }
    
    // Mark as complete
    if err := p.queue.Complete(ctx, task); err != nil {
        logger.Error("complete task failed", "error", err)
    }
    
    // Update metrics
    p.metrics.TaskProcessed(task.Type, "success", duration)
}
```

### 2. Task Handlers

```go
// Router-based task handler
type TaskRouter struct {
    handlers map[string]TaskHandler
    mu       sync.RWMutex
}

func (r *TaskRouter) Register(taskType string, handler TaskHandler) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.handlers[taskType] = handler
}

func (r *TaskRouter) Handle(ctx context.Context, task *Task) (*TaskResult, error) {
    r.mu.RLock()
    handler, exists := r.handlers[task.Type]
    r.mu.RUnlock()
    
    if !exists {
        return nil, fmt.Errorf("no handler for task type: %s", task.Type)
    }
    
    return handler.Handle(ctx, task)
}

// Example handlers
func NewEmbeddingHandler(service EmbeddingService) TaskHandler {
    return TaskHandlerFunc(func(ctx context.Context, task *Task) (*TaskResult, error) {
        var req EmbeddingRequest
        if err := json.Unmarshal(task.Payload.([]byte), &req); err != nil {
            return nil, fmt.Errorf("unmarshal payload: %w", err)
        }
        
        embedding, err := service.GenerateEmbedding(ctx, req.Text, req.Model)
        if err != nil {
            return nil, err
        }
        
        return &TaskResult{
            TaskID: task.ID,
            Status: TaskCompleted,
            Result: embedding,
        }, nil
    })
}
```

### 3. Retry Mechanism

```go
// RetryHandler wraps handlers with retry logic
type RetryHandler struct {
    handler     TaskHandler
    maxRetries  int
    backoff     BackoffStrategy
}

func (h *RetryHandler) Handle(ctx context.Context, task *Task) (*TaskResult, error) {
    result, err := h.handler.Handle(ctx, task)
    if err == nil {
        return result, nil
    }
    
    // Check if retryable
    if !isRetryable(err) {
        return nil, err
    }
    
    // Check retry limit
    if task.RetryCount >= h.maxRetries {
        return nil, fmt.Errorf("max retries exceeded: %w", err)
    }
    
    // Calculate backoff
    delay := h.backoff.NextBackoff(task.RetryCount)
    
    // Update task for retry
    task.RetryCount++
    task.ScheduledFor = aws.Time(time.Now().Add(delay))
    
    // Return error to trigger requeue
    return nil, &RetryableError{
        Err:   err,
        Delay: delay,
    }
}

// Exponential backoff strategy
type ExponentialBackoff struct {
    BaseDelay time.Duration
    MaxDelay  time.Duration
    Factor    float64
}

func (b *ExponentialBackoff) NextBackoff(retryCount int) time.Duration {
    delay := time.Duration(float64(b.BaseDelay) * math.Pow(b.Factor, float64(retryCount)))
    if delay > b.MaxDelay {
        delay = b.MaxDelay
    }
    return delay
}
```

## Dead Letter Queue

```go
// DLQHandler handles failed tasks
type DLQHandler struct {
    dlq        Queue
    maxRetries int
}

func (h *DLQHandler) HandleFailedTask(ctx context.Context, task *Task, err error) error {
    // Add failure metadata
    if task.Metadata == nil {
        task.Metadata = make(map[string]string)
    }
    task.Metadata["failure_reason"] = err.Error()
    task.Metadata["failed_at"] = time.Now().Format(time.RFC3339)
    task.Metadata["original_queue"] = task.Metadata["queue"]
    
    // Send to DLQ
    return h.dlq.Enqueue(ctx, task)
}

// DLQ processor for manual intervention
type DLQProcessor struct {
    dlq          Queue
    store        TaskStore
    notifier     Notifier
}

func (p *DLQProcessor) ProcessDLQ(ctx context.Context) error {
    for {
        task, err := p.dlq.Dequeue(ctx)
        if err != nil {
            if errors.Is(err, ErrNoTasks) {
                time.Sleep(30 * time.Second)
                continue
            }
            return err
        }
        
        // Store for analysis
        if err := p.store.StoreFailedTask(ctx, task); err != nil {
            logger.Error("store failed task", "error", err)
        }
        
        // Notify administrators
        if err := p.notifier.NotifyFailedTask(ctx, task); err != nil {
            logger.Error("notify failed task", "error", err)
        }
        
        // Mark as processed
        p.dlq.Complete(ctx, task)
    }
}
```

## Visibility Timeout Management

```go
// VisibilityExtender extends task visibility during processing
type VisibilityExtender struct {
    queue    Queue
    interval time.Duration
}

func (e *VisibilityExtender) ExtendDuringProcessing(ctx context.Context, task *Task, process func() error) error {
    // Start extension routine
    extendCtx, cancel := context.WithCancel(ctx)
    defer cancel()
    
    errCh := make(chan error, 1)
    go func() {
        ticker := time.NewTicker(e.interval)
        defer ticker.Stop()
        
        for {
            select {
            case <-extendCtx.Done():
                return
            case <-ticker.C:
                if err := e.queue.Extend(extendCtx, task, task.VisibilityTimeout); err != nil {
                    errCh <- err
                    return
                }
            }
        }
    }()
    
    // Process task
    processErr := process()
    
    // Stop extension
    cancel()
    
    // Check for extension errors
    select {
    case err := <-errCh:
        if processErr == nil {
            return err
        }
    default:
    }
    
    return processErr
}
```

## Monitoring & Metrics

### Queue Metrics

```go
var (
    tasksEnqueued = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_queue_tasks_enqueued_total",
            Help: "Total number of tasks enqueued",
        },
        []string{"queue", "task_type"},
    )
    
    tasksDequeued = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_queue_tasks_dequeued_total",
            Help: "Total number of tasks dequeued",
        },
        []string{"queue", "task_type"},
    )
    
    tasksCompleted = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_queue_tasks_completed_total",
            Help: "Total number of tasks completed",
        },
        []string{"queue", "task_type", "status"},
    )
    
    taskProcessingDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "mcp_queue_task_processing_duration_seconds",
            Help:    "Task processing duration",
            Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
        },
        []string{"queue", "task_type"},
    )
    
    queueDepth = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_queue_depth",
            Help: "Current queue depth",
        },
        []string{"queue"},
    )
    
    dlqDepth = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_dlq_depth",
            Help: "Current DLQ depth",
        },
        []string{"queue"},
    )
)
```

### Queue Stats

```go
// QueueStats provides queue statistics
type QueueStats struct {
    QueueName           string
    ApproximateMessages int32
    InflightMessages    int32
    OldestMessageAge    time.Duration
    Attributes          map[string]string
}

func (q *SQSQueue) Stats(ctx context.Context) (*QueueStats, error) {
    result, err := q.client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
        QueueUrl: &q.queueURL,
        AttributeNames: []types.QueueAttributeName{
            types.QueueAttributeNameApproximateNumberOfMessages,
            types.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
            types.QueueAttributeNameApproximateAgeOfOldestMessage,
        },
    })
    if err != nil {
        return nil, err
    }
    
    stats := &QueueStats{
        QueueName:  q.queueURL,
        Attributes: result.Attributes,
    }
    
    if v, ok := result.Attributes["ApproximateNumberOfMessages"]; ok {
        if n, err := strconv.ParseInt(v, 10, 32); err == nil {
            stats.ApproximateMessages = int32(n)
        }
    }
    
    // Update metrics
    queueDepth.WithLabelValues(q.queueURL).Set(float64(stats.ApproximateMessages))
    
    return stats, nil
}
```

## Configuration

### Environment Variables

```bash
# SQS Configuration
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test
SQS_DLQ_URL=https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test-dlq
SQS_VISIBILITY_TIMEOUT=300  # 5 minutes
SQS_MAX_RECEIVE_COUNT=3
SQS_WAIT_TIME_SECONDS=20    # Long polling

# Worker Configuration
QUEUE_WORKER_COUNT=10
QUEUE_BATCH_SIZE=10
QUEUE_PROCESSING_TIMEOUT=5m

# Retry Configuration
QUEUE_MAX_RETRIES=3
QUEUE_RETRY_BASE_DELAY=1s
QUEUE_RETRY_MAX_DELAY=5m
```

### Configuration File

```yaml
queue:
  sqs:
    region: us-east-1
    queues:
      - name: tasks
        url: ${SQS_QUEUE_URL}
        type: standard
        visibility_timeout: 300
        max_receive_count: 3
        
      - name: priority-tasks
        url: ${SQS_PRIORITY_QUEUE_URL}
        type: fifo
        message_group_id: "default"
        
      - name: dlq
        url: ${SQS_DLQ_URL}
        type: standard
        
  workers:
    count: 10
    batch_size: 10
    processing_timeout: 5m
    
  retry:
    max_attempts: 3
    backoff:
      type: exponential
      base_delay: 1s
      max_delay: 5m
      factor: 2.0
```

## Task Types

### Common Task Types

```go
// Embedding generation task
type EmbeddingTask struct {
    Text     string `json:"text"`
    Model    string `json:"model"`
    AgentID  string `json:"agent_id"`
}

// Document processing task
type DocumentTask struct {
    DocumentID string `json:"document_id"`
    Operation  string `json:"operation"`
    Parameters map[string]interface{} `json:"parameters"`
}

// Webhook delivery task
type WebhookTask struct {
    URL      string          `json:"url"`
    Method   string          `json:"method"`
    Headers  map[string]string `json:"headers"`
    Body     json.RawMessage `json:"body"`
    Retries  int             `json:"retries"`
}

// Collaboration task
type CollaborationTask struct {
    SessionID    string   `json:"session_id"`
    Participants []string `json:"participants"`
    Strategy     string   `json:"strategy"`
    Payload      interface{} `json:"payload"`
}
```

## Error Handling

```go
// Queue errors
var (
    ErrNoTasks          = errors.New("no tasks available")
    ErrTaskExpired      = errors.New("task expired")
    ErrQueueFull        = errors.New("queue is full")
    ErrInvalidTask      = errors.New("invalid task")
    ErrVisibilityTimeout = errors.New("visibility timeout expired")
)

// RetryableError indicates task should be retried
type RetryableError struct {
    Err   error
    Delay time.Duration
}

func (e *RetryableError) Error() string {
    return fmt.Sprintf("retryable error (delay=%s): %v", e.Delay, e.Err)
}

// Check if error is retryable
func isRetryable(err error) bool {
    // AWS SDK errors
    var awsErr smithy.APIError
    if errors.As(err, &awsErr) {
        switch awsErr.ErrorCode() {
        case "ServiceUnavailable", "RequestLimitExceeded", "ThrottlingException":
            return true
        }
    }
    
    // Custom retryable errors
    var retryErr *RetryableError
    if errors.As(err, &retryErr) {
        return true
    }
    
    // Network errors
    var netErr net.Error
    if errors.As(err, &netErr) && netErr.Temporary() {
        return true
    }
    
    return false
}
```

## Testing

```go
// Mock queue for testing
type MockQueue struct {
    mu      sync.Mutex
    tasks   []*Task
    dequeued map[string]*Task
}

func (m *MockQueue) Enqueue(ctx context.Context, task *Task) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    task.ID = uuid.New().String()
    m.tasks = append(m.tasks, task)
    return nil
}

func (m *MockQueue) Dequeue(ctx context.Context) (*Task, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if len(m.tasks) == 0 {
        return nil, ErrNoTasks
    }
    
    task := m.tasks[0]
    m.tasks = m.tasks[1:]
    m.dequeued[task.ID] = task
    
    return task, nil
}

// Integration test with real SQS
func TestSQSIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    ctx := context.Background()
    cfg, _ := config.LoadDefaultConfig(ctx)
    client := sqs.NewFromConfig(cfg)
    
    queue := &SQSQueue{
        client:   client,
        queueURL: os.Getenv("TEST_SQS_QUEUE_URL"),
    }
    
    // Test enqueue
    task := &Task{
        Type:    "test",
        Payload: map[string]string{"test": "data"},
    }
    
    err := queue.Enqueue(ctx, task)
    assert.NoError(t, err)
    
    // Test dequeue
    received, err := queue.Dequeue(ctx)
    assert.NoError(t, err)
    assert.Equal(t, task.Type, received.Type)
    
    // Test complete
    err = queue.Complete(ctx, received)
    assert.NoError(t, err)
}
```

## Best Practices

1. **Message Size**: Keep messages under 256KB for SQS
2. **Visibility Timeout**: Set appropriately for task processing time
3. **Idempotency**: Ensure tasks can be safely processed multiple times
4. **Error Handling**: Use DLQ for persistent failures
5. **Monitoring**: Track queue depth and processing times
6. **Batching**: Use batch operations for efficiency
7. **Long Polling**: Enable to reduce API calls
8. **FIFO Queues**: Use for strict ordering requirements

---

Package Version: 1.0.0
Last Updated: 2024-01-10