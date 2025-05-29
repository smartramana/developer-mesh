package util

import "github.com/google/uuid"

// GenerateUUID returns a new UUID string
func GenerateUUID() string {
	return uuid.New().String()
}
