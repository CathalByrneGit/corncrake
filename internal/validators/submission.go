// Package validators implements request validation for EHECS submissions.
// Rules are sourced from CSO Notes for Payroll Software Providers v4.0.
package validators

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/CathalByrneGit/corncrake/internal/models"
)

var ppsnRegex = regexp.MustCompile(`^[0-9]{7}[A-Z]{1,2}$`)

// ValidEmploymentTypes is the set of accepted employment type values.
var ValidEmploymentTypes = map[models.EmploymentType]bool{
	models.EmploymentFullTime:   true,
	models.EmploymentPartTime:   true,
	models.EmploymentTrainee:    true,
	models.EmploymentApprentice: true,
	models.EmploymentOther:      true,
}

// ValidateSubmissionRequest performs schema-level validation (HTTP 400).
// Returns a slice of ValidationItems; empty slice means valid.
func ValidateSubmissionRequest(req *models.SubmissionRequest) []models.ValidationItem {
	var errs []models.ValidationItem

	add := func(code, field, msg string) {
		errs = append(errs, models.ValidationItem{Code: code, Field: field, Message: msg})
	}

	// Holding number
	if strings.TrimSpace(req.HoldingNumber) == "" {
		add("REQUIRED", "holdingNumber", "holdingNumber is required")
	}

	// Tax year
	if req.TaxYear < 2008 || req.TaxYear > 2099 {
		add("INVALID_VALUE", "taxYear", fmt.Sprintf("taxYear must be between 2008 and 2099, got %d", req.TaxYear))
	}

	// Quarter
	if req.Quarter < 1 || req.Quarter > 4 {
		add("INVALID_VALUE", "quarter", fmt.Sprintf("quarter must be 1–4, got %d", req.Quarter))
	}

	// Return type
	if req.ReturnType == "" {
		req.ReturnType = models.ReturnTypeOriginal
	} else if req.ReturnType != models.ReturnTypeOriginal && req.ReturnType != models.ReturnTypeAmended {
		add("INVALID_VALUE", "returnType", "returnType must be ORIGINAL or AMENDED")
	}

	// Employees
	if len(req.Employees) == 0 {
		add("REQUIRED", "employees", "at least one employee record is required")
		return errs // no point validating individual records
	}

	for i, emp := range req.Employees {
		prefix := fmt.Sprintf("employees[%d]", i)

		if !ppsnRegex.MatchString(emp.PPSN) {
			add("INVALID_FORMAT", prefix+".ppsn",
				fmt.Sprintf("%s.ppsn: must be 7 digits followed by 1–2 uppercase letters (e.g. 1234567A), got %q", prefix, emp.PPSN))
		}

		if strings.TrimSpace(emp.EmploymentID) == "" {
			add("REQUIRED", prefix+".employmentId", prefix+".employmentId is required")
		}

		if emp.OccupationCode < 1 || emp.OccupationCode > 9 {
			add("INVALID_VALUE", prefix+".occupationCode",
				fmt.Sprintf("%s.occupationCode: must be 1–9 (CSO SOC-2010), got %d", prefix, emp.OccupationCode))
		}

		if !ValidEmploymentTypes[emp.EmploymentType] {
			add("INVALID_VALUE", prefix+".employmentType",
				fmt.Sprintf("%s.employmentType: must be one of FULL_TIME, PART_TIME, TRAINEE, APPRENTICE, OTHER", prefix))
		}

		if emp.GrossEarnings < 0 {
			add("INVALID_VALUE", prefix+".grossEarnings", prefix+".grossEarnings must be >= 0")
		}
		if emp.BasicPay < 0 {
			add("INVALID_VALUE", prefix+".basicPay", prefix+".basicPay must be >= 0")
		}
		if emp.BasicHours < 0 {
			add("INVALID_VALUE", prefix+".basicHours", prefix+".basicHours must be >= 0")
		}
		if emp.OvertimePay < 0 {
			add("INVALID_VALUE", prefix+".overtimePay", prefix+".overtimePay must be >= 0")
		}
		if emp.OvertimeHours < 0 {
			add("INVALID_VALUE", prefix+".overtimeHours", prefix+".overtimeHours must be >= 0")
		}
		if emp.EmployerPRSI < 0 {
			add("INVALID_VALUE", prefix+".employerPRSI", prefix+".employerPRSI must be >= 0")
		}
	}

	return errs
}

// ValidateSubmissionLogic performs cross-field business logic checks (HTTP 422).
// These are checks that require the data to be valid before they can be assessed.
func ValidateSubmissionLogic(req *models.SubmissionRequest) (errors, warnings []models.ValidationItem) {
	addErr := func(code, field, msg string) {
		errors = append(errors, models.ValidationItem{Code: code, Field: field, Message: msg})
	}
	addWarn := func(code, field, msg string) {
		warnings = append(warnings, models.ValidationItem{Code: code, Field: field, Message: msg})
	}

	for i, emp := range req.Employees {
		prefix := fmt.Sprintf("employees[%d] PPSN=%s", i, emp.PPSN)

		// Overtime consistency
		if emp.OvertimePay > 0 && emp.OvertimeHours == 0 {
			addErr("OVERTIME_INCONSISTENCY",
				fmt.Sprintf("employees[%d].overtimeHours", i),
				fmt.Sprintf("%s: OvertimePay > 0 but OvertimeHours = 0", prefix))
		}

		// GrossEarnings must cover OvertimePay
		if emp.GrossEarnings < emp.OvertimePay {
			addErr("EARNINGS_INCONSISTENCY",
				fmt.Sprintf("employees[%d].grossEarnings", i),
				fmt.Sprintf("%s: GrossEarnings (%.2f) < OvertimePay (%.2f)", prefix, emp.GrossEarnings, emp.OvertimePay))
		}

		// Quarterly hours cap
		if emp.BasicHours > models.BasicHoursQuarterlyCap {
			addErr("HOURS_EXCEEDED",
				fmt.Sprintf("employees[%d].basicHours", i),
				fmt.Sprintf("%s: BasicHours (%.1f) exceeds quarterly cap of %.0f", prefix, emp.BasicHours, models.BasicHoursQuarterlyCap))
		}

		// Soft warnings
		if emp.GrossEarnings == 0 {
			addWarn("ZERO_EARNINGS",
				fmt.Sprintf("employees[%d].grossEarnings", i),
				fmt.Sprintf("%s: GrossEarnings is zero — confirm this is intended", prefix))
		}

		if emp.EmployerPRSI == 0 && emp.EmploymentType == models.EmploymentFullTime {
			addWarn("ZERO_PRSI_FULL_TIME",
				fmt.Sprintf("employees[%d].employerPRSI", i),
				fmt.Sprintf("%s: EmployerPRSI is zero for a FULL_TIME employee", prefix))
		}
	}

	return errors, warnings
}
