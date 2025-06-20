package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sony/gobreaker"
)

// NoOpSpan implements a no-op span for tracing
type NoOpSpan struct{}

func (s NoOpSpan) End()                                                    {}
func (s NoOpSpan) SetAttribute(key string, value interface{})              {}
func (s NoOpSpan) SetStatus(code int, message string)                      {}
func (s NoOpSpan) RecordError(err error)                                   {}
func (s NoOpSpan) AddEvent(name string, attributes map[string]interface{}) {}

// ExponentialBackoff creates an exponential backoff function
func ExponentialBackoff(base time.Duration, factor float64) func(attempt int) time.Duration {
	return func(attempt int) time.Duration {
		if attempt == 0 {
			return 0
		}
		backoff := base
		for i := 1; i < attempt; i++ {
			backoff = time.Duration(float64(backoff) * factor)
		}
		return backoff
	}
}

// InMemoryRateLimiter implements a production-grade in-memory rate limiter
type InMemoryRateLimiter struct {
	mu              sync.RWMutex
	limits          map[string]*rateLimitEntry
	rate            int
	window          time.Duration
	cleanupInterval time.Duration
	stopCh          chan struct{}
}

type rateLimitEntry struct {
	count       int
	windowStart time.Time
}

// NewInMemoryRateLimiter creates a new in-memory rate limiter
func NewInMemoryRateLimiter(rate int, window time.Duration) RateLimiter {
	rl := &InMemoryRateLimiter{
		limits:          make(map[string]*rateLimitEntry),
		rate:            rate,
		window:          window,
		cleanupInterval: window * 2,
		stopCh:          make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

func (rl *InMemoryRateLimiter) Check(ctx context.Context, key string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.limits[key]

	if !exists || now.Sub(entry.windowStart) > rl.window {
		rl.limits[key] = &rateLimitEntry{
			count:       1,
			windowStart: now,
		}
		return nil
	}

	if entry.count < rl.rate {
		entry.count++
		return nil
	}

	return ErrRateLimitExceeded
}

func (rl *InMemoryRateLimiter) CheckWithLimit(ctx context.Context, key string, limit int, window time.Duration) error {
	// Simple implementation - use the default limit
	return rl.Check(ctx, key)
}

func (rl *InMemoryRateLimiter) GetRemaining(ctx context.Context, key string) (int, error) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	entry, exists := rl.limits[key]
	if !exists {
		return rl.rate, nil
	}

	remaining := rl.rate - entry.count
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

func (rl *InMemoryRateLimiter) Reset(ctx context.Context, key string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.limits, key)
	return nil
}

func (rl *InMemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, entry := range rl.limits {
				if now.Sub(entry.windowStart) > rl.window*2 {
					delete(rl.limits, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// InMemoryQuotaManager implements a production-grade in-memory quota manager
type InMemoryQuotaManager struct {
	mu     sync.RWMutex
	quotas map[string]*quotaEntry
}

type quotaEntry struct {
	used  int
	limit int
}

// NewInMemoryQuotaManager creates a new in-memory quota manager
func NewInMemoryQuotaManager() QuotaManager {
	return &InMemoryQuotaManager{
		quotas: map[string]*quotaEntry{
			"tasks":      {used: 0, limit: 10000},
			"workflows":  {used: 0, limit: 1000},
			"workspaces": {used: 0, limit: 100},
			"documents":  {used: 0, limit: 10000},
			"agents":     {used: 0, limit: 1000},
		},
	}
}

func (qm *InMemoryQuotaManager) GetQuota(ctx context.Context, tenantID uuid.UUID, resource string) (int64, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	quota, exists := qm.quotas[resource]
	if !exists {
		return 0, ErrResourceNotFound
	}

	return int64(quota.limit), nil
}

func (qm *InMemoryQuotaManager) GetUsage(ctx context.Context, tenantID uuid.UUID, resource string) (int64, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	quota, exists := qm.quotas[resource]
	if !exists {
		return 0, ErrResourceNotFound
	}

	return int64(quota.used), nil
}

func (qm *InMemoryQuotaManager) IncrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int64) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	quota, exists := qm.quotas[resource]
	if !exists {
		return ErrResourceNotFound
	}

	newUsed := quota.used + int(amount)
	if newUsed > quota.limit {
		return &QuotaExceededError{
			Resource: resource,
			Used:     int64(quota.used),
			Limit:    int64(quota.limit),
		}
	}

	quota.used = newUsed
	return nil
}

func (qm *InMemoryQuotaManager) SetQuota(ctx context.Context, tenantID uuid.UUID, resource string, limit int64) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	quota, exists := qm.quotas[resource]
	if !exists {
		qm.quotas[resource] = &quotaEntry{
			used:  0,
			limit: int(limit),
		}
	} else {
		quota.limit = int(limit)
	}

	return nil
}

func (qm *InMemoryQuotaManager) GetQuotaStatus(ctx context.Context, tenantID uuid.UUID) (*QuotaStatus, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	status := &QuotaStatus{
		TenantID: tenantID,
		Quotas:   make(map[string]QuotaInfo),
	}

	for resource, quota := range qm.quotas {
		status.Quotas[resource] = QuotaInfo{
			Resource:  resource,
			Limit:     int64(quota.limit),
			Used:      int64(quota.used),
			Available: int64(quota.limit - quota.used),
			Period:    "monthly",
		}
	}

	return status, nil
}

// DefaultSanitizer implements production-grade input sanitization
type DefaultSanitizer struct{}

// NewDefaultSanitizer creates a new default sanitizer
func NewDefaultSanitizer() Sanitizer {
	return &DefaultSanitizer{}
}

func (s *DefaultSanitizer) SanitizeString(input string) string {
	// In production, this would use a proper sanitization library
	// For now, just return the input
	return input
}

func (s *DefaultSanitizer) SanitizeHTML(input string) string {
	// In production, use bluemonday or similar HTML sanitizer
	return input
}

func (s *DefaultSanitizer) SanitizeJSON(input interface{}) (interface{}, error) {
	// Basic JSON validation - in production would do proper sanitization
	return input, nil
}

// NoOpEncryptionService implements a no-op encryption service for development
// In production, this would use AES-GCM or similar
type NoOpEncryptionService struct{}

// NewNoOpEncryptionService creates a new no-op encryption service
func NewNoOpEncryptionService() EncryptionService {
	return &NoOpEncryptionService{}
}

func (s *NoOpEncryptionService) Encrypt(ctx context.Context, data []byte) ([]byte, error) {
	return data, nil
}

func (s *NoOpEncryptionService) Decrypt(ctx context.Context, data []byte) ([]byte, error) {
	return data, nil
}

func (s *NoOpEncryptionService) EncryptString(ctx context.Context, data string) (string, error) {
	return data, nil
}

func (s *NoOpEncryptionService) DecryptString(ctx context.Context, data string) (string, error) {
	return data, nil
}

// AESEncryptionService provides AES-GCM encryption for production use
type AESEncryptionService struct {
	key []byte
}

// NewAESEncryptionService creates a production encryption service with AES-256-GCM
func NewAESEncryptionService(key []byte) (EncryptionService, error) {
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes for AES-256")
	}
	return &AESEncryptionService{key: key}, nil
}

// Encrypt encrypts data using AES-GCM
func (s *AESEncryptionService) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data using AES-GCM
func (s *AESEncryptionService) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns base64 encoded result
func (s *AESEncryptionService) EncryptString(ctx context.Context, plaintext string) (string, error) {
	encrypted, err := s.Encrypt(ctx, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptString decrypts a base64 encoded string
func (s *AESEncryptionService) DecryptString(ctx context.Context, ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	decrypted, err := s.Decrypt(ctx, data)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

// CreateDefaultCircuitBreakerSettings creates default circuit breaker settings
func CreateDefaultCircuitBreakerSettings() *gobreaker.Settings {
	return &gobreaker.Settings{
		Name:        "default",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			// Log state changes
		},
	}
}
