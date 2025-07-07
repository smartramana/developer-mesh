package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GinMiddlewareWithPassthrough creates a Gin middleware for authentication with token passthrough support
func (s *Service) GinMiddlewareWithPassthrough(authTypes ...Type) gin.HandlerFunc {
	// Default to API key auth if no types specified
	if len(authTypes) == 0 {
		authTypes = []Type{TypeAPIKey}
	}

	return func(c *gin.Context) {
		var user *User
		var err error

		// Try each auth type in order (copied from base middleware)
		for _, authType := range authTypes {
			switch authType {
			case TypeAPIKey:
				// Check Authorization header first
				authHeader := c.GetHeader("Authorization")
				if authHeader != "" {
					// Handle "Bearer <token>" format
					if strings.HasPrefix(authHeader, "Bearer ") {
						apiKey := strings.TrimPrefix(authHeader, "Bearer ")
						user, err = s.ValidateAPIKey(c.Request.Context(), apiKey)
					} else {
						// Direct API key
						user, err = s.ValidateAPIKey(c.Request.Context(), authHeader)
					}
				}

				// Check custom API key header if Authorization header didn't work
				if user == nil && s.config.APIKeyHeader != "" {
					apiKey := c.GetHeader(s.config.APIKeyHeader)
					if apiKey != "" {
						user, err = s.ValidateAPIKey(c.Request.Context(), apiKey)
					}
				}

			case TypeJWT:
				// Check Authorization header for JWT
				authHeader := c.GetHeader("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					token := strings.TrimPrefix(authHeader, "Bearer ")
					user, err = s.ValidateJWT(c.Request.Context(), token)
				}

			case TypeNone:
				// No authentication required
				c.Next()
				return
			}

			// If we found a valid user, break out of the loop
			if user != nil {
				break
			}
		}

		// Check if authentication succeeded
		if user == nil {
			s.logger.Warn("Authentication failed", map[string]interface{}{
				"error":     err,
				"path":      c.Request.URL.Path,
				"remote_ip": c.ClientIP(),
			})

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		// Store user in context
		c.Set(string(UserContextKey), user)

		// For compatibility with existing code that expects "user" key
		c.Set("user", map[string]interface{}{
			"id":        user.ID,
			"tenant_id": user.TenantID,
			"email":     user.Email,
			"scopes":    user.Scopes,
			"auth_type": string(user.AuthType),
		})

		// Also set individual keys for backward compatibility
		c.Set("user_id", user.ID)
		c.Set("tenant_id", user.TenantID)

		// Log successful authentication
		s.logger.Debug("Authentication successful", map[string]interface{}{
			"user_id":   user.ID,
			"tenant_id": user.TenantID,
			"auth_type": string(user.AuthType),
			"path":      c.Request.URL.Path,
		})

		// Check if this is a gateway key
		if user.Metadata != nil {
			metadata := user.Metadata

			keyType, _ := metadata["key_type"].(string)
			if keyType != string(KeyTypeGateway) {
				// Not a gateway key, no passthrough needed
				c.Next()
				return
			}

			// This is a gateway key, check for passthrough token
			userToken := c.GetHeader("X-User-Token")
			if userToken == "" {
				// No passthrough token provided
				c.Next()
				return
			}

			provider := c.GetHeader("X-Token-Provider")
			if provider == "" {
				s.logger.Warn("Passthrough token provided without provider", map[string]interface{}{
					"user_id": user.ID,
				})
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "X-Token-Provider header required when using X-User-Token",
				})
				c.Abort()
				return
			}

			// Extract allowed services
			allowedServices := ExtractAllowedServices(metadata)

			// Validate provider is allowed
			if !ValidateProviderAllowed(provider, allowedServices) {
				s.logger.Warn("Provider not allowed for gateway key", map[string]interface{}{
					"user_id":          user.ID,
					"provider":         provider,
					"allowed_services": allowedServices,
				})
				c.JSON(http.StatusForbidden, gin.H{
					"error": fmt.Sprintf("Provider %s not allowed for this gateway key", provider),
				})
				c.Abort()
				return
			}

			// Add passthrough token to context
			passthroughToken := PassthroughToken{
				Provider: provider,
				Token:    userToken,
			}

			ctx := WithPassthroughToken(c.Request.Context(), passthroughToken)
			ctx = WithTokenProvider(ctx, provider)

			// Add gateway ID if available
			if gatewayID, ok := metadata["gateway_id"].(string); ok {
				ctx = WithGatewayID(ctx, gatewayID)
			}

			c.Request = c.Request.WithContext(ctx)

			// Also store in Gin context for easier access
			c.Set("passthrough_token", passthroughToken)

			s.logger.Info("Passthrough token added to context", map[string]interface{}{
				"user_id":  user.ID,
				"provider": provider,
			})
		}

		c.Next()
	}
}

