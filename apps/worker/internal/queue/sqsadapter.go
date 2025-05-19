package queue

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SQSAdapter interface represents the high-level operations needed by the worker
type SQSAdapter interface {
	// EnqueueEvent sends a message to SQS - matches the SQSClient interface
	EnqueueEvent(ctx context.Context, event SQSEvent) error 
	// ReceiveEvents receives messages from SQS - matches the SQSClient interface
	ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]SQSEvent, []string, error)
	// DeleteMessage deletes a message from SQS - matches the SQSClient interface
	DeleteMessage(ctx context.Context, receiptHandle string) error
}

// SQSReceiverDeleter defines the SQS operations we need for the worker
type SQSReceiverDeleter interface {
	SendMessage(ctx context.Context, input *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, input *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, input *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
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

	// Custom endpoint resolver for LocalStack
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               config.Endpoint,
			HostnameImmutable: true,
			SigningRegion:     config.Region,
		}, nil
	})

	// Create a custom HTTP client with disabled SSL verification
	// This is important for working with LocalStack
	customHTTPClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Load AWS SDK configuration with LocalStack settings
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(config.Region),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.AccessKey, config.SecretKey, "", // Access key, secret key, session token
		)),
		awsconfig.WithHTTPClient(customHTTPClient),
	)
	if err != nil {
		log.Printf("Failed to load AWS config for LocalStack: %v", err)
		return nil, err
	}

	return sqs.NewFromConfig(cfg), nil
}

// createProductionSQSClient creates an SQS client for production AWS
func createProductionSQSClient(ctx context.Context, config *SQSAdapterConfig) (*sqs.Client, error) {
	log.Println("Creating production AWS SQS client")
	
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(config.Region))
	if err != nil {
		log.Printf("Failed to load AWS config: %v", err)
		return nil, err
	}

	return sqs.NewFromConfig(cfg), nil
}

// constructLocalStackQueueURL constructs a queue URL for LocalStack based on configuration
func constructLocalStackQueueURL(config *SQSAdapterConfig) string {
	// Log the parameters we're using to construct the URL
	log.Printf("Constructing LocalStack queue URL with endpoint: %s, region: %s, queue name: %s", 
		config.Endpoint, config.Region, config.QueueName)
	
	// Check if we're using the localstack:4566 format
	endpoint := strings.TrimRight(config.Endpoint, "/")
	var queueURL string
	
	// Use the new LocalStack URL format which includes the service and region
	// Format: http://sqs.{region}.localhost.localstack.cloud:4566/000000000000/queue_name
	if strings.Contains(endpoint, "localstack:4566") {
		// Extract just the protocol and port
		protocol := "http"
		port := "4566"
		if strings.HasPrefix(endpoint, "https") {
			protocol = "https"
		}
		
		// Construct the proper SQS endpoint URL with region
		queueURL = fmt.Sprintf("%s://sqs.%s.localhost.localstack.cloud:%s/000000000000/%s", 
			protocol, config.Region, port, config.QueueName)
	} else {
		// Fall back to the traditional format if not using standard localstack:4566
		queueURL = fmt.Sprintf("%s/000000000000/%s", endpoint, config.QueueName)
	}
	
	// Log the constructed URL
	log.Printf("Constructed LocalStack queue URL: %s", queueURL)
	
	return queueURL
}

// MockSQSClient is a mock implementation of the SQS client for development
type MockSQSClient struct {
	messages []*types.Message
}

// NewMockSQSClient creates a new mock SQS client
func NewMockSQSClient() *SQSClientAdapter {
	log.Println("Creating mock SQS client for development")
	return &SQSClientAdapter{
		client:   &MockSQSClient{messages: []*types.Message{}},
		queueURL: "mock://queue/tasks",
		config:   &SQSAdapterConfig{MockMode: true},
	}
}

// SendMessage mock implementation
func (m *MockSQSClient) SendMessage(ctx context.Context, input *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	log.Printf("[MOCK] Sending message to SQS: %s", *input.MessageBody)
	messageId := "mock-msg-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	
	m.messages = append(m.messages, &types.Message{
		Body:          input.MessageBody,
		MessageId:     aws.String(messageId),
		ReceiptHandle: aws.String("mock-receipt-" + messageId),
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
