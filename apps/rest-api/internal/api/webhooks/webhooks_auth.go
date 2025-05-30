package webhooks

import (
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/queue"
)

// extractAuthContext extracts authentication context from the webhook payload
func extractAuthContext(payload map[string]interface{}, eventType string) *queue.EventAuthContext {
	// Check if there's an installation context
	if installation, ok := payload["installation"].(map[string]interface{}); ok {
		installationID, _ := installation["id"].(float64)
		if installationID > 0 {
			// Extract repository info
			var repoOwner string
			if repo, ok := payload["repository"].(map[string]interface{}); ok {
				if owner, ok := repo["owner"].(map[string]interface{}); ok {
					repoOwner, _ = owner["login"].(string)
				}
			}

			// Extract sender info
			var senderLogin string
			var senderType string
			if sender, ok := payload["sender"].(map[string]interface{}); ok {
				senderLogin, _ = sender["login"].(string)
				senderType, _ = sender["type"].(string)
			}

			// Build auth context
			installationIDInt64 := int64(installationID)
			return &queue.EventAuthContext{
				TenantID:       fmt.Sprintf("github-%d", installationIDInt64), // This will be mapped to actual tenant by worker
				PrincipalID:    fmt.Sprintf("github:webhook:%d:%s", installationIDInt64, eventType),
				PrincipalType:  "installation",
				InstallationID: &installationIDInt64,
				Permissions:    []string{"webhook:process", "queue:publish"},
				Metadata: map[string]interface{}{
					"event_type":   eventType,
					"repo_owner":   repoOwner,
					"sender_login": senderLogin,
					"sender_type":  senderType,
				},
			}
		}
	}

	// No installation context - this might be an org-level event
	return nil
}
