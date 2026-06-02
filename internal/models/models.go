// Package models defines the domain types for the EHECS API.
// Field names and constraints follow CSO Notes for Payroll Software Providers v4.0
// and the EHECS XML Schema v5.
package models

import "time"

// ReturnType distinguishes original from amended submissions.
type ReturnType string

const (
	ReturnTypeOriginal ReturnType = "ORIGINAL"
	ReturnTypeAmended  ReturnType = "AMENDED"
)

// EmploymentType maps to CSO employment classification.
type EmploymentType string

const (
	EmploymentFullTime   EmploymentType = "FULL_TIME"
	EmploymentPartTime   EmploymentType = "PART_TIME"
	EmploymentTrainee    EmploymentType = "TRAINEE"
	EmploymentApprentice EmploymentType = "APPRENTICE"
	EmploymentOther      EmploymentType = "OTHER"
)

// BasicHoursQuarterlyCap is the maximum hours a single employee can report
// in one quarter (168 hrs/week × 13 weeks).
const BasicHoursQuarterlyCap = 2184.0

// Employee holds payroll data for a single employee in a quarterly return.
type Employee struct {
	PPSN           string         `json:"ppsn"`
	EmploymentID   string         `json:"employmentId"`
	OccupationCode int            `json:"occupationCode"` // CSO SOC-2010 broad category 1–9
	EmploymentType EmploymentType `json:"employmentType"`
	GrossEarnings  float64        `json:"grossEarnings"`
	BasicPay       float64        `json:"basicPay"`
	OvertimePay    float64        `json:"overtimePay"`
	BasicHours     float64        `json:"basicHours"`
	OvertimeHours  float64        `json:"overtimeHours"`
	EmployerPRSI   float64        `json:"employerPRSI"`
	Bonuses        float64        `json:"bonuses"`
	Allowances     float64        `json:"allowances"`
	ShiftPremiums  float64        `json:"shiftPremiums"`
	OtherSubsidies float64        `json:"otherSubsidies"`
}

// SubmissionRequest is the POST body for creating a submission.
type SubmissionRequest struct {
	HoldingNumber string     `json:"holdingNumber"`
	TaxYear       int        `json:"taxYear"`
	Quarter       int        `json:"quarter"`
	ReturnType    ReturnType `json:"returnType"`
	ContactEmail  string     `json:"contactEmail,omitempty"`
	ContactPhone  string     `json:"contactPhone,omitempty"`
	Comments      string     `json:"comments,omitempty"`
	Employees     []Employee `json:"employees"`
}

// SubmissionRecord is the full stored record including server metadata.
type SubmissionRecord struct {
	SubmissionID    string           `json:"submissionId"`
	RunReference    string           `json:"runReference"`
	HoldingNumber   string           `json:"holdingNumber"`
	TaxYear         int              `json:"taxYear"`
	Quarter         int              `json:"quarter"`
	ReturnType      ReturnType       `json:"returnType"`
	Status          string           `json:"status"`
	ReceivedAt      time.Time        `json:"receivedAt"`
	SoftwareUsed    string           `json:"softwareUsed"`
	SoftwareVersion string           `json:"softwareVersion"`
	EmployeeCount   int              `json:"employeeCount"`
	Employees       []Employee       `json:"employees,omitempty"`
	Warnings        []ValidationItem `json:"warnings,omitempty"`
	ContactEmail    string           `json:"contactEmail,omitempty"`
	ContactPhone    string           `json:"contactPhone,omitempty"`
	Comments        string           `json:"comments,omitempty"`
}

// SubmissionSummary is the lightweight version used in list responses.
type SubmissionSummary struct {
	SubmissionID  string     `json:"submissionId"`
	RunReference  string     `json:"runReference"`
	Quarter       int        `json:"quarter"`
	TaxYear       int        `json:"taxYear"`
	ReturnType    ReturnType `json:"returnType"`
	Status        string     `json:"status"`
	ReceivedAt    time.Time  `json:"receivedAt"`
	EmployeeCount int        `json:"employeeCount"`
}

// RunStatus groups submissions sharing a run reference.
type RunStatus struct {
	RunReference     string              `json:"runReference"`
	TotalSubmissions int                 `json:"totalSubmissions"`
	TotalEmployees   int                 `json:"totalEmployees"`
	Submissions      []SubmissionSummary `json:"submissions"`
}

// ValidationItem is a single error or warning with a machine-readable code.
type ValidationItem struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// APIResponse is the standard JSON envelope — consistent with Revenue's pattern.
type APIResponse struct {
	Status   int              `json:"status"`
	Data     any              `json:"data,omitempty"`
	Count    int              `json:"count,omitempty"`
	Errors   []ValidationItem `json:"errors,omitempty"`
	Warnings []ValidationItem `json:"warnings,omitempty"`
}

// OccupationCode is a CSO SOC-2010 broad category entry.
type OccupationCode struct {
	Code         int      `json:"code"`
	Label        string   `json:"label"`
	NACEExamples []string `json:"naceExamples"`
}

// ReportingPeriod describes an available quarter.
type ReportingPeriod struct {
	TaxYear  int    `json:"taxYear"`
	Quarter  int    `json:"quarter"`
	Label    string `json:"label"`
	Deadline string `json:"deadline"`
	Status   string `json:"status"`
}

// SchemaVersion describes the current EHECS XML schema target.
type SchemaVersion struct {
	Version          string `json:"schemaVersion"`
	XMLNamespace     string `json:"xmlNamespace"`
	SpecificationURL string `json:"specificationUrl"`
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
