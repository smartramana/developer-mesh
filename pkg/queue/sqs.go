package queue

import (
	"context"
	"encoding/json"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSEvent represents an event in the SQS queue with auth context
type SQSEvent struct {
	DeliveryID string          `json:"delivery_id"`
	EventType  string          `json:"event_type"`
	RepoName   string          `json:"repo_name"`
	SenderName string          `json:"sender_name"`
	Payload    json.RawMessage `json:"payload"`
	// Auth context for the event
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

type SQSAPI interface {
	SendMessage(ctx context.Context, input *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, input *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, input *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

type SQSClient struct {
	Client   SQSAPI
	QueueURL string
}

func NewSQSClient(ctx context.Context) (*SQSClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	client := sqs.NewFromConfig(cfg)
	queueURL := os.Getenv("SQS_QUEUE_URL")
	return &SQSClient{Client: client, QueueURL: queueURL}, nil
}

// NewSQSClientWithAPI allows injecting a custom SQSAPI (for testing)
func NewSQSClientWithAPI(api SQSAPI, queueURL string) *SQSClient {
	return &SQSClient{Client: api, QueueURL: queueURL}
}

func (q *SQSClient) EnqueueEvent(ctx context.Context, event SQSEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = q.Client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(q.QueueURL),
		MessageBody: aws.String(string(body)),
	})
	return err
}

func (q *SQSClient) ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]SQSEvent, []string, error) {
	resp, err := q.Client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.QueueURL),
		MaxNumberOfMessages: maxMessages,
		WaitTimeSeconds:     waitSeconds,
	})
	if err != nil {
		return nil, nil, err
	}
	var events []SQSEvent
	var receiptHandles []string
	for _, msg := range resp.Messages {
		var event SQSEvent
		if err := json.Unmarshal([]byte(*msg.Body), &event); err == nil {
			events = append(events, event)
			receiptHandles = append(receiptHandles, *msg.ReceiptHandle)
		}
	}
	return events, receiptHandles, nil
}

func (q *SQSClient) DeleteMessage(ctx context.Context, receiptHandle string) error {
	_, err := q.Client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.QueueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	return err
}
