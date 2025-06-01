package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/embedding"
	"github.com/gin-gonic/gin"
)

// SearchHandler manages vector search API endpoints
type SearchHandler struct {
	searchService embedding.SearchService
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(searchService embedding.SearchService) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
	}
}

// SearchRequest represents a vector search request
type SearchRequest struct {
	// Query text to search for
	Query string `json:"query"`
	// ContentTypes to filter by
	ContentTypes []string `json:"content_types,omitempty"`
	// Filters to apply to metadata
	Filters []embedding.SearchFilter `json:"filters,omitempty"`
	// Sorting criteria
	Sorts []embedding.SearchSort `json:"sorts,omitempty"`
	// Maximum number of results to return
	Limit int `json:"limit,omitempty"`
	// Number of results to skip (for pagination)
	Offset int `json:"offset,omitempty"`
	// Minimum similarity threshold (0.0 to 1.0)
	MinSimilarity float32 `json:"min_similarity,omitempty"`
	// Weight factors for scoring
	WeightFactors map[string]float32 `json:"weight_factors,omitempty"`
}

// SearchByVectorRequest represents a vector search request with a pre-computed vector
type SearchByVectorRequest struct {
	// Pre-computed vector to search with
	Vector []float32 `json:"vector"`
	// ContentTypes to filter by
	ContentTypes []string `json:"content_types,omitempty"`
	// Filters to apply to metadata
	Filters []embedding.SearchFilter `json:"filters,omitempty"`
	// Sorting criteria
	Sorts []embedding.SearchSort `json:"sorts,omitempty"`
	// Maximum number of results to return
	Limit int `json:"limit,omitempty"`
	// Number of results to skip (for pagination)
	Offset int `json:"offset,omitempty"`
	// Minimum similarity threshold (0.0 to 1.0)
	MinSimilarity float32 `json:"min_similarity,omitempty"`
	// Weight factors for scoring
	WeightFactors map[string]float32 `json:"weight_factors,omitempty"`
}

// SearchResponse represents the API response for search endpoints
type SearchResponse struct {
	// Results is the list of search results
	Results []*embedding.SearchResult `json:"results"`
	// Total is the total number of results found (for pagination)
	Total int `json:"total"`
	// HasMore indicates if there are more results available
	HasMore bool `json:"has_more"`
	// Query information for debugging and auditing
	Query struct {
		// Text or ContentID that was searched for
		Input string `json:"input,omitempty"`
		// Options that were used for the search
		Options *embedding.SearchOptions `json:"options,omitempty"`
	} `json:"query"`
}

// RegisterRoutes registers the search endpoints with the given router group
func (h *SearchHandler) RegisterRoutes(router *gin.RouterGroup) {
	searchGroup := router.Group("/search")
	searchGroup.POST("", h.HandleSearch)
	searchGroup.GET("", h.HandleSearch)
	searchGroup.POST("/vector", h.HandleSearchByVector)
	searchGroup.POST("/similar", h.HandleSearchSimilar)
	searchGroup.GET("/similar", h.HandleSearchSimilar)
}

