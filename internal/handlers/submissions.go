package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/CathalByrneGit/corncrake/internal/models"
	"github.com/CathalByrneGit/corncrake/internal/services"
	"github.com/CathalByrneGit/corncrake/internal/tenant"
)

// SubmissionHandler groups all submission endpoints.
type SubmissionHandler struct {
	Store services.SubmissionStore
}

// submissionMeta extracts common fields shared across all tenant submission bodies.
type submissionMeta struct {
	HoldingNumber string            `json:"holdingNumber"`
	ReturnType    models.ReturnType `json:"returnType"`
	ContactEmail  string            `json:"contactEmail"`
	ContactPhone  string            `json:"contactPhone"`
	Comments      string            `json:"comments"`
}

// Create handles POST /corncrake/v1/{tenantID}/submissions/{holdingNumber}/{taxYear}/{quarter}/{runReference}/{submissionId}
func (h *SubmissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
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

	// Resolve the tenant
	t := tenant.Lookup(tenantID)
	if t == nil {
		JSONError(w, http.StatusNotFound, "UNKNOWN_TENANT",
			"No survey type registered for tenantID: "+tenantID)
		return
	}

	// Read raw body
	rawBytes, err := io.ReadAll(r.Body)
	if err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to read request body: "+err.Error())
		return
	}
	raw := json.RawMessage(rawBytes)

	// Layer 1: schema validation (HTTP 400)
	if schemaErrs := t.ValidateSchema(raw); len(schemaErrs) > 0 {
		JSONErrors(w, http.StatusBadRequest, schemaErrs, nil)
		return
	}

	// Extract common meta fields (path params are authoritative for holdingNumber/taxYear/quarter)
	var meta submissionMeta
	_ = json.Unmarshal(raw, &meta)

	// Layer 2: business logic validation (HTTP 422)
	logicErrs, warnings := t.ValidateLogic(raw)
	if len(logicErrs) > 0 {
		JSONErrors(w, http.StatusUnprocessableEntity, logicErrs, warnings)
		return
	}

	// Persist
	result, err := h.Store.Create(services.CreateParams{
		TenantID:      tenantID,
		SubmissionID:  submissionID,
		RunReference:  runReference,
		HoldingNumber: holdingNumber,
		TaxYear:       taxYear,
		Quarter:       quarter,
		ReturnType:    meta.ReturnType,
		Body:          raw,
		ItemCount:     t.ItemCount(raw),
		Warnings:      warnings,
		Client:        client,
		ContactEmail:  meta.ContactEmail,
		ContactPhone:  meta.ContactPhone,
		Comments:      meta.Comments,
	})
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred.")
		return
	}

	JSON(w, http.StatusCreated, result)
}

// GetSubmission handles GET /corncrake/v1/{tenantID}/submissions/{holdingNumber}/{taxYear}/{quarter}/{runReference}/{submissionId}
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

// GetRun handles GET /corncrake/v1/{tenantID}/submissions/{holdingNumber}/{taxYear}/{quarter}/{runReference}
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

// ListSubmissions handles GET /corncrake/v1/{tenantID}/submissions/{holdingNumber}
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
