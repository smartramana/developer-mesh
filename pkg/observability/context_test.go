package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextOperations(t *testing.T) {
	t.Run("correlation ID operations", func(t *testing.T) {
		ctx := context.Background()
		correlationID := "test-correlation-123"

		// Test setting and getting
		ctx = WithCorrelationID(ctx, correlationID)
		assert.Equal(t, correlationID, GetCorrelationID(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetCorrelationID(emptyCtx))
	})

	t.Run("causation ID operations", func(t *testing.T) {
		ctx := context.Background()
		causationID := "test-causation-456"

		// Test setting and getting
		ctx = WithCausationID(ctx, causationID)
		assert.Equal(t, causationID, GetCausationID(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetCausationID(emptyCtx))
	})

	t.Run("tenant ID operations", func(t *testing.T) {
		ctx := context.Background()
		tenantID := "tenant-789"

		// Test setting and getting
		ctx = WithTenantID(ctx, tenantID)
		assert.Equal(t, tenantID, GetTenantID(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetTenantID(emptyCtx))
	})

	t.Run("user ID operations", func(t *testing.T) {
		ctx := context.Background()
		userID := "user-abc"

		// Test setting and getting
		ctx = WithUserID(ctx, userID)
		assert.Equal(t, userID, GetUserID(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetUserID(emptyCtx))
	})

	t.Run("request ID operations", func(t *testing.T) {
		ctx := context.Background()
		requestID := "req-def"

		// Test setting and getting
		ctx = WithRequestID(ctx, requestID)
		assert.Equal(t, requestID, GetRequestID(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetRequestID(emptyCtx))
	})

	t.Run("operation operations", func(t *testing.T) {
		ctx := context.Background()
		operation := "CreateWorkflow"

		// Test setting and getting
		ctx = WithOperation(ctx, operation)
		assert.Equal(t, operation, GetOperation(ctx))

		// Test empty context
		emptyCtx := context.Background()
		assert.Equal(t, "", GetOperation(emptyCtx))
	})

	t.Run("extract metadata", func(t *testing.T) {
		ctx := context.Background()

		// Add all metadata
		ctx = WithCorrelationID(ctx, "corr-123")
		ctx = WithCausationID(ctx, "caus-456")
		ctx = WithTenantID(ctx, "tenant-789")
		ctx = WithUserID(ctx, "user-abc")
		ctx = WithRequestID(ctx, "req-def")
		ctx = WithOperation(ctx, "TestOp")

		// Extract metadata
		metadata := ExtractMetadata(ctx)

		// Verify all values
		assert.Equal(t, "corr-123", metadata["correlation_id"])
		assert.Equal(t, "caus-456", metadata["causation_id"])
		assert.Equal(t, "tenant-789", metadata["tenant_id"])
		assert.Equal(t, "user-abc", metadata["user_id"])
		assert.Equal(t, "req-def", metadata["request_id"])
		assert.Equal(t, "TestOp", metadata["operation"])

		// Test empty context
		emptyMetadata := ExtractMetadata(context.Background())
		assert.Empty(t, emptyMetadata)
	})

	t.Run("inject metadata", func(t *testing.T) {
		ctx := context.Background()

		metadata := map[string]string{
			"correlation_id": "corr-123",
			"causation_id":   "caus-456",
			"tenant_id":      "tenant-789",
			"user_id":        "user-abc",
			"request_id":     "req-def",
			"operation":      "TestOp",
		}

		// Inject metadata
		ctx = InjectMetadata(ctx, metadata)

		// Verify all values
		assert.Equal(t, "corr-123", GetCorrelationID(ctx))
		assert.Equal(t, "caus-456", GetCausationID(ctx))
		assert.Equal(t, "tenant-789", GetTenantID(ctx))
		assert.Equal(t, "user-abc", GetUserID(ctx))
		assert.Equal(t, "req-def", GetRequestID(ctx))
		assert.Equal(t, "TestOp", GetOperation(ctx))
	})

	t.Run("inject partial metadata", func(t *testing.T) {
		ctx := context.Background()

		// Only inject some metadata
		metadata := map[string]string{
			"correlation_id": "corr-123",
			"tenant_id":      "tenant-789",
		}

		ctx = InjectMetadata(ctx, metadata)

		// Verify injected values
		assert.Equal(t, "corr-123", GetCorrelationID(ctx))
		assert.Equal(t, "tenant-789", GetTenantID(ctx))

		// Verify non-injected values are empty
		assert.Equal(t, "", GetCausationID(ctx))
		assert.Equal(t, "", GetUserID(ctx))
		assert.Equal(t, "", GetRequestID(ctx))
		assert.Equal(t, "", GetOperation(ctx))
	})

	t.Run("inject unknown metadata keys", func(t *testing.T) {
		ctx := context.Background()

		// Include unknown keys
		metadata := map[string]string{
			"correlation_id": "corr-123",
			"unknown_key":    "unknown_value",
		}

		ctx = InjectMetadata(ctx, metadata)

		// Verify known key works
		assert.Equal(t, "corr-123", GetCorrelationID(ctx))

		// Unknown key should be ignored (no panic)
	})

	t.Run("backwards compatibility with string keys", func(t *testing.T) {
		ctx := context.Background()

		// Set values with string keys directly
		ctx = context.WithValue(ctx, "correlation_id", "corr-string")
		ctx = context.WithValue(ctx, "causation_id", "caus-string")
		ctx = context.WithValue(ctx, "tenant_id", "tenant-string")
		ctx = context.WithValue(ctx, "user_id", "user-string")
		ctx = context.WithValue(ctx, "request_id", "req-string")
		ctx = context.WithValue(ctx, "operation", "op-string")

		// Getters should still work
		assert.Equal(t, "corr-string", GetCorrelationID(ctx))
		assert.Equal(t, "caus-string", GetCausationID(ctx))
		assert.Equal(t, "tenant-string", GetTenantID(ctx))
		assert.Equal(t, "user-string", GetUserID(ctx))
		assert.Equal(t, "req-string", GetRequestID(ctx))
		assert.Equal(t, "op-string", GetOperation(ctx))
	})
}
