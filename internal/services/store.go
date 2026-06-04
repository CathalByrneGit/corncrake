// Package services implements the business logic and in-memory store.
// The Store interface is designed for easy replacement with a PostgreSQL
// implementation — swap the concrete type in main.go.
package services

import (
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/CathalByrneGit/corncrake/internal/models"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// SubmissionStore is the interface a persistent backend must implement.
type SubmissionStore interface {
	Create(params CreateParams) (*CreateResult, error)
	GetByID(submissionID string) (*models.SubmissionRecord, error)
	GetRun(runReference string) (*models.RunStatus, error)
	List(holdingNumber string, taxYear, quarter int) ([]models.SubmissionSummary, error)
}

// CreateParams bundles everything needed to persist a submission.
// Validation must happen before this is called — the store only persists.
type CreateParams struct {
	TenantID      string
	SubmissionID  string
	RunReference  string
	HoldingNumber string
	TaxYear       int
	Quarter       int
	ReturnType    models.ReturnType
	Body          json.RawMessage
	ItemCount     int
	Warnings      []models.ValidationItem
	Client        *models.ClientIdentity
	ContactEmail  string
	ContactPhone  string
	Comments      string
}

// CreateResult is the immediate response after a successful submission.
type CreateResult struct {
	SubmissionID string                  `json:"submissionId"`
	RunReference string                  `json:"runReference"`
	Status       string                  `json:"status"`
	ReceivedAt   time.Time               `json:"receivedAt"`
	ItemCount    int                     `json:"itemCount"`
	Warnings     []models.ValidationItem `json:"warnings,omitempty"`
}

// ── In-memory store ───────────────────────────────────────────────────────────

// MemoryStore is a thread-safe in-memory implementation of SubmissionStore.
type MemoryStore struct {
	mu          sync.RWMutex
	submissions map[string]*models.SubmissionRecord
	runs        map[string][]string // runReference → []submissionID
}

// NewMemoryStore creates an initialised in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		submissions: make(map[string]*models.SubmissionRecord),
		runs:        make(map[string][]string),
	}
}

func (s *MemoryStore) Create(p CreateParams) (*CreateResult, error) {
	receivedAt := time.Now().UTC()

	record := &models.SubmissionRecord{
		SubmissionID:    p.SubmissionID,
		TenantID:        p.TenantID,
		RunReference:    p.RunReference,
		HoldingNumber:   p.HoldingNumber,
		TaxYear:         p.TaxYear,
		Quarter:         p.Quarter,
		ReturnType:      p.ReturnType,
		Status:          "RECEIVED",
		ReceivedAt:      receivedAt,
		SoftwareUsed:    p.Client.SoftwareUsed,
		SoftwareVersion: p.Client.SoftwareVersion,
		ItemCount:       p.ItemCount,
		Body:            p.Body,
		Warnings:        p.Warnings,
		ContactEmail:    p.ContactEmail,
		ContactPhone:    p.ContactPhone,
		Comments:        p.Comments,
	}

	s.mu.Lock()
	// Idempotent: if already stored, return the existing result unchanged.
	if existing, ok := s.submissions[p.SubmissionID]; ok {
		s.mu.Unlock()
		return &CreateResult{
			SubmissionID: existing.SubmissionID,
			RunReference: existing.RunReference,
			Status:       existing.Status,
			ReceivedAt:   existing.ReceivedAt,
			ItemCount:    existing.ItemCount,
			Warnings:     existing.Warnings,
		}, nil
	}
	s.submissions[p.SubmissionID] = record
	s.runs[p.RunReference] = append(s.runs[p.RunReference], p.SubmissionID)
	s.mu.Unlock()

	return &CreateResult{
		SubmissionID: p.SubmissionID,
		RunReference: p.RunReference,
		Status:       "RECEIVED",
		ReceivedAt:   receivedAt,
		ItemCount:    p.ItemCount,
		Warnings:     p.Warnings,
	}, nil
}

func (s *MemoryStore) GetByID(submissionID string) (*models.SubmissionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.submissions[submissionID]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

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
		run.TotalItems += r.ItemCount
		run.Submissions = append(run.Submissions, models.SubmissionSummary{
			SubmissionID: r.SubmissionID,
			TenantID:     r.TenantID,
			RunReference: r.RunReference,
			Quarter:      r.Quarter,
			TaxYear:      r.TaxYear,
			ReturnType:   r.ReturnType,
			Status:       r.Status,
			ReceivedAt:   r.ReceivedAt,
			ItemCount:    r.ItemCount,
		})
	}
	run.TotalSubmissions = len(run.Submissions)
	return run, nil
}

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
			SubmissionID: r.SubmissionID,
			TenantID:     r.TenantID,
			RunReference: r.RunReference,
			Quarter:      r.Quarter,
			TaxYear:      r.TaxYear,
			ReturnType:   r.ReturnType,
			Status:       r.Status,
			ReceivedAt:   r.ReceivedAt,
			ItemCount:    r.ItemCount,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].ReceivedAt.After(results[j].ReceivedAt)
	})
	return results, nil
}
