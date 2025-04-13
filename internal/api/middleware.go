package api

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RequestLogger middleware logs HTTP requests
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code and client IP
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()

		// Log request
		log.Printf("[API] %s | %3d | %12v | %s | %s",
			clientIP,
			statusCode,
			latency,
			c.Request.Method,
			path,
		)

		// Log errors separately
		if len(c.Errors) > 0 {
			log.Printf("[API ERROR] %s\n", c.Errors.String())
		}
	}
}

// MetricsMiddleware collects API metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// Calculate latency
		_ = time.Since(start) // Using _ to ignore unused latency for now

		// Record metrics (to be implemented based on metrics client)
		// Example: metricsClient.RecordAPIRequest(c.Request.Method, c.Request.URL.Path, c.Writer.Status(), latency)
	}
}

// RateLimiterStorage provides storage for rate limiting
type RateLimiterStorage struct {
	limiters map[string]*rate.Limiter
	expiry   map[string]time.Time
	config   RateLimitConfig
	mu       sync.RWMutex // Protect map access with mutex
	done     chan struct{}
}

// NewRateLimiterStorage creates a new rate limiter storage
func NewRateLimiterStorage(config RateLimitConfig) *RateLimiterStorage {
	storage := &RateLimiterStorage{
		limiters: make(map[string]*rate.Limiter),
		expiry:   make(map[string]time.Time),
		config:   config,
		done:     make(chan struct{}),
	}
	
	// Start a background cleanup job
	go storage.cleanupTask()
	
	return storage
}

// GetLimiter returns a rate limiter for a given key
func (s *RateLimiterStorage) GetLimiter(key string) *rate.Limiter {
	s.mu.RLock()
	// Check if limiter exists and is not expired
	if limiter, exists := s.limiters[key]; exists {
		if time.Now().Before(s.expiry[key]) {
			s.mu.RUnlock()
			return limiter
		}
	}
	s.mu.RUnlock()
	
	// Need to create or update limiter
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Check again in case it was created between locks
	if limiter, exists := s.limiters[key]; exists {
		if time.Now().Before(s.expiry[key]) {
			return limiter
		}
		// Expired, delete it
		delete(s.limiters, key)
		delete(s.expiry, key)
	}

	// Create new limiter
	limiter := rate.NewLimiter(rate.Limit(s.config.Limit), s.config.Burst)
	s.limiters[key] = limiter
	s.expiry[key] = time.Now().Add(s.config.Expiration)

	return limiter
}

// cleanupTask periodically cleans up expired limiters
func (s *RateLimiterStorage) cleanupTask() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.done:
			return
		}
	}
}

// cleanup removes expired limiters
func (s *RateLimiterStorage) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	for key, exp := range s.expiry {
		if now.After(exp) {
			delete(s.limiters, key)
			delete(s.expiry, key)
		}
	}
}

// Close stops the cleanup goroutine
func (s *RateLimiterStorage) Close() {
	close(s.done)
}

// RateLimiter middleware implements rate limiting
func RateLimiter(config RateLimitConfig) gin.HandlerFunc {
	storage := NewRateLimiterStorage(config)
	
	// Add to server shutdown hooks to properly close storage
	// This is a placeholder - in a real app, this should be added to shutdown logic
	shutdownHooks = append(shutdownHooks, func() {
		storage.Close()
	})

	return func(c *gin.Context) {
		var clientID string
		
		// Get client identifier - prefer authenticated user ID if available
		if userID, exists := c.Get("user_id"); exists && userID != nil {
			// Use authenticated user ID if available
			clientID = fmt.Sprintf("user:%v", userID)
		} else {
			// Fallback to IP address with proper forwarded header handling
			// Note: X-Forwarded-For can be spoofed, so in production use a secure
			// proxy configuration that sets X-Real-IP or similar
			clientIP := c.ClientIP()
			clientID = fmt.Sprintf("ip:%s", clientIP)
		}

		// Get limiter for this client
		limiter := storage.GetLimiter(clientID)

		// Check if request allowed
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"retry_after": "60", // Add retry information
			})
			return
		}

		// Process request
		c.Next()
	}
}

