package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/models/relationship"
	"github.com/gorilla/mux"
	"mcp-server/internal/api/responses"
)

// RelationshipHandler handles API requests related to entity relationships
type RelationshipHandler struct {
	relationshipService relationship.Service
}

// NewRelationshipHandler creates a new handler for relationship endpoints
func NewRelationshipHandler(service relationship.Service) *RelationshipHandler {
	return &RelationshipHandler{
		relationshipService: service,
	}
}

// RegisterRoutes registers the relationship routes with the router
func (h *RelationshipHandler) RegisterRoutes(router *mux.Router) {
	// Get a relationship by ID
	router.HandleFunc("/api/v1/relationships/{id}", h.GetRelationship).Methods("GET")

	// Create a new relationship
	router.HandleFunc("/api/v1/relationships", h.CreateRelationship).Methods("POST")

	// Create a bidirectional relationship
	router.HandleFunc("/api/v1/relationships/bidirectional", h.CreateBidirectionalRelationship).Methods("POST")

	// Delete a relationship
	router.HandleFunc("/api/v1/relationships/{id}", h.DeleteRelationship).Methods("DELETE")

	// Get relationships for an entity
	router.HandleFunc("/api/v1/entities/{type}/{owner}/{repo}/{id}/relationships", h.GetEntityRelationships).Methods("GET")

	// Get related entities
	router.HandleFunc("/api/v1/entities/{type}/{owner}/{repo}/{id}/related", h.GetRelatedEntities).Methods("GET")

	// Get relationship graph
	router.HandleFunc("/api/v1/entities/{type}/{owner}/{repo}/{id}/graph", h.GetRelationshipGraph).Methods("GET")
}

// CreateRelationshipRequest represents the request body for creating a relationship
type CreateRelationshipRequest struct {
	Type      models.RelationshipType `json:"type"`
	Direction string                  `json:"direction"`
	Source    models.EntityID         `json:"source"`
	Target    models.EntityID         `json:"target"`
	Strength  float64                 `json:"strength"`
	Context   string                  `json:"context,omitempty"`
	Metadata  map[string]interface{}  `json:"metadata,omitempty"`
}

// CreateBidirectionalRequest represents the request body for creating a bidirectional relationship
type CreateBidirectionalRequest struct {
	Type     models.RelationshipType `json:"type"`
	Source   models.EntityID         `json:"source"`
	Target   models.EntityID         `json:"target"`
	Strength float64                 `json:"strength"`
	Context  string                  `json:"context,omitempty"`
	Metadata map[string]interface{}  `json:"metadata,omitempty"`
}

// GetRelationship retrieves a relationship by ID
func (h *RelationshipHandler) GetRelationship(w http.ResponseWriter, r *http.Request) {
	// Extract relationship ID from URL
	vars := mux.Vars(r)
	relationshipID := vars["id"]

	// Get the relationship
	relationship, err := h.relationshipService.GetRelationship(r.Context(), relationshipID)
	if err != nil {
		responses.WriteErrorResponse(w, http.StatusNotFound, "Relationship not found", err)
		return
	}

	// Return the relationship
	responses.WriteJSONResponse(w, http.StatusOK, relationship)
}

// CreateRelationship creates a new relationship
func (h *RelationshipHandler) CreateRelationship(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req CreateRelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responses.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate request
	if req.Type == "" {
		responses.WriteErrorResponse(w, http.StatusBadRequest, "Relationship type is required", nil)
		return
	}

	if req.Source.Type == "" || req.Source.Owner == "" || req.Source.Repo == "" || req.Source.ID == "" {
		responses.WriteErrorResponse(w, http.StatusBadRequest, "Source entity details are required", nil)
		return
	}

	if req.Target.Type == "" || req.Target.Owner == "" || req.Target.Repo == "" || req.Target.ID == "" {
		responses.WriteErrorResponse(w, http.StatusBadRequest, "Target entity details are required", nil)
		return
	}

	// Set default direction if not provided
	if req.Direction == "" {
		req.Direction = models.DirectionOutgoing
	}

	// Create the relationship
	relationship := models.NewEntityRelationship(
		req.Type,
		req.Source,
		req.Target,
		req.Direction,
		req.Strength,
	)

	// Add optional properties
	if req.Context != "" {
		relationship.WithContext(req.Context)
	}

	if req.Metadata != nil {
		relationship.WithMetadata(req.Metadata)
	}

	// Save the relationship
	err := h.relationshipService.CreateRelationship(r.Context(), relationship)
	if err != nil {
		responses.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to create relationship", err)
		return
	}

	// Return the created relationship
	responses.WriteJSONResponse(w, http.StatusCreated, relationship)
}

