package auth

import (
	"context"
	"testing"

	"github.com/S-Corkum/devops-mcp/internal/observability"
)

func TestTokenProvider_GetToken(t *testing.T) {
	logger := observability.NewLogger("test.authprovider")
	provider := NewTokenProvider("test-token", logger)
	token, err := provider.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "test-token" {
		t.Errorf("expected token 'test-token', got '%s'", token)
	}
}

func TestTokenProvider_EmptyToken(t *testing.T) {
	logger := observability.NewLogger("test.authprovider")
	provider := NewTokenProvider("", logger)
	_, err := provider.GetToken(context.Background())
	if err == nil {
		t.Error("expected error for empty token, got nil")
	}
}

func TestNoAuthProvider_IsValid(t *testing.T) {
	logger := observability.NewLogger("test.authprovider")
	provider := NewNoAuthProvider(logger)
	if provider.IsValid() {
		t.Error("expected IsValid to be false for NoAuthProvider")
	}
}
