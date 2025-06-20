package proxies

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EmbeddingProxy handles MCP embedding requests and forwards them to the REST API
type EmbeddingProxy struct {
	restAPIURL string
	httpClient *http.Client
	logger     observability.Logger
}

// NewEmbeddingProxy creates a new embedding proxy
func NewEmbeddingProxy(restAPIURL string, logger observability.Logger) *EmbeddingProxy {
	return &EmbeddingProxy{
		restAPIURL: restAPIURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// RegisterRoutes registers all embedding proxy routes
func (p *EmbeddingProxy) RegisterRoutes(router *gin.RouterGroup) {
	embeddings := router.Group("/embeddings")
	{
		// Core operations
		embeddings.POST("/generate", p.generateEmbedding)
		embeddings.POST("/batch", p.batchGenerateEmbeddings)
		embeddings.POST("/search", p.searchEmbeddings)
		embeddings.POST("/search/cross-model", p.crossModelSearch)

		// Provider health
		embeddings.GET("/providers/health", p.getProviderHealth)

		// Agent configuration
		agents := embeddings.Group("/agents")
		{
			agents.POST("", p.createAgentConfig)
			agents.GET("/:agentId", p.getAgentConfig)
			agents.PUT("/:agentId", p.updateAgentConfig)
			agents.GET("/:agentId/models", p.getAgentModels)
			agents.GET("/:agentId/costs", p.getAgentCosts)
		}
	}
}

// generateEmbedding proxies POST /embeddings/generate
func (p *EmbeddingProxy) generateEmbedding(c *gin.Context) {
	p.proxyRequest(c, "/api/v1/embeddings", "POST")
}

// batchGenerateEmbeddings proxies POST /embeddings/batch
func (p *EmbeddingProxy) batchGenerateEmbeddings(c *gin.Context) {
	p.proxyRequest(c, "/api/v1/embeddings/batch", "POST")
}

// searchEmbeddings proxies POST /embeddings/search
func (p *EmbeddingProxy) searchEmbeddings(c *gin.Context) {
	p.proxyRequest(c, "/api/v1/embeddings/search", "POST")
}

// crossModelSearch proxies POST /embeddings/search/cross-model
func (p *EmbeddingProxy) crossModelSearch(c *gin.Context) {
	p.proxyRequest(c, "/api/v1/embeddings/search/cross-model", "POST")
}

// getProviderHealth proxies GET /embeddings/providers/health
func (p *EmbeddingProxy) getProviderHealth(c *gin.Context) {
	p.proxyRequest(c, "/api/v1/embeddings/providers/health", "GET")
}

// createAgentConfig proxies POST /embeddings/agents
func (p *EmbeddingProxy) createAgentConfig(c *gin.Context) {
	p.proxyRequest(c, "/api/v1/embeddings/agents", "POST")
}

// getAgentConfig proxies GET /embeddings/agents/:agentId
func (p *EmbeddingProxy) getAgentConfig(c *gin.Context) {
	agentID := c.Param("agentId")
	p.proxyRequest(c, fmt.Sprintf("/api/v1/embeddings/agents/%s", agentID), "GET")
}

// updateAgentConfig proxies PUT /embeddings/agents/:agentId
func (p *EmbeddingProxy) updateAgentConfig(c *gin.Context) {
	agentID := c.Param("agentId")
	p.proxyRequest(c, fmt.Sprintf("/api/v1/embeddings/agents/%s", agentID), "PUT")
}

// getAgentModels proxies GET /embeddings/agents/:agentId/models
func (p *EmbeddingProxy) getAgentModels(c *gin.Context) {
	agentID := c.Param("agentId")
	p.proxyRequest(c, fmt.Sprintf("/api/v1/embeddings/agents/%s/models", agentID), "GET")
}

// getAgentCosts proxies GET /embeddings/agents/:agentId/costs
func (p *EmbeddingProxy) getAgentCosts(c *gin.Context) {
	agentID := c.Param("agentId")
	p.proxyRequest(c, fmt.Sprintf("/api/v1/embeddings/agents/%s/costs", agentID), "GET")
}

// proxyRequest forwards requests to the REST API
func (p *EmbeddingProxy) proxyRequest(c *gin.Context, path string, method string) {
	// Build target URL
	targetURL := p.restAPIURL + path

	// Add query parameters
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	// Read request body
	var body []byte
	var err error
	if c.Request.Body != nil {
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			p.logger.Error("Failed to read request body", map[string]any{
				"error": err.Error(),
				"path":  path,
			})
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(c.Request.Context(), method, targetURL, bytes.NewReader(body))
	if err != nil {
		p.logger.Error("Failed to create proxy request", map[string]any{
			"error": err.Error(),
			"path":  path,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Forward headers
	for key, values := range c.Request.Header {
		// Skip hop-by-hop headers
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Inject tenant ID if available
	if tenantID, exists := c.Get("tenant_id"); exists {
		req.Header.Set("X-Tenant-ID", tenantID.(uuid.UUID).String())
	}

	// Add request ID for tracing
	requestID := uuid.New().String()
	req.Header.Set("X-Request-ID", requestID)

	// Log proxy request
	p.logger.Debug("Proxying embedding request", map[string]any{
		"method":     method,
		"path":       path,
		"target_url": sanitizeLogValue(targetURL),
		"request_id": requestID,
	})

	// Execute request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.logger.Error("Proxy request failed", map[string]any{
			"error":      err.Error(),
			"path":       path,
			"request_id": requestID,
		})
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service unavailable"})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		p.logger.Error("Failed to read response body", map[string]any{
			"error":      err.Error(),
			"path":       path,
			"request_id": requestID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	// Forward response headers
	for key, values := range resp.Header {
		// Skip hop-by-hop headers
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// Forward status code and body
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}

// isHopByHopHeader checks if a header is hop-by-hop
func isHopByHopHeader(header string) bool {
	hopByHopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"TE",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, h := range hopByHopHeaders {
		if header == h {
			return true
		}
	}
	return false
}

// EmbeddingRequest represents a request to generate embeddings
type EmbeddingRequest struct {
	AgentID   string                 `json:"agent_id" validate:"required"`
	Text      string                 `json:"text" validate:"required,max=50000"`
	TaskType  string                 `json:"task_type,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// EmbeddingResponse represents the response from embedding generation
type EmbeddingResponse struct {
	EmbeddingID          uuid.UUID              `json:"embedding_id"`
	RequestID            string                 `json:"request_id"`
	ModelUsed            string                 `json:"model_used"`
	Provider             string                 `json:"provider"`
	Dimensions           int                    `json:"dimensions"`
	NormalizedDimensions int                    `json:"normalized_dimensions"`
	CostUSD              float64                `json:"cost_usd"`
	TokensUsed           int                    `json:"tokens_used"`
	GenerationTimeMs     int64                  `json:"generation_time_ms"`
	Cached               bool                   `json:"cached"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
}

// SearchRequest represents a search request
type SearchRequest struct {
	Query          string                 `json:"query,omitempty"`
	QueryEmbedding []float32              `json:"query_embedding,omitempty"`
	AgentID        string                 `json:"agent_id" validate:"required"`
	Limit          int                    `json:"limit,omitempty"`
	MinSimilarity  float64                `json:"min_similarity,omitempty"`
	MetadataFilter map[string]interface{} `json:"metadata_filter,omitempty"`
}

// SearchResult represents a search result
type SearchResult struct {
	ID         uuid.UUID              `json:"id"`
	Content    string                 `json:"content"`
	Similarity float64                `json:"similarity"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// sanitizeLogValue removes newlines and carriage returns from user input to prevent log injection
func sanitizeLogValue(input string) string {
	// Remove newlines, carriage returns, and other control characters
	sanitized := strings.ReplaceAll(input, "\n", "\\n")
	sanitized = strings.ReplaceAll(sanitized, "\r", "\\r")
	sanitized = strings.ReplaceAll(sanitized, "\t", "\\t")
	// Limit length to prevent excessive log sizes
	if len(sanitized) > 100 {
		sanitized = sanitized[:100] + "..."
	}
	return sanitized
}
