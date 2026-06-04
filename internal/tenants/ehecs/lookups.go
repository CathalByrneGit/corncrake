package ehecs

import (
	"fmt"
	"net/http"
	"strings"
)

// occupationCode is a CSO SOC-2010 broad category entry.
type occupationCode struct {
	Code         int      `json:"code"`
	Label        string   `json:"label"`
	NACEExamples []string `json:"naceExamples"`
}

// occupationCodes is the canonical SOC-2010 broad category list for EHECS.
var occupationCodes = []occupationCode{
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

var schemaVersion = struct {
	Version          string `json:"schemaVersion"`
	XMLNamespace     string `json:"xmlNamespace"`
	SpecificationURL string `json:"specificationUrl"`
}{
	Version:          "5.0.0",
	XMLNamespace:     "http://www.cso.ie/ehecs/schema/v5",
	SpecificationURL: "https://www.cso.ie/en/methods/earnings/earningsmethodologicaltechnicaldocuments",
}

// Lookup implements tenant.LookupProvider for EHECS.
// Supported names: "occupation-codes" (supports ?search=), "schema-version".
func (e *EHECS) Lookup(name string, r *http.Request) (any, int, bool) {
	switch name {
	case "occupation-codes":
		search := strings.ToLower(r.URL.Query().Get("search"))
		if search == "" {
			return occupationCodes, len(occupationCodes), true
		}
		var filtered []occupationCode
		for _, c := range occupationCodes {
			if strings.Contains(strings.ToLower(c.Label), search) ||
				fmt.Sprintf("%d", c.Code) == search {
				filtered = append(filtered, c)
			}
		}
		return filtered, len(filtered), true

	case "schema-version":
		return schemaVersion, 0, true
	}

	return nil, 0, false
}
