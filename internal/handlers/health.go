package handlers

import (
	"net/http"
	"time"
)

// Health handles GET /health — unauthenticated, used by load balancers.
func Health(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, struct {
		Service   string `json:"service"`
		Version   string `json:"version"`
		Timestamp string `json:"timestamp"`
	}{
		Service:   "Corncrake Submission API",
		Version:   "1.0.0",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
