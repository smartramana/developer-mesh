package auth

import (
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyType_Valid(t *testing.T) {
	tests := []struct {
		name     string
		keyType  KeyType
		expected bool
	}{
		{"User type", KeyTypeUser, true},
		{"Admin type", KeyTypeAdmin, true},
		{"Agent type", KeyTypeAgent, true},
		{"Gateway type", KeyTypeGateway, true},
		{"Invalid type", KeyType("invalid"), false},
		{"Empty type", KeyType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.keyType.Valid())
		})
	}
}

func TestKeyType_Scan(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected KeyType
	}{
		{"String user", "user", KeyTypeUser},
		{"String admin", "admin", KeyTypeAdmin},
		{"Byte array", []byte("agent"), KeyTypeAgent},
		{"Invalid string", "invalid", KeyTypeUser}, // Defaults to user
		{"Nil value", nil, KeyTypeUser},            // Defaults to user
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var kt KeyType
			err := kt.Scan(tt.value)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, kt)
		})
	}
}

func TestKeyType_Value(t *testing.T) {
	tests := []struct {
		name     string
		keyType  KeyType
		expected driver.Value
	}{
		{"User type", KeyTypeUser, "user"},
		{"Admin type", KeyTypeAdmin, "admin"},
		{"Agent type", KeyTypeAgent, "agent"},
		{"Gateway type", KeyTypeGateway, "gateway"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.keyType.Value()
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, value)
		})
	}
}

func TestKeyType_GetScopes(t *testing.T) {
	tests := []struct {
		name     string
		keyType  KeyType
		expected []string
	}{
		{"User scopes", KeyTypeUser, []string{"read"}},
		{"Admin scopes", KeyTypeAdmin, []string{"read", "write", "admin"}},
		{"Agent scopes", KeyTypeAgent, []string{"read", "write", "agent"}},
		{"Gateway scopes", KeyTypeGateway, []string{"read", "write", "gateway"}},
		{"Invalid type scopes", KeyType("invalid"), []string{"read"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.keyType.GetScopes())
		})
	}
}

func TestKeyType_GetRateLimit(t *testing.T) {
	tests := []struct {
		name     string
		keyType  KeyType
		expected int
	}{
		{"User rate limit", KeyTypeUser, 100},
		{"Admin rate limit", KeyTypeAdmin, 10000},
		{"Agent rate limit", KeyTypeAgent, 1000},
		{"Gateway rate limit", KeyTypeGateway, 5000},
		{"Invalid type rate limit", KeyType("invalid"), 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.keyType.GetRateLimit())
		})
	}
}