// List of functions to call during shutdown
var shutdownHooks []func()

// CORSMiddleware enables Cross-Origin Resource Sharing
func CORSMiddleware(corsConfig *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get origin from request
		origin := c.Request.Header.Get("Origin")
		
		// Use configuration if available, otherwise default to more restrictive list
		allowedOrigins := corsConfig.CORSOrigins
		if len(allowedOrigins) == 0 {
			// Fallback to default if not configured
			allowedOrigins = []string{
				"http://localhost:3000", // For development
			}
		}
		
		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			// Handle wildcard domains (*.example.com)
			if allowedOrigin == "*" {
				// Special case: allow any origin, but still specify the actual origin
				// rather than using the wildcard in the response header
				allowed = true
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				break
			} else if allowedOrigin == origin {
				allowed = true
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
		
		// Only set additional CORS headers if origin is allowed
		if allowed {
			// Set more restrictive CORS headers
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
		}

		// Always respond to OPTIONS method to avoid hanging preflight requests
		if c.Request.Method == "OPTIONS" {
			if allowed {
				c.AbortWithStatus(204) // No content
			} else {
				c.AbortWithStatus(403) // Forbidden
			}
			return
		}

		c.Next()
	}
}

// AuthMiddleware authenticates API requests
func AuthMiddleware(authType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get authentication token from header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization header"})
			return
		}

		// Basic implementation - to be expanded based on auth requirements
		switch authType {
		case "api_key":
			// Validate API key
			if !validateAPIKey(authHeader) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
				return
			}

		case "jwt":
			// Check if token format is valid (should begin with "Bearer ")
			if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
				return
			}
			
			// Extract the JWT token
			tokenString := authHeader[7:]
			
			// Validate JWT token
			if !validateJWT(tokenString) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired JWT token"})
				return
			}

		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Unsupported auth type: %s", authType)})
			return
		}

		c.Next()
	}
}

// apiKeyStorage holds the API keys for validation
var apiKeyStorage struct {
	keys []string
	mu   sync.RWMutex
}

// InitAPIKeys initializes the API key storage
func InitAPIKeys(keys []string) {
	apiKeyStorage.mu.Lock()
	defer apiKeyStorage.mu.Unlock()
	
	apiKeyStorage.keys = make([]string, len(keys))
	copy(apiKeyStorage.keys, keys)
}

// validateAPIKey validates an API key against stored API keys
func validateAPIKey(key string) bool {
	if key == "" {
		return false
	}
	
	// Properly format the API key
	if len(key) < 8 || key[:7] != "ApiKey " {
		return false
	}
	
	apiKey := key[7:]
	
	// Use read lock to protect concurrent access
	apiKeyStorage.mu.RLock()
	defer apiKeyStorage.mu.RUnlock()
	
	// Check if the API key exists in the authorized keys
	for _, validKey := range apiKeyStorage.keys {
		if apiKey == validKey {
			return true
		}
	}
	
	return false
}

// jwtSecret holds the secret used to sign and verify JWT tokens
var jwtSecret []byte

// InitJWT initializes the JWT validation with the given secret
func InitJWT(secret string) {
	if secret != "" {
		jwtSecret = []byte(secret)
	}
}

// validateJWT validates a JWT token
func validateJWT(tokenString string) bool {
	if tokenString == "" || len(jwtSecret) == 0 {
		return false
	}
	
	// Parse and validate the token
	// This is a placeholder - in a real implementation, you would:
	// 1. Parse the JWT token (using a library like github.com/golang-jwt/jwt)
	// 2. Validate the signature using the secret
	// 3. Check if the token has expired
	// 4. Verify any required claims (issuer, audience, etc.)
	
	// Example JWT validation code (commented to avoid adding dependencies):
	/*
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	
	if err != nil {
		return false
	}
	
	// Check if token is valid
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Verify expiration
		exp, ok := claims["exp"].(float64)
		if !ok {
			return false
		}
		
		if time.Now().Unix() > int64(exp) {
			return false
		}
		
		// Additional claims validation can be added here
		
		return true
	}
	*/
	
	// Placeholder return - replace with actual implementation
	return true
}
