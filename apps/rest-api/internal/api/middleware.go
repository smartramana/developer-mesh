package api

import (
	"compress/gzip"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// RateLimiterConfig defines configuration for rate limiting used by the middleware
type RateLimiterConfig struct {
	Limit      float64       // Number of requests allowed per second
	Burst      int           // Number of requests that can be made in a burst
	Expiration time.Duration // How long to keep track of rate limits for a user
}

// NewRateLimiterConfigFromConfig creates a middleware rate limiter config from the API config
func NewRateLimiterConfigFromConfig(cfg RateLimitConfig) RateLimiterConfig {
	return RateLimiterConfig{
		Limit:      float64(cfg.Limit),
		Burst:      cfg.Limit * cfg.BurstFactor,
		Expiration: 1 * time.Hour, // Default expiration
	}
}

// RateLimiterStorage provides storage for rate limiting
type RateLimiterStorage struct {
	limiters map[string]*rate.Limiter
	expiry   map[string]time.Time
	config   RateLimiterConfig
	mu       sync.RWMutex // Protect map access with mutex
	done     chan struct{}
}

// NewRateLimiterStorage creates a new rate limiter storage
func NewRateLimiterStorage(config RateLimiterConfig) *RateLimiterStorage {
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
func RateLimiter(config RateLimiterConfig) gin.HandlerFunc {
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
				"error":       "Rate limit exceeded",
				"retry_after": "60", // Add retry information
			})
			return
		}

		// Process request
		c.Next()
	}
}

// Avoid duplicate declaration - shutdownHooks is already defined in server.go

// CompressionMiddleware compresses HTTP responses
func CompressionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if client accepts gzip encoding
		if !strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
			// Client doesn't accept gzip encoding, skip compression
			c.Next()
			return
		}

		// Create gzip writer
		gz, err := gzip.NewWriterLevel(c.Writer, gzip.BestCompression)
		if err != nil {
			// If gzip writer creation fails, skip compression
			c.Next()
			return
		}
		defer func() {
			if err := gz.Close(); err != nil {
				// Log error if logger is available via context
				// For now, we'll ignore close errors as they're non-critical
				// and any write errors would have already been reported
				_ = err
			}
		}()

		// Create a gzipped response writer
		gzWriter := &gzipResponseWriter{
			ResponseWriter: c.Writer,
			Writer:         gz,
		}

		// Replace writer with gzip writer
		c.Writer = gzWriter

		// Add gzip content encoding header
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")

		// Continue with the request
		c.Next()
	}
}

// gzipResponseWriter wraps the original response writer with gzip
type gzipResponseWriter struct {
	gin.ResponseWriter
	Writer *gzip.Writer
}

// Write implements the io.Writer interface
func (g *gzipResponseWriter) Write(data []byte) (int, error) {
	// Write the data through the gzip writer
	return g.Writer.Write(data)
}

// WriteString implements the io.StringWriter interface
func (g *gzipResponseWriter) WriteString(s string) (int, error) {
	// Write the string through the gzip writer
	return g.Writer.Write([]byte(s))
}

// CORSConfig defines configuration for CORS middleware
type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// CachingMiddleware adds HTTP caching headers
func CachingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip non-GET requests
		if c.Request.Method != "GET" {
			c.Next()
			return
		}

		// Process the request
		c.Next()

		// After the request is processed, add caching headers if status is successful
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			// Check if Cache-Control is already set
			if c.Writer.Header().Get("Cache-Control") == "" {
				// Default cache policy for GET requests
				// Define different cache policies based on path
				path := c.Request.URL.Path

				// Schema and documentation can be cached longer
				if strings.Contains(path, "/swagger") {
					c.Header("Cache-Control", "public, max-age=86400") // 1 day
				} else if strings.HasPrefix(path, "/api/v1/tools") && !strings.Contains(path, "/actions/") {
					// Tool metadata can be cached but not tool actions
					c.Header("Cache-Control", "public, max-age=3600") // 1 hour
				} else {
					// Default for other GET requests - short cache with revalidation
					c.Header("Cache-Control", "private, max-age=60, must-revalidate") // 1 minute
				}

				// Add ETag based on response size and last modified time
				// In a real implementation, this would be a hash of the response content
				etag := fmt.Sprintf("W/\"%d-%s\"", c.Writer.Size(), time.Now().UTC().Format(http.TimeFormat))
				c.Header("ETag", etag)

				// Add Last-Modified header - in a real implementation this would come from the resource
				c.Header("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			}
		}
	}
}

// CORSMiddleware enables Cross-Origin Resource Sharing
func CORSMiddleware(corsConfig CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get origin from request
		origin := c.Request.Header.Get("Origin")

		// Use configuration if available, otherwise default to more restrictive list
		allowedOrigins := corsConfig.AllowedOrigins
		if len(allowedOrigins) == 0 {
			// Only allow localhost in development mode
			if os.Getenv("ENVIRONMENT") == "development" || os.Getenv("GO_ENV") == "development" {
				allowedOrigins = []string{"http://localhost:3000"}
			} else {
				// In production, origins must be explicitly configured
				allowedOrigins = []string{} // No default origins in production
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

// TenantMiddleware extracts the X-Tenant-ID header and sets it in the Gin context as "user".
// This should be registered before any handler that requires tenant scoping.
// Usage (in server.go):
// router.Use(api.TenantMiddleware())
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.Request.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing tenant id"})
			return
		}
		c.Set("user", map[string]any{"tenant_id": tenantID})
		c.Next()
	}
}

// NoAuthMiddleware is a middleware that allows all requests through without authentication
func NoAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		fmt.Println("Using NoAuthMiddleware - all requests allowed")
		c.Next()
	}
}

// ExtractTenantContext follows CLAUDE.md adapter pattern
// This middleware should be called AFTER authentication middleware
func ExtractTenantContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// The auth middleware sets the User under auth.UserContextKey
		// but also sets tenant_id directly (as UUID)
		if tenantID, exists := c.Get("tenant_id"); exists {
			var tidStr string

			// Handle both string and UUID types
			switch v := tenantID.(type) {
			case string:
				tidStr = v
			case uuid.UUID:
				tidStr = v.String()
			case fmt.Stringer: // This covers other types that implement Stringer
				tidStr = v.String()
			default:
				// Log unexpected type for debugging
				log.Printf("ExtractTenantContext: unexpected tenant_id type: %T, value: %v", v, v)
			}

			if tidStr != "" {
				c.Set("tenant_id", tidStr) // Always store as string
				c.Header("X-Tenant-ID", tidStr)
				c.Next()
				return
			}
		}

		// Try to get from user context (backward compatibility)
		if userVal, userExists := c.Get("user"); userExists {
			if userMap, ok := userVal.(map[string]interface{}); ok {
				// tenant_id in the map could be either string or UUID
				if tenantIDRaw, hasTenant := userMap["tenant_id"]; hasTenant {
					var tidStr string
					switch v := tenantIDRaw.(type) {
					case string:
						tidStr = v
					case uuid.UUID:
						tidStr = v.String()
					case fmt.Stringer: // This covers other types that implement Stringer
						tidStr = v.String()
					default:
						// Log unexpected type for debugging
						log.Printf("ExtractTenantContext: unexpected user.tenant_id type: %T, value: %v", v, v)
					}

					if tidStr != "" {
						c.Set("tenant_id", tidStr)
						c.Header("X-Tenant-ID", tidStr)
					}
				}
			}
		}

		c.Next()
	}
}