// CreateBidirectionalRelationship creates a bidirectional relationship
func (h *RelationshipHandler) CreateBidirectionalRelationship(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req CreateBidirectionalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responses.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate request
	if req.Type == "" {
		responses.WriteErrorResponse(w, http.StatusBadRequest, "Relationship type is required", nil)
		return
	}

	if req.Source.Type == "" || req.Source.Owner == "" || req.Source.Repo == "" || req.Source.ID == "" {
		responses.WriteErrorResponse(w, http.StatusBadRequest, "Source entity details are required", nil)
		return
	}

	if req.Target.Type == "" || req.Target.Owner == "" || req.Target.Repo == "" || req.Target.ID == "" {
		responses.WriteErrorResponse(w, http.StatusBadRequest, "Target entity details are required", nil)
		return
	}

	// Create the bidirectional relationship
	err := h.relationshipService.CreateBidirectionalRelationship(
		r.Context(),
		req.Type,
		req.Source,
		req.Target,
		req.Strength,
		req.Metadata,
	)
	if err != nil {
		responses.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to create bidirectional relationship", err)
		return
	}

	// Return success
	responses.WriteJSONResponse(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Bidirectional relationship created successfully",
	})
}

// DeleteRelationship deletes a relationship by ID
func (h *RelationshipHandler) DeleteRelationship(w http.ResponseWriter, r *http.Request) {
	// Extract relationship ID from URL
	vars := mux.Vars(r)
	relationshipID := vars["id"]

	// Delete the relationship
	err := h.relationshipService.DeleteRelationship(r.Context(), relationshipID)
	if err != nil {
		responses.WriteErrorResponse(w, http.StatusNotFound, "Failed to delete relationship", err)
		return
	}

	// Return success
	responses.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Relationship deleted successfully",
	})
}

// GetEntityRelationships retrieves relationships for an entity
func (h *RelationshipHandler) GetEntityRelationships(w http.ResponseWriter, r *http.Request) {
	// Extract entity details from URL
	vars := mux.Vars(r)
	entityType := models.EntityType(vars["type"])
	owner := vars["owner"]
	repo := vars["repo"]
	entityID := vars["id"]

	// Create the entity ID
	entity := models.NewEntityID(entityType, owner, repo, entityID)

	// Extract query parameters
	query := r.URL.Query()

	// Get direction (default: bidirectional)
	direction := query.Get("direction")
	if direction == "" {
		direction = models.DirectionBidirectional
	}

	// Get relationship types
	var relTypes []models.RelationshipType
	if typesParam := query.Get("types"); typesParam != "" {
		for _, t := range strings.Split(typesParam, ",") {
			relTypes = append(relTypes, models.RelationshipType(t))
		}
	}

	// Get relationships
	relationships, err := h.relationshipService.GetDirectRelationships(
		r.Context(),
		entity,
		direction,
		relTypes,
	)
	if err != nil {
		responses.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to get relationships", err)
		return
	}

	// Return the relationships
	responses.WriteJSONResponse(w, http.StatusOK, relationships)
}

// GetRelatedEntities retrieves entities related to the specified entity
func (h *RelationshipHandler) GetRelatedEntities(w http.ResponseWriter, r *http.Request) {
	// Extract entity details from URL
	vars := mux.Vars(r)
	entityType := models.EntityType(vars["type"])
	owner := vars["owner"]
	repo := vars["repo"]
	entityID := vars["id"]

	// Create the entity ID
	entity := models.NewEntityID(entityType, owner, repo, entityID)

	// Extract query parameters
	query := r.URL.Query()

	// Get relationship types
	var relTypes []models.RelationshipType
	if typesParam := query.Get("types"); typesParam != "" {
		for _, t := range strings.Split(typesParam, ",") {
			relTypes = append(relTypes, models.RelationshipType(t))
		}
	}

	// Get max depth (default: 1)
	maxDepth := 1
	if depthParam := query.Get("depth"); depthParam != "" {
		if parsedDepth, err := strconv.Atoi(depthParam); err == nil && parsedDepth > 0 {
			maxDepth = parsedDepth
		}
	}

	// Get related entities
	entities, err := h.relationshipService.GetRelatedEntities(
		r.Context(),
		entity,
		relTypes,
		maxDepth,
	)
	if err != nil {
		responses.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to get related entities", err)
		return
	}

	// Return the related entities
	responses.WriteJSONResponse(w, http.StatusOK, entities)
}

// GetRelationshipGraph retrieves the relationship graph for an entity
func (h *RelationshipHandler) GetRelationshipGraph(w http.ResponseWriter, r *http.Request) {
	// Extract entity details from URL
	vars := mux.Vars(r)
	entityType := models.EntityType(vars["type"])
	owner := vars["owner"]
	repo := vars["repo"]
	entityID := vars["id"]

	// Create the entity ID
	entity := models.NewEntityID(entityType, owner, repo, entityID)

	// Extract query parameters
	query := r.URL.Query()

	// Get max depth (default: 1)
	maxDepth := 1
	if depthParam := query.Get("depth"); depthParam != "" {
		if parsedDepth, err := strconv.Atoi(depthParam); err == nil && parsedDepth > 0 {
			maxDepth = parsedDepth
		}
	}

	// Get relationship graph
	graph, err := h.relationshipService.GetRelationshipGraph(
		r.Context(),
		entity,
		maxDepth,
	)
	if err != nil {
		responses.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to get relationship graph", err)
		return
	}

	// Return the relationship graph
	responses.WriteJSONResponse(w, http.StatusOK, graph)
}
