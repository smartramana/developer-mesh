package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIVersion represents a specific API version
type APIVersion string

// Supported API versions
const (
	APIVersionUnspecified APIVersion = ""
	APIVersionV1          APIVersion = "v1"
	APIVersionV2          APIVersion = "v2"
)

// VersioningConfig holds configuration for API versioning
type VersioningConfig struct {
	DefaultVersion    APIVersion `mapstructure:"default_version"`
	AcceptHeaderCheck bool       `mapstructure:"accept_header_check"`
	URLVersioning     bool       `mapstructure:"url_versioning"`
}

// acceptHeaderRegex is a regex to extract version from Accept header
// Example: application/vnd.devops-mcp.v1+json
var acceptHeaderRegex = regexp.MustCompile(`application/vnd\.devops-mcp\.v(\d+)(\+\w+)?`)

// VersioningMiddleware adds API versioning support
func VersioningMiddleware(config VersioningConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		var version APIVersion
		
		// Check Accept header if enabled
		if config.AcceptHeaderCheck {
			accept := c.GetHeader("Accept")
			if accept != "" {
				matches := acceptHeaderRegex.FindStringSubmatch(accept)
				if len(matches) >= 2 {
					version = APIVersion("v" + matches[1])
				}
			}
		}
		
		// Check URL version if enabled and no version found yet
		if config.URLVersioning && version == APIVersionUnspecified {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api/") {
				parts := strings.Split(path, "/")
				if len(parts) >= 3 {
					if strings.HasPrefix(parts[2], "v") {
						version = APIVersion(parts[2])
					}
				}
			}
		}
		
		// Use default version if no version found
		if version == APIVersionUnspecified {
			version = config.DefaultVersion
		}
		
		// Store version in context
		c.Set("api_version", version)
		
		// Add version header to response
		c.Header("X-API-Version", string(version))
		
		c.Next()
	}
}

// GetAPIVersion returns the API version from the context
func GetAPIVersion(c *gin.Context) APIVersion {
	version, exists := c.Get("api_version")
	if !exists {
		return APIVersionV1 // Default to v1 if not set
	}
	return version.(APIVersion)
}

// RouteToVersion routes requests to the appropriate version handler
func RouteToVersion(c *gin.Context, handlers map[APIVersion]gin.HandlerFunc) {
	version := GetAPIVersion(c)
	
	// Find handler for version
	handler, exists := handlers[version]
	if !exists {
		// Try to find the default handler
		handler, exists = handlers[APIVersionUnspecified]
		if !exists {
			c.JSON(http.StatusNotAcceptable, gin.H{
				"error": "API version not supported",
				"supported_versions": getSupportedVersions(handlers),
			})
			c.Abort()
			return
		}
	}
	
	// Call handler
	handler(c)
}

// getSupportedVersions returns a list of supported versions
func getSupportedVersions(handlers map[APIVersion]gin.HandlerFunc) []string {
	versions := make([]string, 0, len(handlers))
	
	for version := range handlers {
		if version != APIVersionUnspecified {
			versions = append(versions, string(version))
		}
	}
	
	return versions
}

// VersionedHandlers holds handlers for different API versions
type VersionedHandlers struct {
	handlers map[APIVersion]gin.HandlerFunc
}

// NewVersionedHandlers creates a new versioned handlers instance
func NewVersionedHandlers() *VersionedHandlers {
	return &VersionedHandlers{
		handlers: make(map[APIVersion]gin.HandlerFunc),
	}
}

// Add adds a handler for a specific version
func (vh *VersionedHandlers) Add(version APIVersion, handler gin.HandlerFunc) *VersionedHandlers {
	vh.handlers[version] = handler
	return vh
}

// AddDefault adds a default handler
func (vh *VersionedHandlers) AddDefault(handler gin.HandlerFunc) *VersionedHandlers {
	vh.handlers[APIVersionUnspecified] = handler
	return vh
}

// Handle handles a request with the appropriate version handler
func (vh *VersionedHandlers) Handle(c *gin.Context) {
	RouteToVersion(c, vh.handlers)
}
