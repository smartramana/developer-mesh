package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/queue"
)

type mockSQS struct {
	recvFunc    func(context.Context, int32, int32) ([]queue.SQSEvent, []string, error)
	deleteFunc  func(context.Context, string) error
}

func (m *mockSQS) ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]queue.SQSEvent, []string, error) {
	return m.recvFunc(ctx, maxMessages, waitSeconds)
}
func (m *mockSQS) DeleteMessage(ctx context.Context, receiptHandle string) error {
	return m.deleteFunc(ctx, receiptHandle)
}

type mockRedis struct {
	existsFunc func(context.Context, string) (int64, error)
	setFunc    func(context.Context, string, string, time.Duration) error
}

func (m *mockRedis) Exists(ctx context.Context, key string) (int64, error) {
	return m.existsFunc(ctx, key)
}
func (m *mockRedis) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return m.setFunc(ctx, key, value, ttl)
}

func TestRun_Success(t *testing.T) {
	sqs := &mockSQS{
		recvFunc: func(ctx context.Context, max, wait int32) ([]queue.SQSEvent, []string, error) {
			return nil, nil, errors.New("stop") // simulate one poll then exit
		},
		deleteFunc: func(ctx context.Context, h string) error { return nil },
	}
	redis := &mockRedis{
		existsFunc: func(ctx context.Context, key string) (int64, error) { return 0, nil },
		setFunc: func(ctx context.Context, key, val string, ttl time.Duration) error { return nil },
	}
	processFunc := func(ev queue.SQSEvent) error { return nil }
	err := run(context.Background(), sqs, redis, processFunc)
	if err == nil || err.Error() != "stop" {
		t.Errorf("Expected stop error, got %v", err)
	}
}

func TestRun_ErrorPropagation(t *testing.T) {
	sqs := &mockSQS{
		recvFunc: func(ctx context.Context, max, wait int32) ([]queue.SQSEvent, []string, error) {
			return nil, nil, errors.New("fail")
		},
		deleteFunc: func(ctx context.Context, h string) error { return nil },
	}
	redis := &mockRedis{
		existsFunc: func(ctx context.Context, key string) (int64, error) { return 0, nil },
		setFunc: func(ctx context.Context, key, val string, ttl time.Duration) error { return nil },
	}
	processFunc := func(ev queue.SQSEvent) error { return nil }
	err := run(context.Background(), sqs, redis, processFunc)
	if err == nil || err.Error() != "fail" {
		t.Errorf("Expected fail error, got %v", err)
	}
}
