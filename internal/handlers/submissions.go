package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/CathalByrneGit/corncrake/internal/models"
	"github.com/CathalByrneGit/corncrake/internal/services"
	"github.com/CathalByrneGit/corncrake/internal/validators"
)

// SubmissionHandler groups all submission endpoints.
type SubmissionHandler struct {
	Store services.SubmissionStore
}

// Create handles POST /submissions/{holdingNumber}/{taxYear}/{quarter}/{runReference}/{submissionId}
// Mirrors: POST /paye-employers/v1/rest/payroll/{empRegNum}/{taxYear}/{runRef}/{subId}
func (h *SubmissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	holdingNumber := chi.URLParam(r, "holdingNumber")
	taxYear := mustInt(chi.URLParam(r, "taxYear"))
	quarter := mustInt(chi.URLParam(r, "quarter"))
	runReference := chi.URLParam(r, "runReference")
	submissionID := chi.URLParam(r, "submissionId")
	client := clientFromContext(r)

	// The holding number in the URL path must match the authenticated client
	if holdingNumber != client.HoldingNumber {
		JSONError(w, http.StatusForbidden, "FORBIDDEN",
			"Holding number in path does not match authenticated client credentials.")
		return
	}

	var body models.SubmissionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_JSON", "Request body is not valid JSON: "+err.Error())
		return
	}

	// Layer 1: schema validation (HTTP 400)
	if schemaErrs := validators.ValidateSubmissionRequest(&body); len(schemaErrs) > 0 {
		JSONErrors(w, http.StatusBadRequest, schemaErrs, nil)
		return
	}

	// Sync path params into body (path is authoritative for these fields)
	body.HoldingNumber = holdingNumber
	body.TaxYear = taxYear
	body.Quarter = quarter

	// Layer 2: business logic (HTTP 422) + persist
	result, err := h.Store.Create(services.CreateParams{
		SubmissionID: submissionID,
		RunReference: runReference,
		Body:         &body,
		Client:       client,
	})
	if err != nil {
		if ve, ok := err.(*services.ValidationError); ok {
			JSONErrors(w, http.StatusUnprocessableEntity, ve.Items, ve.Warnings)
			return
		}
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred.")
		return
	}

	JSON(w, http.StatusCreated, result)
}

// GetSubmission handles GET /submissions/{holdingNumber}/{taxYear}/{quarter}/{runReference}/{submissionId}
func (h *SubmissionHandler) GetSubmission(w http.ResponseWriter, r *http.Request) {
	holdingNumber := chi.URLParam(r, "holdingNumber")
	submissionID := chi.URLParam(r, "submissionId")
	client := clientFromContext(r)

	record, err := h.Store.GetByID(submissionID)
	if err != nil {
		JSONError(w, http.StatusNotFound, "NOT_FOUND",
			"Submission "+submissionID+" not found.")
		return
	}

	if record.HoldingNumber != holdingNumber || record.HoldingNumber != client.HoldingNumber {
		JSONError(w, http.StatusForbidden, "FORBIDDEN", "Access denied to this submission.")
		return
	}

	JSON(w, http.StatusOK, record)
}

// GetRun handles GET /submissions/{holdingNumber}/{taxYear}/{quarter}/{runReference}
func (h *SubmissionHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	runReference := chi.URLParam(r, "runReference")

	run, err := h.Store.GetRun(runReference)
	if err != nil {
		JSONError(w, http.StatusNotFound, "NOT_FOUND",
			"Run reference "+runReference+" not found.")
		return
	}

	JSON(w, http.StatusOK, run)
}

// ListSubmissions handles GET /submissions/{holdingNumber}
func (h *SubmissionHandler) ListSubmissions(w http.ResponseWriter, r *http.Request) {
	holdingNumber := chi.URLParam(r, "holdingNumber")
	client := clientFromContext(r)

	if holdingNumber != client.HoldingNumber {
		JSONError(w, http.StatusForbidden, "FORBIDDEN", "Access denied to this holding number.")
		return
	}

	taxYear := queryInt(r, "taxYear")
	quarter := queryInt(r, "quarter")

	results, _ := h.Store.List(holdingNumber, taxYear, quarter)
	if results == nil {
		results = []models.SubmissionSummary{}
	}

	JSONWithCount(w, http.StatusOK, results, len(results))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func mustInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func queryInt(r *http.Request, key string) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}
