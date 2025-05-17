package queue

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSEvent represents a webhook event from SQS
type SQSEvent struct {
	DeliveryID string          `json:"delivery_id"`
	EventType  string          `json:"event_type"`
	RepoName   string          `json:"repo_name"`
	SenderName string          `json:"sender_name"`
	Payload    json.RawMessage `json:"payload"`
}

// SQSAPI interface for testing purposes
type SQSAPI interface {
	SendMessage(ctx context.Context, input *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, input *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, input *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// SQSClient handles SQS operations
type SQSClient struct {
	Client   SQSAPI
	QueueURL string
}

// NewSQSClient creates a new SQS client
func NewSQSClient(ctx context.Context) (*SQSClient, error) {
	log.Println("Initializing SQS client")
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Printf("Failed to load AWS config: %v", err)
		return nil, err
	}
	client := sqs.NewFromConfig(cfg)
	queueURL := os.Getenv("SQS_QUEUE_URL")
	if queueURL == "" {
		log.Println("Warning: SQS_QUEUE_URL is not set")
	}
	log.Printf("SQS client initialized with queue URL: %s", queueURL)
	return &SQSClient{Client: client, QueueURL: queueURL}, nil
}

// NewSQSClientWithAPI allows injecting a custom SQSAPI (for testing)
func NewSQSClientWithAPI(api SQSAPI, queueURL string) *SQSClient {
	return &SQSClient{Client: api, QueueURL: queueURL}
}

// EnqueueEvent sends a message to SQS
func (q *SQSClient) EnqueueEvent(ctx context.Context, event SQSEvent) error {
	log.Printf("Enqueueing event: %s (type: %s)", event.DeliveryID, event.EventType)
	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
		return err
	}
	_, err = q.Client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(q.QueueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		log.Printf("Failed to send message to SQS: %v", err)
		return err
	}
	log.Printf("Successfully enqueued event: %s", event.DeliveryID)
	return nil
}

// ReceiveEvents receives messages from SQS
func (q *SQSClient) ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]SQSEvent, []string, error) {
	log.Printf("Receiving messages (max: %d, wait: %ds)", maxMessages, waitSeconds)
	resp, err := q.Client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.QueueURL),
		MaxNumberOfMessages: maxMessages,
		WaitTimeSeconds:     waitSeconds,
	})
	if err != nil {
		log.Printf("Failed to receive messages: %v", err)
		return nil, nil, err
	}
	log.Printf("Received %d messages", len(resp.Messages))
	var events []SQSEvent
	var receiptHandles []string
	for _, msg := range resp.Messages {
		var event SQSEvent
		if err := json.Unmarshal([]byte(*msg.Body), &event); err == nil {
			log.Printf("Successfully unmarshaled event: %s", event.DeliveryID)
			events = append(events, event)
			receiptHandles = append(receiptHandles, *msg.ReceiptHandle)
		} else {
			log.Printf("Failed to unmarshal message: %v", err)
		}
	}
	return events, receiptHandles, nil
}

// DeleteMessage deletes a message from SQS
func (q *SQSClient) DeleteMessage(ctx context.Context, receiptHandle string) error {
	if len(receiptHandle) > 10 {
		log.Printf("Deleting message with receipt handle: %s...", receiptHandle[:10])
	} else {
		log.Printf("Deleting message with receipt handle: %s", receiptHandle)
	}
	_, err := q.Client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.QueueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		log.Printf("Failed to delete message: %v", err)
		return err
	}
	log.Printf("Successfully deleted message")
	return nil
}