// HandleSearch handles text-based vector search requests
func (h *SearchHandler) HandleSearch(c *gin.Context) {
	var searchReq SearchRequest

	// Handle both GET and POST requests
	if c.Request.Method == http.MethodPost {
		// Parse JSON request body
		if err := c.ShouldBindJSON(&searchReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
			return
		}
	} else {
		// Parse query parameters
		q := c.Request.URL.Query()

		searchReq.Query = q.Get("query")
		if types := q.Get("content_types"); types != "" {
			searchReq.ContentTypes = strings.Split(types, ",")
		}

		if limit := q.Get("limit"); limit != "" {
			if l, err := strconv.Atoi(limit); err == nil && l > 0 {
				searchReq.Limit = l
			}
		}

		if offset := q.Get("offset"); offset != "" {
			if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
				searchReq.Offset = o
			}
		}

		if minSim := q.Get("min_similarity"); minSim != "" {
			if ms, err := strconv.ParseFloat(minSim, 32); err == nil {
				searchReq.MinSimilarity = float32(ms)
			}
		}
	}

	// Validate the request
	if searchReq.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter is required"})
		return
	}

	// Prepare search options
	options := &embedding.SearchOptions{
		ContentTypes:  searchReq.ContentTypes,
		Filters:       searchReq.Filters,
		Sorts:         searchReq.Sorts,
		Limit:         searchReq.Limit,
		Offset:        searchReq.Offset,
		MinSimilarity: searchReq.MinSimilarity,
		WeightFactors: searchReq.WeightFactors,
	}

	// Perform the search
	results, err := h.searchService.Search(c.Request.Context(), searchReq.Query, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Search error: %v", err)})
		return
	}

	// Prepare the response
	response := SearchResponse{
		Results: results.Results,
		Total:   results.Total,
		HasMore: results.HasMore,
	}
	response.Query.Input = searchReq.Query
	response.Query.Options = options

	// Send the response
	c.JSON(http.StatusOK, response)
}

// HandleSearchByVector handles vector-based search requests
func (h *SearchHandler) HandleSearchByVector(c *gin.Context) {
	// Parse JSON request body
	var searchReq SearchByVectorRequest
	if err := c.ShouldBindJSON(&searchReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	// Validate the request
	if len(searchReq.Vector) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vector parameter is required"})
		return
	}

	// Prepare search options
	options := &embedding.SearchOptions{
		ContentTypes:  searchReq.ContentTypes,
		Filters:       searchReq.Filters,
		Sorts:         searchReq.Sorts,
		Limit:         searchReq.Limit,
		Offset:        searchReq.Offset,
		MinSimilarity: searchReq.MinSimilarity,
		WeightFactors: searchReq.WeightFactors,
	}

	// Perform the search
	results, err := h.searchService.SearchByVector(c.Request.Context(), searchReq.Vector, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Search error: %v", err)})
		return
	}

	// Prepare the response
	response := SearchResponse{
		Results: results.Results,
		Total:   results.Total,
		HasMore: results.HasMore,
	}
	response.Query.Input = fmt.Sprintf("vector[%d]", len(searchReq.Vector))
	response.Query.Options = options

	// Send the response
	c.JSON(http.StatusOK, response)
}

// HandleSearchSimilar handles "more like this" search requests
func (h *SearchHandler) HandleSearchSimilar(c *gin.Context) {
	var contentID string
	var options embedding.SearchOptions

	// Handle both GET and POST requests
	if c.Request.Method == http.MethodPost {
		// Parse JSON request body
		var requestBody struct {
			ContentID string                   `json:"content_id"`
			Options   *embedding.SearchOptions `json:"options,omitempty"`
		}

		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
			return
		}

		contentID = requestBody.ContentID
		if requestBody.Options != nil {
			options = *requestBody.Options
		}
	} else {
		// Parse query parameters
		q := c.Request.URL.Query()

		contentID = q.Get("content_id")

		if types := q.Get("content_types"); types != "" {
			options.ContentTypes = strings.Split(types, ",")
		}

		if limit := q.Get("limit"); limit != "" {
			if l, err := strconv.Atoi(limit); err == nil && l > 0 {
				options.Limit = l
			}
		}

		if offset := q.Get("offset"); offset != "" {
			if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
				options.Offset = o
			}
		}

		if minSim := q.Get("min_similarity"); minSim != "" {
			if ms, err := strconv.ParseFloat(minSim, 32); err == nil {
				options.MinSimilarity = float32(ms)
			}
		}
	}

	// Validate the request
	if contentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content_id parameter is required"})
		return
	}

	// Perform the search
	results, err := h.searchService.SearchByContentID(c.Request.Context(), contentID, &options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Search error: %v", err)})
		return
	}

	// Prepare the response
	response := SearchResponse{
		Results: results.Results,
		Total:   results.Total,
		HasMore: results.HasMore,
	}
	response.Query.Input = contentID
	response.Query.Options = &options

	// Send the response
	c.JSON(http.StatusOK, response)
}