// StandardMiddlewareWithPassthrough returns standard HTTP middleware for authentication with passthrough support
func (s *Service) StandardMiddlewareWithPassthrough(authTypes ...Type) func(http.Handler) http.Handler {
	// Get the base middleware
	baseMiddleware := s.StandardMiddleware(authTypes...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a wrapper to intercept the response
			wrapper := &responseWrapper{
				ResponseWriter: w,
				request:        r,
				service:        s,
			}

			// Call the base middleware with our wrapper
			baseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// If we get here, authentication succeeded
				wrapper.handlePassthrough(w, r, next)
			})).ServeHTTP(wrapper, r)
		})
	}
}

// responseWrapper wraps http.ResponseWriter to intercept auth failures
type responseWrapper struct {
	http.ResponseWriter
	request       *http.Request
	service       *Service
	headerWritten bool
}

func (rw *responseWrapper) WriteHeader(code int) {
	if !rw.headerWritten {
		rw.headerWritten = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWrapper) Write(b []byte) (int, error) {
	if !rw.headerWritten {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWrapper) handlePassthrough(w http.ResponseWriter, r *http.Request, next http.Handler) {
	// Get the authenticated user
	user, exists := GetUserFromRequest(r)
	if !exists {
		// This shouldn't happen as base middleware should have set it
		next.ServeHTTP(w, r)
		return
	}

	// Check if this is a gateway key
	if user.Metadata != nil {
		metadata := user.Metadata

		keyType, _ := metadata["key_type"].(string)
		if keyType != string(KeyTypeGateway) {
			// Not a gateway key, no passthrough needed
			next.ServeHTTP(w, r)
			return
		}

		// This is a gateway key, check for passthrough token
		userToken := r.Header.Get("X-User-Token")
		if userToken == "" {
			// No passthrough token provided
			next.ServeHTTP(w, r)
			return
		}

		provider := r.Header.Get("X-Token-Provider")
		if provider == "" {
			rw.service.logger.Warn("Passthrough token provided without provider", map[string]interface{}{
				"user_id": user.ID,
			})
			http.Error(w, "X-Token-Provider header required when using X-User-Token", http.StatusBadRequest)
			return
		}

		// Extract allowed services
		allowedServices := ExtractAllowedServices(metadata)

		// Validate provider is allowed
		if !ValidateProviderAllowed(provider, allowedServices) {
			rw.service.logger.Warn("Provider not allowed for gateway key", map[string]interface{}{
				"user_id":          user.ID,
				"provider":         provider,
				"allowed_services": allowedServices,
			})
			http.Error(w, fmt.Sprintf("Provider %s not allowed for this gateway key", provider), http.StatusForbidden)
			return
		}

		// Add passthrough token to context
		passthroughToken := PassthroughToken{
			Provider: provider,
			Token:    userToken,
		}

		ctx := WithPassthroughToken(r.Context(), passthroughToken)
		ctx = WithTokenProvider(ctx, provider)

		// Add gateway ID if available
		if gatewayID, ok := metadata["gateway_id"].(string); ok {
			ctx = WithGatewayID(ctx, gatewayID)
		}

		r = r.WithContext(ctx)

		rw.service.logger.Info("Passthrough token added to context", map[string]interface{}{
			"user_id":  user.ID,
			"provider": provider,
		})
	}

	next.ServeHTTP(w, r)
}
