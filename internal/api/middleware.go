package api

import (
	"fmt"
	"log"
	"net/http"
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
}

// NewRateLimiterStorage creates a new rate limiter storage
func NewRateLimiterStorage(config RateLimitConfig) *RateLimiterStorage {
	return &RateLimiterStorage{
		limiters: make(map[string]*rate.Limiter),
		expiry:   make(map[string]time.Time),
		config:   config,
	}
}

// GetLimiter returns a rate limiter for a given key
func (s *RateLimiterStorage) GetLimiter(key string) *rate.Limiter {
	// Check if limiter exists and is not expired
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

	// Periodically clean up expired limiters
	// In a production system, this would be done with a separate goroutine

	return limiter
}

// RateLimiter middleware implements rate limiting
func RateLimiter(config RateLimitConfig) gin.HandlerFunc {
	storage := NewRateLimiterStorage(config)

	return func(c *gin.Context) {
		// Get client identifier (IP address in this simple case)
		// In production, might use authenticated user ID or API key
		clientIP := c.ClientIP()

		// Get limiter for this client
		limiter := storage.GetLimiter(clientIP)

		// Check if request allowed
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			return
		}

		// Process request
		c.Next()
	}
}

// CORSMiddleware enables Cross-Origin Resource Sharing
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
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

		// Basic implementation - to be expanded based on auth requirements
		switch authType {
		case "api_key":
			// Validate API key
			if !validateAPIKey(authHeader) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
				return
			}

		case "jwt":
			// Validate JWT token
			if !validateJWT(authHeader) {
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

// validateAPIKey validates an API key
func validateAPIKey(key string) bool {
	// Implementation to be added
	// Should check against stored API keys
	return true
}

// validateJWT validates a JWT token
func validateJWT(token string) bool {
	// Implementation to be added
	// Should verify token signature, expiry, etc.
	return true
}
