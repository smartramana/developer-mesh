package queue

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SQSReceiverDeleter defines the low-level AWS SQS operations we need
type SQSReceiverDeleter interface {
	SendMessage(ctx context.Context, input *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, input *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, input *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// SQSAdapter interface represents the high-level operations needed by the worker
type SQSAdapter interface {
	// EnqueueEvent sends a message to SQS - matches the SQSClient interface
	EnqueueEvent(ctx context.Context, event SQSEvent) error
	// ReceiveEvents receives messages from SQS - matches the SQSClient interface
	ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]SQSEvent, []string, error)
	// DeleteMessage deletes a message from SQS - matches the SQSClient interface
	DeleteMessage(ctx context.Context, receiptHandle string) error
}

// SQSClientAdapter adapts between production SQS client, LocalStack, and mock implementations
// It matches the SQSClient interface to ensure compatibility with existing code
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

// LoadSQSConfigFromEnv loads SQS configuration from environment variables
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

// getEnvWithDefault gets an environment variable or returns the default value if not set
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// NewSQSClientAdapter creates a new SQS client adapter using configuration
func NewSQSClientAdapter(ctx context.Context, config *SQSAdapterConfig) (*SQSClientAdapter, error) {
	log.Printf("Initializing SQS client adapter with config: use_localstack=%v, mock_mode=%v",
		config.UseLocalStack, config.MockMode)

	if config.MockMode {
		return NewMockSQSClient(), nil
	}

	var sqsClient SQSReceiverDeleter
	var err error

	if config.UseLocalStack {
		sqsClient, err = createLocalStackSQSClient(ctx, config)
	} else {
		sqsClient, err = createProductionSQSClient(ctx, config)
	}

	if err != nil {
		return nil, err
	}

	// Determine the queue URL
	queueURL := config.QueueURL
	if queueURL == "" && config.UseLocalStack {
		// Try to construct a LocalStack queue URL if not provided
		queueURL = constructLocalStackQueueURL(config)
	}

	return &SQSClientAdapter{
		client:   sqsClient,
		queueURL: queueURL,
		config:   config,
	}, nil
}

// createLocalStackSQSClient creates an SQS client configured for LocalStack
func createLocalStackSQSClient(ctx context.Context, config *SQSAdapterConfig) (*sqs.Client, error) {
	log.Printf("Creating LocalStack SQS client with endpoint: %s", config.Endpoint)

	// Custom HTTP client to allow insecure connections for LocalStack
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	customHTTPClient := &http.Client{
		Transport: customTransport,
		Timeout:   30 * time.Second,
	}

	// Create custom resolver for LocalStack endpoint
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               config.Endpoint,
			HostnameImmutable: true,
			SigningRegion:     config.Region,
		}, nil
	})

	// Create AWS config
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithHTTPClient(customHTTPClient),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.AccessKey, config.SecretKey, "")),
		awsconfig.WithRegion(config.Region),
	)
	if err != nil {
		log.Printf("Failed to create LocalStack AWS config: %v", err)
		return nil, err
	}

	// Create SQS client
	sqsClient := sqs.NewFromConfig(cfg)
	return sqsClient, nil
}

// createProductionSQSClient creates an SQS client for production AWS
func createProductionSQSClient(ctx context.Context, config *SQSAdapterConfig) (*sqs.Client, error) {
	log.Printf("Creating production SQS client for region: %s", config.Region)

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(config.Region))
	if err != nil {
		log.Printf("Failed to create production AWS config: %v", err)
		return nil, err
	}

	return sqs.NewFromConfig(cfg), nil
}

// constructLocalStackQueueURL constructs a queue URL for LocalStack based on configuration
func constructLocalStackQueueURL(config *SQSAdapterConfig) string {
	if config.QueueName == "" {
		log.Println("Warning: Cannot construct LocalStack queue URL without queue name")
		return ""
	}

	// Parse the endpoint
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = "http://localhost:4566"
	}

	// Strip trailing slash if present
	endpoint = strings.TrimSuffix(endpoint, "/")

	// Extract hostname and port
	var hostname, port string
	if strings.HasPrefix(endpoint, "http://") {
		hostPort := strings.TrimPrefix(endpoint, "http://")
		parts := strings.Split(hostPort, ":")
		hostname = parts[0]
		if len(parts) > 1 {
			port = parts[1]
		} else {
			port = "4566" // Default LocalStack port
		}
	} else if strings.HasPrefix(endpoint, "https://") {
		hostPort := strings.TrimPrefix(endpoint, "https://")
		parts := strings.Split(hostPort, ":")
		hostname = parts[0]
		if len(parts) > 1 {
			port = parts[1]
		} else {
			port = "4566" // Default LocalStack port
		}
	} else {
		// Assume it's just a hostname
		parts := strings.Split(endpoint, ":")
		hostname = parts[0]
		if len(parts) > 1 {
			port = parts[1]
		} else {
			port = "4566" // Default LocalStack port
		}
	}

	// Format: http://hostname:port/000000000000/queue-name
	queueURL := fmt.Sprintf("http://%s:%s/000000000000/%s", hostname, port, config.QueueName)
	log.Printf("Constructed LocalStack queue URL: %s", queueURL)
	return queueURL
}

