package worker_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/queue"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerSQSIntegration(t *testing.T) {
	// Skip if not in integration test mode
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	require.NoError(t, err)

	// Create SQS client
	sqsClient := sqs.NewFromConfig(cfg)
	queueURL := os.Getenv("SQS_QUEUE_URL")
	require.NotEmpty(t, queueURL, "SQS_QUEUE_URL must be set")

	// Create test event
	testEvent := queue.SQSEvent{
		DeliveryID: "functional-test-" + time.Now().Format("20060102-150405"),
		EventType:  "functional_test",
		RepoName:   "devops-mcp",
		SenderName: "functional-test-suite",
		Payload:    json.RawMessage(`{"test": true, "timestamp": "` + time.Now().UTC().Format(time.RFC3339) + `"}`),
		AuthContext: &queue.EventAuthContext{
			TenantID:      "test-tenant",
			PrincipalID:   "test-principal",
			PrincipalType: "api_key",
			Permissions:   []string{"read", "write"},
		},
	}

	// Marshal event to JSON
	eventJSON, err := json.Marshal(testEvent)
	require.NoError(t, err)

	// Send message to SQS
	_, err = sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    &queueURL,
		MessageBody: aws.String(string(eventJSON)),
	})
	require.NoError(t, err)

	t.Logf("Successfully sent test message to SQS queue: %s", testEvent.DeliveryID)

	// Note: In a real test, you would:
	// 1. Start the worker
	// 2. Verify the message was processed
	// 3. Check any side effects (database updates, etc.)
	// For now, we're just testing that we can send to the IP-restricted queue
}

func TestSQSQueueSecurity(t *testing.T) {
	// Skip if not in integration test mode
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	require.NoError(t, err)

	// Create SQS client
	sqsClient := sqs.NewFromConfig(cfg)
	queueURL := os.Getenv("SQS_QUEUE_URL")
	require.NotEmpty(t, queueURL, "SQS_QUEUE_URL must be set")

	// Get queue attributes
	attrs, err := sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       &queueURL,
		AttributeNames: []types.QueueAttributeName{"All"},
	})
	require.NoError(t, err)

	// Verify encryption is enabled
	sseEnabled, exists := attrs.Attributes["SqsManagedSseEnabled"]
	assert.True(t, exists, "SqsManagedSseEnabled attribute should exist")
	assert.Equal(t, "true", sseEnabled, "Server-side encryption should be enabled")

	// Verify policy exists (IP restriction)
	policy, exists := attrs.Attributes["Policy"]
	assert.True(t, exists, "Queue policy should exist")
	assert.Contains(t, policy, "IpAddress", "Policy should contain IP address condition")
	assert.Contains(t, policy, "aws:SourceIp", "Policy should restrict by source IP")

	t.Log("SQS queue security configuration verified successfully")
}
