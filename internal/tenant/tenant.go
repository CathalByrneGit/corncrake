// Package tenant defines the pluggable survey-type interface and global registry.
package tenant

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/CathalByrneGit/corncrake/internal/models"
)

// Tenant defines the per-survey-type behaviour that varies between statistical surveys.
// Implement this interface and call Register to add a new survey type at startup.
type Tenant interface {
	// ID returns the unique URL slug (e.g. "ehecs"). Used in /corncrake/v1/{tenantID}/submissions/...
	ID() string
	// Name returns a human-readable label for logging.
	Name() string
	// ValidateSchema performs structural checks (HTTP 400). Return nil/empty for valid.
	ValidateSchema(body json.RawMessage) []models.ValidationItem
	// ValidateLogic performs cross-field business rule checks (HTTP 422).
	ValidateLogic(body json.RawMessage) (errors, warnings []models.ValidationItem)
	// ItemCount extracts the number of individual records in the body (e.g. employee count).
	ItemCount(body json.RawMessage) int
}

// LookupProvider is an optional extension of Tenant for tenants that expose
// named reference datasets at GET /corncrake/v1/{tenantID}/lookups/{lookupName}.
// Implement this alongside Tenant to add lookup endpoints for a survey type.
// The request is passed so implementations can handle query params (e.g. ?search=).
// count is included in the response envelope; pass 0 to omit it (e.g. for a singleton).
type LookupProvider interface {
	Lookup(name string, r *http.Request) (data any, count int, ok bool)
}

var (
	mu       sync.RWMutex
	registry = make(map[string]Tenant)
)

// Register adds a tenant to the global registry. Panics on duplicate ID.
func Register(t Tenant) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[t.ID()]; ok {
		panic("tenant: duplicate ID registered: " + t.ID())
	}
	registry[t.ID()] = t
}

// Lookup returns the tenant for the given ID, or nil if not registered.
func Lookup(id string) Tenant {
	mu.RLock()
	defer mu.RUnlock()
	return registry[id]
}

// All returns all registered tenants.
func All() []Tenant {
	mu.RLock()
	defer mu.RUnlock()
	ts := make([]Tenant, 0, len(registry))
	for _, t := range registry {
		ts = append(ts, t)
	}
	return ts
}