// MockSQSClient creates a mock SQS client adapter for testing
func NewMockSQSClient() *SQSClientAdapter {
	log.Println("Creating mock SQS client")
	mockClient := &MockSQSClient{
		messages: []*types.Message{},
	}

	return &SQSClientAdapter{
		client:   mockClient,
		queueURL: "mock-queue-url",
		config:   &SQSAdapterConfig{MockMode: true},
	}
}

// MockSQSClient is a mock implementation of the SQS client for development
type MockSQSClient struct {
	messages []*types.Message
}

// SendMessage mock implementation
func (m *MockSQSClient) SendMessage(ctx context.Context, input *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	log.Printf("[MOCK] Sending message to SQS: %s", *input.MessageBody)

	// Generate a random message ID
	messageId := fmt.Sprintf("mock-msg-%d", time.Now().UnixNano())

	// Store the message for later retrieval
	m.messages = append(m.messages, &types.Message{
		Body:          input.MessageBody,
		MessageId:     aws.String(messageId),
		ReceiptHandle: aws.String("receipt-" + messageId),
	})

	return &sqs.SendMessageOutput{
		MessageId: aws.String(messageId),
	}, nil
}

// ReceiveMessage mock implementation
func (m *MockSQSClient) ReceiveMessage(ctx context.Context, input *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	log.Printf("[MOCK] Receiving messages from SQS (max: %d)", input.MaxNumberOfMessages)

	// Simulate waiting for messages
	if len(m.messages) == 0 && input.WaitTimeSeconds > 0 {
		select {
		case <-ctx.Done():
			return &sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil
		case <-time.After(time.Duration(input.WaitTimeSeconds) * time.Second):
			// Continue after waiting
		}
	}

	// Return available messages up to MaxNumberOfMessages
	var messagesToReturn []types.Message
	maxMessages := int(input.MaxNumberOfMessages)
	if maxMessages > len(m.messages) {
		maxMessages = len(m.messages)
	}

	for i := 0; i < maxMessages; i++ {
		messagesToReturn = append(messagesToReturn, *m.messages[i])
	}

	// Remove returned messages
	if maxMessages > 0 {
		m.messages = m.messages[maxMessages:]
	}

	return &sqs.ReceiveMessageOutput{
		Messages: messagesToReturn,
	}, nil
}

// DeleteMessage mock implementation
func (m *MockSQSClient) DeleteMessage(ctx context.Context, input *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	log.Printf("[MOCK] Deleting message with receipt handle: %s", *input.ReceiptHandle)
	return &sqs.DeleteMessageOutput{}, nil
}

// Ensure SQSClientAdapter implements SQSAdapter
var _ SQSAdapter = (*SQSClientAdapter)(nil)

// EnqueueEvent sends a webhook event to SQS
func (a *SQSClientAdapter) EnqueueEvent(ctx context.Context, event SQSEvent) error {
	log.Printf("Adapter: Enqueueing event: %s (type: %s)", event.DeliveryID, event.EventType)

	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("Adapter: Failed to marshal event: %v", err)
		return err
	}

	_, err = a.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(a.queueURL),
		MessageBody: aws.String(string(body)),
	})

	if err != nil {
		log.Printf("Adapter: Failed to send message to SQS: %v", err)
		return err
	}

	log.Printf("Adapter: Successfully enqueued event: %s", event.DeliveryID)
	return nil
}

// ReceiveEvents receives, parses and returns webhook events from SQS
func (a *SQSClientAdapter) ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]SQSEvent, []string, error) {
	log.Printf("Adapter: Receiving messages (max: %d, wait: %ds)", maxMessages, waitSeconds)

	resp, err := a.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(a.queueURL),
		MaxNumberOfMessages: maxMessages,
		WaitTimeSeconds:     waitSeconds,
	})

	if err != nil {
		log.Printf("Adapter: Failed to receive messages: %v", err)
		return nil, nil, err
	}

	log.Printf("Adapter: Received %d messages", len(resp.Messages))

	var events []SQSEvent
	var receiptHandles []string

	for _, msg := range resp.Messages {
		var event SQSEvent
		if err := json.Unmarshal([]byte(*msg.Body), &event); err == nil {
			log.Printf("Adapter: Successfully unmarshaled event: %s", event.DeliveryID)
			events = append(events, event)
			receiptHandles = append(receiptHandles, *msg.ReceiptHandle)
		} else {
			log.Printf("Adapter: Failed to unmarshal message: %v", err)
		}
	}

	return events, receiptHandles, nil
}

// DeleteMessage deletes a message from the SQS queue
func (a *SQSClientAdapter) DeleteMessage(ctx context.Context, receiptHandle string) error {
	if len(receiptHandle) > 10 {
		log.Printf("Adapter: Deleting message with receipt handle: %s...", receiptHandle[:10])
	} else {
		log.Printf("Adapter: Deleting message with receipt handle: %s", receiptHandle)
	}

	_, err := a.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(a.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})

	if err != nil {
		log.Printf("Adapter: Failed to delete message: %v", err)
		return err
	}

	log.Printf("Adapter: Successfully deleted message")
	return nil
}
