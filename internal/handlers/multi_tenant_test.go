package handlers_test

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/CathalByrneGit/corncrake/internal/models"
	"github.com/CathalByrneGit/corncrake/internal/tenant"
)

// stubTenant accepts any JSON body without validation.
// Used to prove the submission pipeline is tenant-agnostic.
type stubTenant struct{}

func (s *stubTenant) ID() string   { return "stub" }
func (s *stubTenant) Name() string { return "Stub Survey" }
func (s *stubTenant) ValidateSchema(_ json.RawMessage) []models.ValidationItem {
	return nil
}
func (s *stubTenant) ValidateLogic(_ json.RawMessage) ([]models.ValidationItem, []models.ValidationItem) {
	return nil, nil
}
func (s *stubTenant) ItemCount(_ json.RawMessage) int { return 1 }

// stubTenant intentionally does NOT implement tenant.LookupProvider.

// TestMain registers the stub tenant once for the entire test binary.
// The EHECS tenant is registered earlier via its init() (blank import at the top of submissions_test.go).
func TestMain(m *testing.M) {
	tenant.Register(&stubTenant{})
	os.Exit(m.Run())
}

// TestStubTenant_Create_Valid proves the generic submission pipeline accepts any body
// when the tenant imposes no schema constraints.
func TestStubTenant_Create_Valid(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=StubSW&softwareVersion=1.0"
	rr := post(t, r,
		"/corncrake/v1/stub/submissions/"+testHolding+"/2026/1/STUB-RUN/STUB-SUB-001"+q,
		tok, map[string]any{"anything": "goes"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d\nbody: %s", rr.Code, rr.Body.String())
	}
	var resp models.APIResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	data := resp.Data.(map[string]any)
	if data["itemCount"].(float64) != 1 {
		t.Errorf("expected itemCount=1 from stub tenant, got %v", data["itemCount"])
	}
	if data["status"] != "RECEIVED" {
		t.Errorf("expected status RECEIVED, got %v", data["status"])
	}
}

// TestUnknownTenant_Returns404 proves the handler rejects an unregistered tenant slug.
func TestUnknownTenant_Returns404(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=X&softwareVersion=1"
	rr := post(t, r,
		"/corncrake/v1/nonexistent/submissions/"+testHolding+"/2026/1/RUN/SUB"+q,
		tok, map[string]any{})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unregistered tenant, got %d", rr.Code)
	}
}

// TestEHECSValidation_RejectsInvalidStubBody proves EHECS validation rejects
// a body that would pass the stub tenant but violates EHECS schema rules.
func TestEHECSValidation_RejectsInvalidStubBody(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=X&softwareVersion=1"
	rr := post(t, r,
		"/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/STUB-SUB-2"+q,
		tok, map[string]any{"anything": "goes"})
	// EHECS requires at least one employee — generic body has none
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (EHECS schema rejection), got %d\nbody: %s", rr.Code, rr.Body.String())
	}
}

// TestLookup_EHECSHasOccupationCodes proves EHECS implements LookupProvider
// and returns the 9 SOC-2010 categories.
func TestLookup_EHECSHasOccupationCodes(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	rr := get(t, r, "/corncrake/v1/ehecs/lookups/occupation-codes", tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d\nbody: %s", rr.Code, rr.Body.String())
	}
	var resp models.APIResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 9 {
		t.Errorf("expected 9 occupation codes, got %d", resp.Count)
	}
}

// TestLookup_StubTenantHasNoLookups proves a tenant that doesn't implement
// LookupProvider gets a 404 on lookup routes rather than EHECS data.
func TestLookup_StubTenantHasNoLookups(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	rr := get(t, r, "/corncrake/v1/stub/lookups/occupation-codes", tok)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for stub tenant lookup, got %d", rr.Code)
	}
}

// TestLookup_UnknownTenantReturns404 proves the lookup route also validates the tenant slug.
func TestLookup_UnknownTenantReturns404(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	rr := get(t, r, "/corncrake/v1/ghost/lookups/occupation-codes", tok)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown tenant, got %d", rr.Code)
	}
}
