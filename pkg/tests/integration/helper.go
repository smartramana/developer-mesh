package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestHelper provides utilities for integration tests
type TestHelper struct {
	t *testing.T
}

// NewTestHelper creates a new test helper
func NewTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{t: t}
}

// Require returns a testify require instance
func (h *TestHelper) Require() *require.Assertions {
	return require.New(h.t)
}

// Context returns a context with timeout for integration tests
func (h *TestHelper) Context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// RunWithTimeout runs the function with a timeout
func (h *TestHelper) RunWithTimeout(f func(ctx context.Context) error) {
	ctx, cancel := h.Context()
	defer cancel()
	err := f(ctx)
	h.Require().NoError(err)
}
