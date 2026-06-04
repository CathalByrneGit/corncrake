package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/CathalByrneGit/corncrake/internal/tenant"
)

// GetLookup handles GET /corncrake/v1/{tenantID}/lookups/{lookupName}
// Dispatches to the tenant's LookupProvider. Returns 404 for unknown tenants,
// tenants that don't implement LookupProvider, or unknown lookup names.
func GetLookup(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	name := chi.URLParam(r, "lookupName")

	t := tenant.Lookup(tenantID)
	if t == nil {
		JSONError(w, http.StatusNotFound, "UNKNOWN_TENANT",
			"No tenant registered with ID: "+tenantID)
		return
	}

	lp, ok := t.(tenant.LookupProvider)
	if !ok {
		JSONError(w, http.StatusNotFound, "NO_LOOKUPS",
			"Tenant "+tenantID+" does not provide lookup data.")
		return
	}

	data, count, found := lp.Lookup(name, r)
	if !found {
		JSONError(w, http.StatusNotFound, "UNKNOWN_LOOKUP",
			"No lookup '"+name+"' for tenant "+tenantID)
		return
	}

	JSONWithCount(w, http.StatusOK, data, count)
}

// GetPeriods handles GET /corncrake/v1/{tenantID}/lookups/periods/{holdingNumber}
// Returns the five most recent open quarterly reporting periods.
// Period logic is survey-agnostic — any tenant using quarterly periods can use this endpoint.
func GetPeriods(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	currentYear := now.Year()
	currentQ := int((now.Month()-1)/3) + 1

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
