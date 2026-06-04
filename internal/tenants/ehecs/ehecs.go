package ehecs

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/CathalByrneGit/corncrake/internal/models"
	"github.com/CathalByrneGit/corncrake/internal/tenant"
)

func init() {
	tenant.Register(&EHECS{})
}

// EHECS implements tenant.Tenant for the CSO Earnings, Hours and Employment Costs Survey.
type EHECS struct{}

func (e *EHECS) ID() string   { return "ehecs" }
func (e *EHECS) Name() string { return "CSO EHECS" }

var (
	ppsnRegex = regexp.MustCompile(`^[0-9]{7}[A-Z]{1,2}$`)

	validEmploymentTypes = map[EmploymentType]bool{
		EmploymentFullTime:   true,
		EmploymentPartTime:   true,
		EmploymentTrainee:    true,
		EmploymentApprentice: true,
		EmploymentOther:      true,
	}
)

func (e *EHECS) ValidateSchema(raw json.RawMessage) []models.ValidationItem {
	var body SubmissionBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return []models.ValidationItem{{Code: "INVALID_JSON", Message: "request body is not valid EHECS JSON: " + err.Error()}}
	}

	var errs []models.ValidationItem
	add := func(code, field, msg string) {
		errs = append(errs, models.ValidationItem{Code: code, Field: field, Message: msg})
	}

	if strings.TrimSpace(body.HoldingNumber) == "" {
		add("REQUIRED", "holdingNumber", "holdingNumber is required")
	}
	if body.TaxYear < 2008 || body.TaxYear > 2099 {
		add("INVALID_VALUE", "taxYear", fmt.Sprintf("taxYear must be 2008–2099, got %d", body.TaxYear))
	}
	if body.Quarter < 1 || body.Quarter > 4 {
		add("INVALID_VALUE", "quarter", fmt.Sprintf("quarter must be 1–4, got %d", body.Quarter))
	}
	if body.ReturnType != "" && body.ReturnType != "ORIGINAL" && body.ReturnType != "AMENDED" {
		add("INVALID_VALUE", "returnType", "returnType must be ORIGINAL or AMENDED")
	}
	if len(body.Employees) == 0 {
		add("REQUIRED", "employees", "at least one employee record is required")
		return errs
	}

	for i, emp := range body.Employees {
		prefix := fmt.Sprintf("employees[%d]", i)
		if !ppsnRegex.MatchString(emp.PPSN) {
			add("INVALID_FORMAT", prefix+".ppsn",
				fmt.Sprintf("%s.ppsn: must be 7 digits + 1–2 uppercase letters (e.g. 1234567A), got %q", prefix, emp.PPSN))
		}
		if strings.TrimSpace(emp.EmploymentID) == "" {
			add("REQUIRED", prefix+".employmentId", prefix+".employmentId is required")
		}
		if emp.OccupationCode < 1 || emp.OccupationCode > 9 {
			add("INVALID_VALUE", prefix+".occupationCode",
				fmt.Sprintf("%s.occupationCode: must be 1–9 (CSO SOC-2010), got %d", prefix, emp.OccupationCode))
		}
		if !validEmploymentTypes[emp.EmploymentType] {
			add("INVALID_VALUE", prefix+".employmentType",
				fmt.Sprintf("%s.employmentType: must be FULL_TIME, PART_TIME, TRAINEE, APPRENTICE, or OTHER", prefix))
		}
		for _, check := range []struct {
			v float64
			f string
		}{
			{emp.GrossEarnings, prefix + ".grossEarnings"},
			{emp.BasicPay, prefix + ".basicPay"},
			{emp.BasicHours, prefix + ".basicHours"},
			{emp.OvertimePay, prefix + ".overtimePay"},
			{emp.OvertimeHours, prefix + ".overtimeHours"},
			{emp.EmployerPRSI, prefix + ".employerPRSI"},
		} {
			if check.v < 0 {
				add("INVALID_VALUE", check.f, check.f+" must be >= 0")
			}
		}
	}
	return errs
}

func (e *EHECS) ValidateLogic(raw json.RawMessage) (errors, warnings []models.ValidationItem) {
	var body SubmissionBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return []models.ValidationItem{{Code: "INVALID_JSON", Message: err.Error()}}, nil
	}

	addErr := func(code, field, msg string) {
		errors = append(errors, models.ValidationItem{Code: code, Field: field, Message: msg})
	}
	addWarn := func(code, field, msg string) {
		warnings = append(warnings, models.ValidationItem{Code: code, Field: field, Message: msg})
	}

	for i, emp := range body.Employees {
		prefix := fmt.Sprintf("employees[%d] PPSN=%s", i, emp.PPSN)
		idx := fmt.Sprintf("employees[%d]", i)
		if emp.OvertimePay > 0 && emp.OvertimeHours == 0 {
			addErr("OVERTIME_INCONSISTENCY", idx+".overtimeHours",
				fmt.Sprintf("%s: OvertimePay > 0 but OvertimeHours = 0", prefix))
		}
		if emp.GrossEarnings < emp.OvertimePay {
			addErr("EARNINGS_INCONSISTENCY", idx+".grossEarnings",
				fmt.Sprintf("%s: GrossEarnings (%.2f) < OvertimePay (%.2f)", prefix, emp.GrossEarnings, emp.OvertimePay))
		}
		if emp.BasicHours > BasicHoursQuarterlyCap {
			addErr("HOURS_EXCEEDED", idx+".basicHours",
				fmt.Sprintf("%s: BasicHours (%.1f) exceeds quarterly cap of %.0f", prefix, emp.BasicHours, BasicHoursQuarterlyCap))
		}
		if emp.GrossEarnings == 0 {
			addWarn("ZERO_EARNINGS", idx+".grossEarnings",
				fmt.Sprintf("%s: GrossEarnings is zero — confirm this is intended", prefix))
		}
		if emp.EmployerPRSI == 0 && emp.EmploymentType == EmploymentFullTime {
			addWarn("ZERO_PRSI_FULL_TIME", idx+".employerPRSI",
				fmt.Sprintf("%s: EmployerPRSI is zero for a FULL_TIME employee", prefix))
		}
	}
	return errors, warnings
}

func (e *EHECS) ItemCount(raw json.RawMessage) int {
	var body SubmissionBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return 0
	}
	return len(body.Employees)
}
