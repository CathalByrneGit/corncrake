package models

import (
	"encoding/json"
	"time"
)

// ReturnType distinguishes original from amended submissions.
type ReturnType string

const (
	ReturnTypeOriginal ReturnType = "ORIGINAL"
	ReturnTypeAmended  ReturnType = "AMENDED"
)

// SubmissionRecord is the full stored record including server metadata.
type SubmissionRecord struct {
	SubmissionID    string          `json:"submissionId"`
	TenantID        string          `json:"tenantId"`
	RunReference    string          `json:"runReference"`
	HoldingNumber   string          `json:"holdingNumber"`
	TaxYear         int             `json:"taxYear"`
	Quarter         int             `json:"quarter"`
	ReturnType      ReturnType      `json:"returnType"`
	Status          string          `json:"status"`
	ReceivedAt      time.Time       `json:"receivedAt"`
	SoftwareUsed    string          `json:"softwareUsed"`
	SoftwareVersion string          `json:"softwareVersion"`
	ItemCount       int             `json:"itemCount"`
	Body            json.RawMessage `json:"body"`
	Warnings        []ValidationItem `json:"warnings,omitempty"`
	ContactEmail    string          `json:"contactEmail,omitempty"`
	ContactPhone    string          `json:"contactPhone,omitempty"`
	Comments        string          `json:"comments,omitempty"`
}

// SubmissionSummary is the lightweight version used in list responses.
type SubmissionSummary struct {
	SubmissionID string     `json:"submissionId"`
	TenantID     string     `json:"tenantId"`
	RunReference string     `json:"runReference"`
	Quarter      int        `json:"quarter"`
	TaxYear      int        `json:"taxYear"`
	ReturnType   ReturnType `json:"returnType"`
	Status       string     `json:"status"`
	ReceivedAt   time.Time  `json:"receivedAt"`
	ItemCount    int        `json:"itemCount"`
}

// RunStatus groups submissions sharing a run reference.
type RunStatus struct {
	RunReference     string              `json:"runReference"`
	TotalSubmissions int                 `json:"totalSubmissions"`
	TotalItems       int                 `json:"totalItems"`
	Submissions      []SubmissionSummary `json:"submissions"`
}

// ValidationItem is a single error or warning with a machine-readable code.
type ValidationItem struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// APIResponse is the standard JSON envelope.
type APIResponse struct {
	Status   int              `json:"status"`
	Data     any              `json:"data,omitempty"`
	Count    int              `json:"count,omitempty"`
	Errors   []ValidationItem `json:"errors,omitempty"`
	Warnings []ValidationItem `json:"warnings,omitempty"`
}

// ClientContextKey is the context key for the authenticated client identity.
type ClientContextKey struct{}

// ClientIdentity holds the verified software provider identity from the JWT.
type ClientIdentity struct {
	HoldingNumber   string
	SoftwareUsed    string
	SoftwareVersion string
	AgentID         string
}
