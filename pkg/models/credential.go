package models

import (
	"database/sql"
	"time"
)

// ServiceType represents the type of third-party service
type ServiceType string

const (
	ServiceTypeGitHub      ServiceType = "github"
	ServiceTypeHarness     ServiceType = "harness"
	ServiceTypeAWS         ServiceType = "aws"
	ServiceTypeAzure       ServiceType = "azure"
	ServiceTypeGCP         ServiceType = "gcp"
	ServiceTypeSnyk        ServiceType = "snyk"
	ServiceTypeJira        ServiceType = "jira"
	ServiceTypeSlack       ServiceType = "slack"
	ServiceTypeSonarQube   ServiceType = "sonarqube"
	ServiceTypeArtifactory ServiceType = "artifactory"
	ServiceTypeJenkins     ServiceType = "jenkins"
	ServiceTypeGitLab      ServiceType = "gitlab"
	ServiceTypeBitbucket   ServiceType = "bitbucket"
	ServiceTypeConfluence  ServiceType = "confluence"
	ServiceTypeGeneric     ServiceType = "generic"
)

// UserCredential represents stored credentials for a third-party service
type UserCredential struct {
	ID                   string                 `json:"id" db:"id"`
	TenantID             string                 `json:"tenant_id" db:"tenant_id"`
	UserID               string                 `json:"user_id" db:"user_id"`
	ServiceType          ServiceType            `json:"service_type" db:"service_type"`
	EncryptedCredentials []byte                 `json:"-" db:"encrypted_credentials"` // Never expose in JSON
	EncryptionKeyVersion int                    `json:"encryption_key_version" db:"encryption_key_version"`
	IsActive             bool                   `json:"is_active" db:"is_active"`
	Metadata             map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt            time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at" db:"updated_at"`
	LastUsedAt           *time.Time             `json:"last_used_at,omitempty" db:"last_used_at"`
	ExpiresAt            *time.Time             `json:"expires_at,omitempty" db:"expires_at"`
}

// CredentialPayload is the incoming request for storing credentials
type CredentialPayload struct {
	ServiceType ServiceType            `json:"service_type" binding:"required"`
	Credentials map[string]string      `json:"credentials" binding:"required"` // Will be encrypted before storage
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
}

// CredentialResponse is returned to the client (without sensitive data)
type CredentialResponse struct {
	ID             string                 `json:"id"`
	ServiceType    ServiceType            `json:"service_type"`
	IsActive       bool                   `json:"is_active"`
	HasCredentials bool                   `json:"has_credentials"` // Indicates if credentials are configured
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	LastUsedAt     *time.Time             `json:"last_used_at,omitempty"`
	ExpiresAt      *time.Time             `json:"expires_at,omitempty"`
}

// ToResponse converts UserCredential to CredentialResponse (safe for API)
func (uc *UserCredential) ToResponse() *CredentialResponse {
	return &CredentialResponse{
		ID:             uc.ID,
		ServiceType:    uc.ServiceType,
		IsActive:       uc.IsActive,
		HasCredentials: len(uc.EncryptedCredentials) > 0,
		Metadata:       uc.Metadata,
		CreatedAt:      uc.CreatedAt,
		UpdatedAt:      uc.UpdatedAt,
		LastUsedAt:     uc.LastUsedAt,
		ExpiresAt:      uc.ExpiresAt,
	}
}

// UserCredentialAudit represents an audit log entry for credential operations
type UserCredentialAudit struct {
	ID           string                 `json:"id" db:"id"`
	CredentialID string                 `json:"credential_id" db:"credential_id"`
	TenantID     string                 `json:"tenant_id" db:"tenant_id"`
	UserID       string                 `json:"user_id" db:"user_id"`
	ServiceType  ServiceType            `json:"service_type" db:"service_type"`
	Operation    string                 `json:"operation" db:"operation"` // created, updated, deleted, used, validated
	Success      bool                   `json:"success" db:"success"`
	ErrorMessage sql.NullString         `json:"error_message,omitempty" db:"error_message"`
	IPAddress    sql.NullString         `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent    sql.NullString         `json:"user_agent,omitempty" db:"user_agent"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
}

