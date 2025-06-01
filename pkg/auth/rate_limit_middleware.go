package auth

import (
    "net/http"
    
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

// RateLimitMiddleware creates HTTP middleware for rate limiting
func RateLimitMiddleware(rateLimiter *RateLimiter, logger observability.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Skip rate limiting for non-auth endpoints
            if !isAuthEndpoint(r.URL.Path) {
                next.ServeHTTP(w, r)
                return
            }
            
            // Get identifier (IP + User-Agent for anonymous, or user ID for authenticated)
            identifier := getIdentifier(r)
            
            // Check rate limit
            if err := rateLimiter.CheckLimit(r.Context(), identifier); err != nil {
                logger.Warn("Rate limit exceeded", map[string]interface{}{
                    "identifier": identifier,
                    "path":       r.URL.Path,
                    "error":      err.Error(),
                })
                
                w.Header().Set("X-RateLimit-Remaining", "0")
                w.Header().Set("Retry-After", "900") // 15 minutes
                http.Error(w, "Too many authentication attempts", http.StatusTooManyRequests)
                return
            }
            
            // Wrap response writer to capture status
            wrapped := &responseWriter{
                ResponseWriter: w,
                statusCode:     http.StatusOK,
                written:        false,
            }
            
            next.ServeHTTP(wrapped, r)
            
            // Record attempt based on response
            success := wrapped.statusCode < 400
            rateLimiter.RecordAttempt(r.Context(), identifier, success)
        })
    }
}

// Note: Helper functions isAuthEndpoint and getIdentifier are defined in auth_middleware.go

type responseWriter struct {
    http.ResponseWriter
    statusCode int
    written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
    if !rw.written {
        rw.statusCode = code
        rw.written = true
        rw.ResponseWriter.WriteHeader(code)
    }
}

func (rw *responseWriter) Write(b []byte) (int, error) {
    if !rw.written {
        rw.WriteHeader(http.StatusOK)
    }
    return rw.ResponseWriter.Write(b)
}