package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type mockSQSAPI struct {
	sendMessageFunc    func(ctx context.Context, input *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	receiveMessageFunc func(ctx context.Context, input *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	deleteMessageFunc  func(ctx context.Context, input *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

func (m *mockSQSAPI) SendMessage(ctx context.Context, input *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	return m.sendMessageFunc(ctx, input, optFns...)
}
func (m *mockSQSAPI) ReceiveMessage(ctx context.Context, input *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	return m.receiveMessageFunc(ctx, input, optFns...)
}
func (m *mockSQSAPI) DeleteMessage(ctx context.Context, input *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	return m.deleteMessageFunc(ctx, input, optFns...)
}

// Patch SQSClient for testing
func newTestSQSClient(mock SQSAPI) *SQSClient {
	return NewSQSClientWithAPI(mock, "test-queue-url")
}

func TestEnqueueEvent(t *testing.T) {
	called := false
	mock := &mockSQSAPI{
		sendMessageFunc: func(ctx context.Context, input *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
			called = true
			if input.QueueUrl == nil || *input.QueueUrl != "test-queue-url" {
				t.Errorf("QueueUrl not set correctly")
			}
			return &sqs.SendMessageOutput{}, nil
		},
	}
	client := newTestSQSClient(mock)
	event := SQSEvent{DeliveryID: "id", EventType: "type", RepoName: "repo", SenderName: "sender", Payload: json.RawMessage(`{"foo":"bar"}`)}
	err := client.EnqueueEvent(context.Background(), event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !called {
		t.Error("SendMessage was not called")
	}
}

func TestReceiveEvents(t *testing.T) {
	mock := &mockSQSAPI{
		receiveMessageFunc: func(ctx context.Context, input *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
			msg := SQSEvent{DeliveryID: "id", EventType: "type", RepoName: "repo", SenderName: "sender", Payload: json.RawMessage(`{"foo":"bar"}`)}
			body, _ := json.Marshal(msg)
			return &sqs.ReceiveMessageOutput{
				Messages: []types.Message{{
					Body:          awsString(string(body)),
					ReceiptHandle: awsString("handle1"),
				}},
			}, nil
		},
	}
	client := newTestSQSClient(mock)
	events, handles, err := client.ReceiveEvents(context.Background(), 1, 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(events) != 1 || len(handles) != 1 {
		t.Errorf("Expected 1 event and 1 handle, got %d, %d", len(events), len(handles))
	}
}

func TestReceiveEvents_Error(t *testing.T) {
	mock := &mockSQSAPI{
		receiveMessageFunc: func(ctx context.Context, input *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
			return nil, errors.New("fail")
		},
	}
	client := newTestSQSClient(mock)
	_, _, err := client.ReceiveEvents(context.Background(), 1, 1)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestDeleteMessage(t *testing.T) {
	called := false
	mock := &mockSQSAPI{
		deleteMessageFunc: func(ctx context.Context, input *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
			called = true
			if input.QueueUrl == nil || *input.QueueUrl != "test-queue-url" {
				t.Errorf("QueueUrl not set correctly")
			}
			if input.ReceiptHandle == nil || *input.ReceiptHandle != "handle1" {
				t.Errorf("ReceiptHandle not set correctly")
			}
			return &sqs.DeleteMessageOutput{}, nil
		},
	}
	client := newTestSQSClient(mock)
	err := client.DeleteMessage(context.Background(), "handle1")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !called {
		t.Error("DeleteMessage was not called")
	}
}

func TestDeleteMessage_Error(t *testing.T) {
	mock := &mockSQSAPI{
		deleteMessageFunc: func(ctx context.Context, input *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
			return nil, errors.New("fail")
		},
	}
	client := newTestSQSClient(mock)
	err := client.DeleteMessage(context.Background(), "handle1")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func awsString(s string) *string { return &s }
