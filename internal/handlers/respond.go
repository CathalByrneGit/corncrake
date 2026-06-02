package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/CathalByrneGit/corncrake/internal/models"
)

// JSON writes a standard API response envelope.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(models.APIResponse{Status: status, Data: data})
}

// JSONWithCount writes a list response with a top-level count field.
func JSONWithCount(w http.ResponseWriter, status int, data any, count int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(models.APIResponse{Status: status, Data: data, Count: count})
}

// JSONErrors writes a validation error response (HTTP 400 or 422).
func JSONErrors(w http.ResponseWriter, status int, errs []models.ValidationItem, warnings []models.ValidationItem) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(models.APIResponse{
		Status:   status,
		Errors:   errs,
		Warnings: warnings,
	})
}

// JSONError writes a single error response.
func JSONError(w http.ResponseWriter, status int, code, message string) {
	JSONErrors(w, status, []models.ValidationItem{{Code: code, Message: message}}, nil)
}

// clientFromContext extracts the authenticated client from the request context.
func clientFromContext(r *http.Request) *models.ClientIdentity {
	if c, ok := r.Context().Value(models.ClientContextKey{}).(*models.ClientIdentity); ok {
		return c
	}
	return &models.ClientIdentity{}
}
