package responses

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Error   any    `json:"error,omitempty"`
}

// WriteJSONResponse writes a JSON response with the given status code and data
func WriteJSONResponse(w http.ResponseWriter, statusCode int, data any) {
	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Set status code
	w.WriteHeader(statusCode)

	// Marshal and write the response
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

// WriteErrorResponse writes a standardized error response
func WriteErrorResponse(w http.ResponseWriter, statusCode int, message string, err error) {
	// Create error response
	response := ErrorResponse{
		Status:  statusCode,
		Message: message,
	}

	// Include error details if provided
	if err != nil {
		response.Error = err.Error()
	}

	// Write the response
	WriteJSONResponse(w, statusCode, response)
}
