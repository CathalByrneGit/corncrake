package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OccupationCodes is the CSO SOC-2010 broad categories lookup.
// Embedded as a static slice — no database needed for reference data.
var OccupationCodes = []struct {
	Code         int      `json:"code"`
	Label        string   `json:"label"`
	NACEExamples []string `json:"naceExamples"`
}{
	{1, "Managers, Directors & Senior Officials", []string{"M", "K", "O"}},
	{2, "Professional Occupations", []string{"P", "Q", "M"}},
	{3, "Associate Professional & Technical", []string{"J", "M", "K"}},
	{4, "Administrative & Secretarial", []string{"O", "K", "N"}},
	{5, "Skilled Trades Occupations", []string{"C", "F", "G"}},
	{6, "Caring, Leisure & Service", []string{"Q", "I", "R"}},
	{7, "Sales & Customer Service", []string{"G", "I", "N"}},
	{8, "Process, Plant & Machine Operatives", []string{"C", "F", "H"}},
	{9, "Elementary Occupations", []string{"I", "N", "H"}},
}

// GetOccupationCodes handles GET /lookups/occupation-codes
func GetOccupationCodes(w http.ResponseWriter, r *http.Request) {
	search := strings.ToLower(r.URL.Query().Get("search"))
	if search == "" {
		JSONWithCount(w, http.StatusOK, OccupationCodes, len(OccupationCodes))
		return
	}

	var filtered []struct {
		Code         int      `json:"code"`
		Label        string   `json:"label"`
		NACEExamples []string `json:"naceExamples"`
	}
	for _, c := range OccupationCodes {
		if strings.Contains(strings.ToLower(c.Label), search) ||
			fmt.Sprintf("%d", c.Code) == search {
			filtered = append(filtered, c)
		}
	}

	JSONWithCount(w, http.StatusOK, filtered, len(filtered))
}

// GetPeriods handles GET /lookups/periods/{holdingNumber}
func GetPeriods(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	currentYear := now.Year()
	currentQ    := int((now.Month()-1)/3) + 1

	type period struct {
		TaxYear  int    `json:"taxYear"`
		Quarter  int    `json:"quarter"`
		Label    string `json:"label"`
		Deadline string `json:"deadline"`
		Status   string `json:"status"`
	}

	var periods []period
	y, q := currentYear, currentQ
	for i := 0; i < 5; i++ {
		deadline := time.Date(y, time.Month(q*3+1), 14, 0, 0, 0, 0, time.UTC)
		periods = append(periods, period{
			TaxYear:  y,
			Quarter:  q,
			Label:    fmt.Sprintf("Q%d %d", q, y),
			Deadline: deadline.Format("2006-01-02"),
			Status:   "OPEN",
		})
		q--
		if q < 1 {
			q = 4
			y--
		}
	}

	JSONWithCount(w, http.StatusOK, periods, len(periods))
}

// GetSchemaVersion handles GET /lookups/schema-version
func GetSchemaVersion(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, struct {
		Version          string `json:"schemaVersion"`
		XMLNamespace     string `json:"xmlNamespace"`
		SpecificationURL string `json:"specificationUrl"`
	}{
		Version:          "5.0.0",
		XMLNamespace:     "http://www.cso.ie/ehecs/schema/v5",
		SpecificationURL: "https://www.cso.ie/en/methods/earnings/earningsmethodologicaltechnicaldocuments",
	})
}
