package auth

import (
	"context"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// Note: contextKey type is already defined in middleware.go

const (
	// userCredentialsKey is the context key for user tool credentials
	userCredentialsKey contextKey = "user_tool_credentials" // #nosec G101 - This is a context key name, not a credential
)

// WithToolCredentials adds tool credentials to context
func WithToolCredentials(ctx context.Context, creds *models.ToolCredentials) context.Context {
	if creds == nil {
		return ctx
	}
	return context.WithValue(ctx, userCredentialsKey, creds)
}

// GetToolCredentials retrieves tool credentials from context
func GetToolCredentials(ctx context.Context) (*models.ToolCredentials, bool) {
	if ctx == nil {
		return nil, false
	}

	creds, ok := ctx.Value(userCredentialsKey).(*models.ToolCredentials)
	return creds, ok
}

// GetToolCredential retrieves a specific tool credential from context
func GetToolCredential(ctx context.Context, tool string) (*models.TokenCredential, bool) {
	creds, ok := GetToolCredentials(ctx)
	if !ok || creds == nil {
		return nil, false
	}

	credential := creds.GetCredentialFor(tool)
	return credential, credential != nil
}

// HasToolCredential checks if a specific tool credential exists in context
func HasToolCredential(ctx context.Context, tool string) bool {
	creds, ok := GetToolCredentials(ctx)
	if !ok || creds == nil {
		return false
	}

	return creds.HasCredentialFor(tool)
}

// CredentialContext wraps common credential operations
type CredentialContext struct {
	ctx context.Context
}

// NewCredentialContext creates a new credential context wrapper
func NewCredentialContext(ctx context.Context) *CredentialContext {
	return &CredentialContext{ctx: ctx}
}

// WithCredentials adds credentials to the context
func (cc *CredentialContext) WithCredentials(creds *models.ToolCredentials) *CredentialContext {
	return &CredentialContext{
		ctx: WithToolCredentials(cc.ctx, creds),
	}
}

// GetCredentials retrieves all credentials
func (cc *CredentialContext) GetCredentials() (*models.ToolCredentials, bool) {
	return GetToolCredentials(cc.ctx)
}

// GetCredential retrieves a specific tool credential
func (cc *CredentialContext) GetCredential(tool string) (*models.TokenCredential, bool) {
	return GetToolCredential(cc.ctx, tool)
}

// HasCredential checks if a specific tool credential exists
func (cc *CredentialContext) HasCredential(tool string) bool {
	return HasToolCredential(cc.ctx, tool)
}

// Context returns the underlying context
func (cc *CredentialContext) Context() context.Context {
	return cc.ctx
}
