package auth

import (
    "context"
    "encoding/json"
    "time"
    
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

// AuditEvent represents an authentication audit event
type AuditEvent struct {
    Timestamp   time.Time              `json:"timestamp"`
    EventType   string                 `json:"event_type"`
    UserID      string                 `json:"user_id,omitempty"`
    TenantID    string                 `json:"tenant_id,omitempty"`
    AuthType    string                 `json:"auth_type"`
    Success     bool                   `json:"success"`
    IPAddress   string                 `json:"ip_address,omitempty"`
    UserAgent   string                 `json:"user_agent,omitempty"`
    Error       string                 `json:"error,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AuditLogger handles authentication audit logging
type AuditLogger struct {
    logger observability.Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger observability.Logger) *AuditLogger {
    return &AuditLogger{
        logger: logger,
    }
}

// LogAuthAttempt logs an authentication attempt
func (al *AuditLogger) LogAuthAttempt(ctx context.Context, event AuditEvent) {
    event.Timestamp = time.Now()
    event.EventType = "auth_attempt"
    
    // Convert to JSON for structured logging
    data, _ := json.Marshal(event)
    
    al.logger.Info("AUDIT: Authentication attempt", map[string]interface{}{
        "audit_event": string(data),
        "event_type":  event.EventType,
        "user_id":     event.UserID,
        "success":     event.Success,
    })
}

// LogAPIKeyCreated logs API key creation
func (al *AuditLogger) LogAPIKeyCreated(ctx context.Context, userID, tenantID, keyName string) {
    event := AuditEvent{
        Timestamp: time.Now(),
        EventType: "api_key_created",
        UserID:    userID,
        TenantID:  tenantID,
        Success:   true,
        Metadata: map[string]interface{}{
            "key_name": keyName,
        },
    }
    
    data, _ := json.Marshal(event)
    al.logger.Info("AUDIT: API key created", map[string]interface{}{
        "audit_event": string(data),
    })
}

// LogAPIKeyRevoked logs API key revocation
func (al *AuditLogger) LogAPIKeyRevoked(ctx context.Context, userID, tenantID, keyID string) {
    event := AuditEvent{
        Timestamp: time.Now(),
        EventType: "api_key_revoked",
        UserID:    userID,
        TenantID:  tenantID,
        Success:   true,
        Metadata: map[string]interface{}{
            "key_id": keyID,
        },
    }
    
    data, _ := json.Marshal(event)
    al.logger.Info("AUDIT: API key revoked", map[string]interface{}{
        "audit_event": string(data),
    })
}

// LogRateLimitExceeded logs rate limit exceeded events
func (al *AuditLogger) LogRateLimitExceeded(ctx context.Context, identifier, ipAddress string) {
    event := AuditEvent{
        Timestamp: time.Now(),
        EventType: "rate_limit_exceeded",
        IPAddress: ipAddress,
        Success:   false,
        Metadata: map[string]interface{}{
            "identifier": identifier,
        },
    }
    
    data, _ := json.Marshal(event)
    al.logger.Warn("AUDIT: Rate limit exceeded", map[string]interface{}{
        "audit_event": string(data),
    })
}