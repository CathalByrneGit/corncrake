// Package services implements the business logic and in-memory store.
// The Store interface is designed for easy replacement with a PostgreSQL
// implementation — swap the concrete type in main.go.
package services

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/CathalByrneGit/corncrake/internal/models"
	"github.com/CathalByrneGit/corncrake/internal/validators"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrValidation is returned when business logic validation fails.
var ErrValidation = errors.New("validation failed")

// SubmissionStore is the interface a persistent backend must implement.
// The in-memory implementation below satisfies it.
type SubmissionStore interface {
	Create(params CreateParams) (*CreateResult, error)
	GetByID(submissionID string) (*models.SubmissionRecord, error)
	GetRun(runReference string) (*models.RunStatus, error)
	List(holdingNumber string, taxYear, quarter int) ([]models.SubmissionSummary, error)
}

// CreateParams bundles everything needed to create a submission.
type CreateParams struct {
	SubmissionID string `json:"submissionId"`
	RunReference string
	Body         *models.SubmissionRequest
	Client       *models.ClientIdentity
}

// CreateResult is the immediate response after a successful submission.
type CreateResult struct {
	SubmissionID  string                  `json:"submissionId"`
	RunReference  string                  `json:"runReference"`
	Status        string                  `json:"status"`
	ReceivedAt    time.Time               `json:"receivedAt"`
	EmployeeCount int                     `json:"employeeCount"`
	Warnings      []models.ValidationItem `json:"warnings,omitempty"`
}

// ── In-memory store ───────────────────────────────────────────────────────────

// MemoryStore is a thread-safe in-memory implementation of SubmissionStore.
// Replace with a PostgreSQL-backed store for production.
type MemoryStore struct {
	mu          sync.RWMutex
	submissions map[string]*models.SubmissionRecord // submissionID → record
	runs        map[string][]string                 // runReference → []submissionID
}

// NewMemoryStore creates an initialised in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		submissions: make(map[string]*models.SubmissionRecord),
		runs:        make(map[string][]string),
	}
}

// Create validates and stores a new submission.
func (s *MemoryStore) Create(p CreateParams) (*CreateResult, error) {
	// Business logic validation
	logicErrs, warnings := validators.ValidateSubmissionLogic(p.Body)
	if len(logicErrs) > 0 {
		return nil, &ValidationError{Items: logicErrs, Warnings: warnings}
	}

	record := &models.SubmissionRecord{
		SubmissionID:    p.SubmissionID,
		RunReference:    p.RunReference,
		HoldingNumber:   p.Body.HoldingNumber,
		TaxYear:         p.Body.TaxYear,
		Quarter:         p.Body.Quarter,
		ReturnType:      p.Body.ReturnType,
		Status:          "RECEIVED",
		ReceivedAt:      time.Now().UTC(),
		SoftwareUsed:    p.Client.SoftwareUsed,
		SoftwareVersion: p.Client.SoftwareVersion,
		EmployeeCount:   len(p.Body.Employees),
		Employees:       p.Body.Employees,
		Warnings:        warnings,
		ContactEmail:    p.Body.ContactEmail,
		ContactPhone:    p.Body.ContactPhone,
		Comments:        p.Body.Comments,
	}

	s.mu.Lock()
	s.submissions[p.SubmissionID] = record
	s.runs[p.RunReference] = append(s.runs[p.RunReference], p.SubmissionID)
	s.mu.Unlock()

	return &CreateResult{
		SubmissionID:  p.SubmissionID,
		RunReference:  p.RunReference,
		Status:        "RECEIVED",
		ReceivedAt:    record.ReceivedAt,
		EmployeeCount: record.EmployeeCount,
		Warnings:      warnings,
	}, nil
}

// GetByID retrieves a full submission record.
func (s *MemoryStore) GetByID(submissionID string) (*models.SubmissionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.submissions[submissionID]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

// GetRun returns aggregated status for all submissions under a run reference.
func (s *MemoryStore) GetRun(runReference string) (*models.RunStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids, ok := s.runs[runReference]
	if !ok {
		return nil, ErrNotFound
	}
	run := &models.RunStatus{RunReference: runReference}
	for _, id := range ids {
		r := s.submissions[id]
		run.TotalEmployees += r.EmployeeCount
		run.Submissions = append(run.Submissions, models.SubmissionSummary{
			SubmissionID:  r.SubmissionID,
			RunReference:  r.RunReference,
			Quarter:       r.Quarter,
			TaxYear:       r.TaxYear,
			ReturnType:    r.ReturnType,
			Status:        r.Status,
			ReceivedAt:    r.ReceivedAt,
			EmployeeCount: r.EmployeeCount,
		})
	}
	run.TotalSubmissions = len(run.Submissions)
	return run, nil
}

// List returns all submissions for a holding number, optionally filtered by year/quarter.
func (s *MemoryStore) List(holdingNumber string, taxYear, quarter int) ([]models.SubmissionSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var results []models.SubmissionSummary
	for _, r := range s.submissions {
		if r.HoldingNumber != holdingNumber {
			continue
		}
		if taxYear > 0 && r.TaxYear != taxYear {
			continue
		}
		if quarter > 0 && r.Quarter != quarter {
			continue
		}
		results = append(results, models.SubmissionSummary{
			SubmissionID:  r.SubmissionID,
			RunReference:  r.RunReference,
			Quarter:       r.Quarter,
			TaxYear:       r.TaxYear,
			ReturnType:    r.ReturnType,
			Status:        r.Status,
			ReceivedAt:    r.ReceivedAt,
			EmployeeCount: r.EmployeeCount,
		})
	}
	// Stable sort newest first
	sort.Slice(results, func(i, j int) bool {
		return results[i].ReceivedAt.After(results[j].ReceivedAt)
	})
	return results, nil
}

// ── ValidationError ───────────────────────────────────────────────────────────

// ValidationError carries both errors and warnings from business logic checks.
type ValidationError struct {
	Items    []models.ValidationItem
	Warnings []models.ValidationItem
}

func (e *ValidationError) Error() string { return ErrValidation.Error() }