// DecryptedCredentials represents the structure after decryption
// This should never be serialized to JSON or logged
type DecryptedCredentials struct {
	ServiceType ServiceType            `json:"service_type"`
	Credentials map[string]string      `json:"credentials"` // e.g., {"token": "ghp_...", "region": "us-east-1"}
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PassthroughCredential converts DecryptedCredentials to PassthroughCredential format
// used by edge-mcp for passing credentials to tool executions
func (dc *DecryptedCredentials) ToPassthroughCredential() *PassthroughCredential {
	cred := &PassthroughCredential{
		Type:       "bearer", // Default type
		Properties: make(map[string]string),
	}

	// Map service-specific credential formats
	switch dc.ServiceType {
	case ServiceTypeGitHub:
		if token, ok := dc.Credentials["token"]; ok {
			cred.Token = token
		}

	case ServiceTypeHarness:
		if token, ok := dc.Credentials["token"]; ok {
			cred.Token = token
		}
		if apiKey, ok := dc.Credentials["api_key"]; ok {
			cred.Token = apiKey
		}
		// Copy permissions if available in metadata
		if dc.Metadata != nil {
			if permissions, ok := dc.Metadata["permissions"].(map[string]interface{}); ok {
				for k, v := range permissions {
					if strVal, ok := v.(string); ok {
						cred.Properties[k] = strVal
					}
				}
			}
		}

	case ServiceTypeAWS:
		cred.Type = "aws_signature"
		if accessKey, ok := dc.Credentials["access_key"]; ok {
			cred.Properties["access_key"] = accessKey
		}
		if secretKey, ok := dc.Credentials["secret_key"]; ok {
			cred.Properties["secret_key"] = secretKey
		}
		if sessionToken, ok := dc.Credentials["session_token"]; ok {
			cred.Properties["session_token"] = sessionToken
		}
		if region, ok := dc.Credentials["region"]; ok {
			cred.Properties["region"] = region
		}

	case ServiceTypeAzure:
		cred.Type = "azure_ad"
		for k, v := range dc.Credentials {
			cred.Properties[k] = v
		}

	case ServiceTypeGCP:
		cred.Type = "gcp_service_account"
		if key, ok := dc.Credentials["service_account_key"]; ok {
			cred.Token = key
		}

	case ServiceTypeSonarQube:
		if token, ok := dc.Credentials["token"]; ok {
			cred.Token = token
		}
		if baseURL, ok := dc.Credentials["base_url"]; ok {
			cred.Properties["base_url"] = baseURL
		}

	case ServiceTypeArtifactory:
		if token, ok := dc.Credentials["token"]; ok {
			cred.Token = token
		}
		if apiKey, ok := dc.Credentials["api_key"]; ok {
			cred.Token = apiKey
		}
		if username, ok := dc.Credentials["username"]; ok {
			cred.Username = username
		}
		if password, ok := dc.Credentials["password"]; ok {
			cred.Password = password
		}
		if baseURL, ok := dc.Credentials["base_url"]; ok {
			cred.Properties["base_url"] = baseURL
		}

	case ServiceTypeJenkins:
		cred.Type = "basic"
		if username, ok := dc.Credentials["username"]; ok {
			cred.Username = username
		}
		if apiToken, ok := dc.Credentials["api_token"]; ok {
			cred.Token = apiToken
		}
		if password, ok := dc.Credentials["password"]; ok {
			cred.Password = password
		}
		if baseURL, ok := dc.Credentials["base_url"]; ok {
			cred.Properties["base_url"] = baseURL
		}

	case ServiceTypeGitLab:
		if token, ok := dc.Credentials["token"]; ok {
			cred.Token = token
		}
		if baseURL, ok := dc.Credentials["base_url"]; ok {
			cred.Properties["base_url"] = baseURL
		}

	case ServiceTypeBitbucket:
		if token, ok := dc.Credentials["token"]; ok {
			cred.Token = token
		}
		if username, ok := dc.Credentials["username"]; ok {
			cred.Username = username
		}
		if appPassword, ok := dc.Credentials["app_password"]; ok {
			cred.Password = appPassword
		}
		if baseURL, ok := dc.Credentials["base_url"]; ok {
			cred.Properties["base_url"] = baseURL
		}

	case ServiceTypeConfluence:
		cred.Type = "basic"
		if email, ok := dc.Credentials["email"]; ok {
			cred.Username = email
		}
		if apiToken, ok := dc.Credentials["api_token"]; ok {
			cred.Token = apiToken
		}
		if baseURL, ok := dc.Credentials["base_url"]; ok {
			cred.Properties["base_url"] = baseURL
		}

	default:
		// Generic handling - store all credentials in properties
		for k, v := range dc.Credentials {
			if k == "token" {
				cred.Token = v
			} else {
				cred.Properties[k] = v
			}
		}
	}

	return cred
}
