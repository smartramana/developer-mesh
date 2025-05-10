package worker

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/queue"
)

func TestProcessSQSEvent_Success(t *testing.T) {
	event := queue.SQSEvent{
		DeliveryID: "123",
		EventType:  "pull_request",
		RepoName:   "repo",
		SenderName: "sender",
		Payload:    json.RawMessage(`{"foo": "bar"}`),
	}
	start := time.Now()
	err := ProcessSQSEvent(event)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if time.Since(start) < 200*time.Millisecond {
		t.Error("Expected simulated processing delay of at least 200ms")
	}
}

func TestProcessSQSEvent_UnmarshalFail(t *testing.T) {
	event := queue.SQSEvent{
		DeliveryID: "124",
		EventType:  "pull_request",
		RepoName:   "repo",
		SenderName: "sender",
		Payload:    json.RawMessage(`not-json`),
	}
	err := ProcessSQSEvent(event)
	if err == nil || !errors.Is(err, err) {
		t.Error("Expected error on bad JSON payload")
	}
}

func TestProcessSQSEvent_PushEvent(t *testing.T) {
	event := queue.SQSEvent{
		DeliveryID: "125",
		EventType:  "push",
		RepoName:   "repo",
		SenderName: "sender",
		Payload:    json.RawMessage(`{"foo": "bar"}`),
	}
	err := ProcessSQSEvent(event)
	if err == nil || err.Error() != "simulated failure for push event" {
		t.Errorf("Expected simulated push failure, got: %v", err)
	}
}

func TestKeys(t *testing.T) {
	m := map[string]interface{}{"a": 1, "b": 2}
	k := keys(m)
	if len(k) != 2 || (k[0] != "a" && k[0] != "b") {
		t.Errorf("Expected keys [a b], got %v", k)
	}
}
