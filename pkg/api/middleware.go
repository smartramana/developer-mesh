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

		// Record metrics
		duration := time.Since(start)
		status := c.Writer.Status()

		// Log metrics (in a real implementation, send to metrics backend)
		if os.Getenv("ENABLE_METRICS_LOGGING") == "true" {
			log.Printf("[METRICS] method=%s path=%s status=%d duration=%v",
				c.Request.Method,
				c.Request.URL.Path,
				status,
				duration,
			)
		}
	}
}

// RateLimiterConfig holds the configuration for rate limiting
type RateLimiterConfig struct {
	RequestsPerSecond float64
	Burst             int
	Message           string
	StatusCode        int
}

// DefaultRateLimiterConfig provides sensible defaults
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             20,
		Message:           "Rate limit exceeded. Please retry later.",
		StatusCode:        http.StatusTooManyRequests,
	}
}

// NewRateLimiterConfigFromConfig creates a rate limiter config from the API config
func NewRateLimiterConfigFromConfig(cfg RateLimitConfig) RateLimiterConfig {
	// Convert limit per period to requests per second
	requestsPerSecond := float64(cfg.Limit) / cfg.Period.Seconds()

	return RateLimiterConfig{
		RequestsPerSecond: requestsPerSecond,
		Burst:             cfg.Limit * cfg.BurstFactor,
		Message:           "Rate limit exceeded. Please retry later.",
		StatusCode:        http.StatusTooManyRequests,
	}
}

// Global rate limiter storage
var (
	limiters = make(map[string]*rate.Limiter)
	mu       sync.Mutex
)

// getLimiter returns the rate limiter for the given key
func getLimiter(key string, cfg RateLimiterConfig) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	limiter, exists := limiters[key]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(cfg.RequestsPerSecond), cfg.Burst)
		limiters[key] = limiter
	}

	return limiter
}

// RateLimiter middleware implements rate limiting per IP address
func RateLimiter(cfg RateLimiterConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		key := c.ClientIP()

		// Get the rate limiter for this IP
		limiter := getLimiter(key, cfg)

		// Check if request is allowed
		if !limiter.Allow() {
			c.JSON(cfg.StatusCode, gin.H{
				"error":   cfg.Message,
				"code":    "RATE_LIMIT_EXCEEDED",
				"details": "Too many requests from this IP address",
			})
			c.Abort()
			return
		}

		// Continue processing
		c.Next()
	}
}

// CompressionMiddleware adds gzip compression to responses
func CompressionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if client accepts gzip
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		// Create gzip writer
		gz := gzip.NewWriter(c.Writer)
		defer func() {
			if err := gz.Close(); err != nil {
				log.Printf("Error closing gzip writer: %v", err)
			}
		}()

		// Replace the writer
		c.Writer = &gzipWriter{Writer: gz, ResponseWriter: c.Writer}

		// Set appropriate headers
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")

		// Process request
		c.Next()
	}
}

// gzipWriter wraps the response writer to provide gzip compression
type gzipWriter struct {
	gin.ResponseWriter
	Writer *gzip.Writer
}

func (g *gzipWriter) WriteString(s string) (int, error) {
	return g.Writer.Write([]byte(s))
}

func (g *gzipWriter) Write(data []byte) (int, error) {
	return g.Writer.Write(data)
}

func (g *gzipWriter) WriteHeader(code int) {
	g.ResponseWriter.WriteHeader(code)
}

// CachingMiddleware adds HTTP caching headers
func CachingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip caching for non-GET requests
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		// Set cache headers based on the path
		path := c.Request.URL.Path
		switch {
		case strings.HasPrefix(path, "/api/v1/contexts/"):
			// Cache context data for 5 minutes
			c.Header("Cache-Control", "public, max-age=300")
		case strings.HasPrefix(path, "/api/v1/tools"):
			// Cache tool information for 1 hour
			c.Header("Cache-Control", "public, max-age=3600")
		case strings.HasPrefix(path, "/health"):
			// Don't cache health checks
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		default:
			// Default cache for 1 minute
			c.Header("Cache-Control", "public, max-age=60")
		}

		// Add ETag support (simplified version)
		// In production, you'd calculate this based on the response content
		etag := fmt.Sprintf(`"%d"`, time.Now().Unix())
		c.Header("ETag", etag)

		// Check If-None-Match header
		if match := c.GetHeader("If-None-Match"); match == etag {
			c.AbortWithStatus(http.StatusNotModified)
			return
		}

		c.Next()
	}
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// CORSMiddleware adds CORS headers to responses
func CORSMiddleware(cfg CORSConfig) gin.HandlerFunc {
	// Set defaults
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	}
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-Key"}
	}
	if len(cfg.ExposedHeaders) == 0 {
		cfg.ExposedHeaders = []string{"Content-Length"}
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 86400 // 24 hours
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range cfg.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
			c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
			c.Header("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))
			c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))

			if cfg.AllowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		}

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
