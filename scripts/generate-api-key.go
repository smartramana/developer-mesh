package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run generate-api-key.go <key-type>")
		fmt.Println("Key types: admin, gateway, agent, user")
		os.Exit(1)
	}

	keyType := os.Args[1]
	prefix := getPrefix(keyType)

	// Generate secure random key (32 bytes)
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating random key: %v\n", err)
		os.Exit(1)
	}

	// Create key string: prefix + base64(random)
	keyString := fmt.Sprintf("%s_%s", prefix, base64.URLEncoding.EncodeToString(keyBytes))
	
	// Generate SHA256 hash
	hasher := sha256.New()
	hasher.Write([]byte(keyString))
	keyHash := hex.EncodeToString(hasher.Sum(nil))
	
	// Get key prefix (first 8 chars)
	keyPrefix := keyString
	if len(keyPrefix) > 8 {
		keyPrefix = keyPrefix[:8]
	}

	fmt.Println("API Key Generation Results:")
	fmt.Println("==========================")
	fmt.Printf("Key Type:   %s\n", keyType)
	fmt.Printf("Full Key:   %s\n", keyString)
	fmt.Printf("Key Hash:   %s\n", keyHash)
	fmt.Printf("Key Prefix: %s\n", keyPrefix)
	fmt.Println("\nSQL Insert Statement:")
	fmt.Printf(`INSERT INTO mcp.api_keys (
    id, key_hash, key_prefix, tenant_id, user_id, name, key_type,
    scopes, is_active, rate_limit_requests, rate_limit_window_seconds,
    created_at, updated_at
) VALUES (
    uuid_generate_v4(), 
    '%s', 
    '%s', 
    'e2e-test-tenant', 
    NULL, 
    'E2E Test %s Key', 
    '%s',
    ARRAY['read', 'write', 'admin'], 
    true, 
    1000, 
    60,
    CURRENT_TIMESTAMP, 
    CURRENT_TIMESTAMP
);`, keyHash, keyPrefix, keyType, keyType)
}

func getPrefix(keyType string) string {
	switch keyType {
	case "admin":
		return "adm"
	case "gateway":
		return "gw"
	case "agent":
		return "agt"
	default:
		return "usr"
	}
}