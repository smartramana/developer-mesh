package worker

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/queue"
)

func TestProcessSQSEvent_Success(t *testing.T) {
	event := queue.SQSEvent{
		DeliveryID: "123",
		EventType:  "pull_request",
		RepoName:   "repo",
		SenderName: "sender",
		Payload:    json.RawMessage(`{"action": "opened", "pull_request": {"number": 42, "title": "Test PR", "state": "open", "user": {"login": "test-user"}}}`),
	}
	err := ProcessSQSEvent(event)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
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
		Payload:    json.RawMessage(`{"ref": "refs/heads/main", "head_commit": {"id": "abc123", "message": "test commit", "author": {"name": "test author"}}}`),
	}
	err := ProcessSQSEvent(event)
	if err != nil {
		t.Errorf("Expected success for valid push event, got error: %v", err)
	}
}

func TestKeys(t *testing.T) {
	m := map[string]interface{}{"a": 1, "b": 2}
	k := keys(m)
	if len(k) != 2 || (k[0] != "a" && k[0] != "b") {
		t.Errorf("Expected keys [a b], got %v", k)
	}
}
