package context

import (
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// ContextResponse wraps a context with HATEOAS links for REST API responses
type ContextResponse struct {
	*models.Context
	Links map[string]string `json:"_links,omitempty"`
}

// NewContextResponse creates a context response with HATEOAS links
func NewContextResponse(ctx *models.Context, baseURL string) *ContextResponse {
	if ctx == nil {
		return nil
	}

	response := &ContextResponse{
		Context: ctx,
		Links:   make(map[string]string),
	}

	// Add HATEOAS links
	response.Links["self"] = baseURL + "/api/v1/contexts/" + ctx.ID
	response.Links["summary"] = baseURL + "/api/v1/contexts/" + ctx.ID + "/summary"
	response.Links["search"] = baseURL + "/api/v1/contexts/" + ctx.ID + "/search"

	return response
}

// ContextListResponse wraps a list of contexts with HATEOAS links
type ContextListResponse struct {
	Contexts []ContextResponse `json:"contexts"`
	Links    map[string]string `json:"_links,omitempty"`
	Meta     *ListMetadata     `json:"_meta,omitempty"`
}

// ListMetadata contains pagination and other metadata for list responses
type ListMetadata struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalPages int `json:"total_pages"`
}

// NewContextListResponse creates a list response with HATEOAS links
func NewContextListResponse(contexts []*models.Context, baseURL string, page, perPage, total int) *ContextListResponse {
	response := &ContextListResponse{
		Contexts: make([]ContextResponse, 0, len(contexts)),
		Links:    make(map[string]string),
	}

	// Convert contexts to responses
	for _, ctx := range contexts {
		if ctx != nil {
			ctxResponse := ContextResponse{
				Context: ctx,
				Links:   make(map[string]string),
			}
			ctxResponse.Links["self"] = baseURL + "/api/v1/contexts/" + ctx.ID
			ctxResponse.Links["summary"] = baseURL + "/api/v1/contexts/" + ctx.ID + "/summary"
			ctxResponse.Links["search"] = baseURL + "/api/v1/contexts/" + ctx.ID + "/search"
			response.Contexts = append(response.Contexts, ctxResponse)
		}
	}

	// Add pagination metadata
	totalPages := (total + perPage - 1) / perPage
	response.Meta = &ListMetadata{
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}

	// Add HATEOAS links for pagination
	response.Links["self"] = fmt.Sprintf("%s/api/v1/contexts?page=%d", baseURL, page)
	if page > 1 {
		response.Links["first"] = baseURL + "/api/v1/contexts?page=1"
		response.Links["prev"] = fmt.Sprintf("%s/api/v1/contexts?page=%d", baseURL, page-1)
	}
	if page < totalPages {
		response.Links["next"] = fmt.Sprintf("%s/api/v1/contexts?page=%d", baseURL, page+1)
		response.Links["last"] = fmt.Sprintf("%s/api/v1/contexts?page=%d", baseURL, totalPages)
	}

	return response
}
