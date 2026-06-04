package ehecs

import "github.com/CathalByrneGit/corncrake/internal/models"

// EmploymentType maps to CSO employment classification.
type EmploymentType string

const (
	EmploymentFullTime   EmploymentType = "FULL_TIME"
	EmploymentPartTime   EmploymentType = "PART_TIME"
	EmploymentTrainee    EmploymentType = "TRAINEE"
	EmploymentApprentice EmploymentType = "APPRENTICE"
	EmploymentOther      EmploymentType = "OTHER"
)

// BasicHoursQuarterlyCap is the maximum hours a single employee can report per quarter (168h × 13w).
const BasicHoursQuarterlyCap = 2184.0

// Employee holds payroll data for a single employee in a quarterly return.
type Employee struct {
	PPSN           string         `json:"ppsn"`
	EmploymentID   string         `json:"employmentId"`
	OccupationCode int            `json:"occupationCode"`
	EmploymentType EmploymentType `json:"employmentType"`
	GrossEarnings  float64        `json:"grossEarnings"`
	BasicPay       float64        `json:"basicPay"`
	OvertimePay    float64        `json:"overtimePay"`
	BasicHours     float64        `json:"basicHours"`
	OvertimeHours  float64        `json:"overtimeHours"`
	EmployerPRSI   float64        `json:"employerPRSI"`
	Bonuses        float64        `json:"bonuses,omitempty"`
	Allowances     float64        `json:"allowances,omitempty"`
	ShiftPremiums  float64        `json:"shiftPremiums,omitempty"`
	OtherSubsidies float64        `json:"otherSubsidies,omitempty"`
}

// SubmissionBody is the EHECS-specific POST body.
type SubmissionBody struct {
	HoldingNumber string            `json:"holdingNumber"`
	TaxYear       int               `json:"taxYear"`
	Quarter       int               `json:"quarter"`
	ReturnType    models.ReturnType `json:"returnType"`
	Employees     []Employee        `json:"employees"`
	ContactEmail  string            `json:"contactEmail,omitempty"`
	ContactPhone  string            `json:"contactPhone,omitempty"`
	Comments      string            `json:"comments,omitempty"`
}
