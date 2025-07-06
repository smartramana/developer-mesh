package auth

import (
	"database/sql/driver"
	"fmt"
)

// KeyType represents the type of API key
type KeyType string

const (
	KeyTypeUser    KeyType = "user"    // Regular user access
	KeyTypeAdmin   KeyType = "admin"   // Full system access
	KeyTypeAgent   KeyType = "agent"   // AI agents
	KeyTypeGateway KeyType = "gateway" // Local MCP instances
)

// Valid returns true if the key type is valid
func (kt KeyType) Valid() bool {
	switch kt {
	case KeyTypeUser, KeyTypeAdmin, KeyTypeAgent, KeyTypeGateway:
		return true
	default:
		return false
	}
}

// Scan implements sql.Scanner for database operations
func (kt *KeyType) Scan(value interface{}) error {
	if value == nil {
		*kt = KeyTypeUser
		return nil
	}

	switch v := value.(type) {
	case string:
		*kt = KeyType(v)
	case []byte:
		*kt = KeyType(string(v))
	default:
		return fmt.Errorf("cannot scan %T into KeyType", value)
	}

	if !kt.Valid() {
		*kt = KeyTypeUser
	}

	return nil
}

// Value implements driver.Valuer for database operations
func (kt KeyType) Value() (driver.Value, error) {
	return string(kt), nil
}

// String implements fmt.Stringer
func (kt KeyType) String() string {
	return string(kt)
}

// GetScopes returns the default scopes for a key type
func (kt KeyType) GetScopes() []string {
	switch kt {
	case KeyTypeAdmin:
		return []string{"read", "write", "admin"}
	case KeyTypeGateway:
		return []string{"read", "write", "gateway"}
	case KeyTypeAgent:
		return []string{"read", "write", "agent"}
	default:
		return []string{"read"}
	}
}

// GetRateLimit returns the default rate limit for a key type
func (kt KeyType) GetRateLimit() int {
	switch kt {
	case KeyTypeAdmin:
		return 10000
	case KeyTypeGateway:
		return 5000
	case KeyTypeAgent:
		return 1000
	default:
		return 100
	}
}
