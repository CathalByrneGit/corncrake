package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"

	"github.com/CathalByrneGit/corncrake/internal/handlers"
	"github.com/CathalByrneGit/corncrake/internal/middleware"
	"github.com/CathalByrneGit/corncrake/internal/models"
	"github.com/CathalByrneGit/corncrake/internal/services"
	_ "github.com/CathalByrneGit/corncrake/internal/tenants/ehecs"
)

const (
	testSecret  = "ehecs-dev-secret-replace-in-production"
	testHolding = "CSO123456"
	testRunRef  = "RUN-2026-Q1-001"
	testSubID   = "550e8400-e29b-41d4-a716-446655440000"
)

func testToken(t *testing.T, holding string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"holdingNumber":   holding,
		"softwareUsed":    "TestSuite",
		"softwareVersion": "1.0.0",
		"exp":             time.Now().Add(time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newRouter(t *testing.T) (chi.Router, *services.MemoryStore) {
	t.Helper()
	store := services.NewMemoryStore()
	subH := &handlers.SubmissionHandler{Store: store}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)

	r.Get("/corncrake/v1/health", handlers.Health)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(middleware.AuthConfig{HMACSecret: []byte(testSecret)}))

		const base = "/corncrake/v1/{tenantID}/submissions/{holdingNumber}/{taxYear}/{quarter}/{runReference}"
		r.With(middleware.RequireSoftwareParams).Post(base+"/{submissionId}", subH.Create)
		r.With(middleware.RequireSoftwareParams).Get(base+"/{submissionId}", subH.GetSubmission)
		r.With(middleware.RequireSoftwareParams).Get(base, subH.GetRun)
		r.Get("/corncrake/v1/{tenantID}/submissions/{holdingNumber}", subH.ListSubmissions)
		r.Get("/corncrake/v1/lookups/occupation-codes", handlers.GetOccupationCodes)
		r.Get("/corncrake/v1/lookups/schema-version", handlers.GetSchemaVersion)
	})

	return r, store
}

func validBody() map[string]any {
	return map[string]any{
		"holdingNumber": testHolding,
		"taxYear":       2026,
		"quarter":       1,
		"returnType":    "ORIGINAL",
		"employees": []map[string]any{{
			"ppsn":           "1234567A",
			"employmentId":   "EMP001",
			"occupationCode": 4,
			"employmentType": "FULL_TIME",
			"grossEarnings":  15250.00,
			"basicPay":       14000.00,
			"overtimePay":    1250.00,
			"basicHours":     520.0,
			"overtimeHours":  25.0,
			"employerPRSI":   1967.25,
		}},
	}
}

func post(t *testing.T, r chi.Router, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func get(t *testing.T, r chi.Router, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func decode(t *testing.T, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response: %v\nbody: %s", err, rr.Body.String())
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestHealth(t *testing.T) {
	r, _ := newRouter(t)
	rr := get(t, r, "/corncrake/v1/health", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAuth_NoToken(t *testing.T) {
	r, _ := newRouter(t)
	q := "?softwareUsed=X&softwareVersion=1"
	rr := post(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/"+testSubID+q, "", validBody())
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	r, _ := newRouter(t)
	q := "?softwareUsed=X&softwareVersion=1"
	rr := post(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/"+testSubID+q,
		"not.a.valid.token", validBody())
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestCreate_Valid(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=TestSuite&softwareVersion=1.0"
	rr := post(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/"+testSubID+q,
		tok, validBody())

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d\nbody: %s", rr.Code, rr.Body.String())
	}
	var resp models.APIResponse
	decode(t, rr, &resp)
	data := resp.Data.(map[string]any)
	if data["submissionId"] != testSubID {
		t.Errorf("unexpected submissionId: %v", data["submissionId"])
	}
	if data["status"] != "RECEIVED" {
		t.Errorf("expected status RECEIVED, got %v", data["status"])
	}
}

func TestCreate_MissingSoftwareParams(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	rr := post(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/"+testSubID,
		tok, validBody())
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreate_WrongHoldingNumber(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=X&softwareVersion=1"
	rr := post(t, r, "/corncrake/v1/ehecs/submissions/WRONGHOLDER/2026/1/"+testRunRef+"/"+testSubID+q,
		tok, validBody())
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestCreate_InvalidPPSN(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=X&softwareVersion=1"
	body := validBody()
	body["employees"].([]map[string]any)[0]["ppsn"] = "BADPPSN"
	rr := post(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/new-uuid-1"+q,
		tok, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d\nbody: %s", rr.Code, rr.Body.String())
	}
	var resp models.APIResponse
	decode(t, rr, &resp)
	if len(resp.Errors) == 0 {
		t.Error("expected validation errors in response")
	}
}

func TestCreate_OvertimeInconsistency(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=X&softwareVersion=1"
	body := validBody()
	body["employees"].([]map[string]any)[0]["overtimePay"] = 500.0
	body["employees"].([]map[string]any)[0]["overtimeHours"] = 0.0
	rr := post(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/new-uuid-2"+q,
		tok, body)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d\nbody: %s", rr.Code, rr.Body.String())
	}
	var resp models.APIResponse
	decode(t, rr, &resp)
	if len(resp.Errors) == 0 || resp.Errors[0].Code != "OVERTIME_INCONSISTENCY" {
		t.Errorf("expected OVERTIME_INCONSISTENCY error, got %+v", resp.Errors)
	}
}

func TestCreate_EarningsInconsistency(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=X&softwareVersion=1"
	body := validBody()
	body["employees"].([]map[string]any)[0]["grossEarnings"] = 100.0
	body["employees"].([]map[string]any)[0]["overtimePay"] = 500.0
	rr := post(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/new-uuid-3"+q,
		tok, body)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestGetSubmission_Found(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=X&softwareVersion=1"
	// Create first
	post(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/"+testSubID+q, tok, validBody())

	// Then retrieve
	rr := get(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/"+testSubID+q, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d\nbody: %s", rr.Code, rr.Body.String())
	}
}

func TestGetSubmission_NotFound(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=X&softwareVersion=1"
	rr := get(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/unknown-id"+q, tok)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestListSubmissions(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=X&softwareVersion=1"
	post(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/"+testSubID+q, tok, validBody())

	rr := get(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp models.APIResponse
	decode(t, rr, &resp)
	if resp.Count == 0 {
		t.Error("expected at least one submission in list")
	}
}

func TestGetOccupationCodes(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	rr := get(t, r, "/corncrake/v1/lookups/occupation-codes", tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp models.APIResponse
	decode(t, rr, &resp)
	if resp.Count != 9 {
		t.Errorf("expected 9 occupation codes, got %d", resp.Count)
	}
}

func TestGetOccupationCodes_Search(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	rr := get(t, r, "/corncrake/v1/lookups/occupation-codes?search=manager", tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp models.APIResponse
	decode(t, rr, &resp)
	if resp.Count == 0 {
		t.Error("expected at least one result for search=manager")
	}
}

func TestGetSchemaVersion(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	rr := get(t, r, "/corncrake/v1/lookups/schema-version", tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCreate_HoursExceededCap(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=X&softwareVersion=1"
	body := validBody()
	body["employees"].([]map[string]any)[0]["basicHours"] = 9999.0
	rr := post(t, r, "/corncrake/v1/ehecs/submissions/"+testHolding+"/2026/1/"+testRunRef+"/new-uuid-4"+q,
		tok, body)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d\nbody: %s", rr.Code, rr.Body.String())
	}
	var resp models.APIResponse
	decode(t, rr, &resp)
	found := false
	for _, e := range resp.Errors {
		if e.Code == "HOURS_EXCEEDED" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected HOURS_EXCEEDED error, got %+v", resp.Errors)
	}
}

func TestCreate_MultipleEmployees(t *testing.T) {
	r, _ := newRouter(t)
	tok := testToken(t, testHolding)
	q := "?softwareUsed=X&softwareVersion=1"
	body := validBody()

	// Add a second employee
	employees := body["employees"].([]map[string]any)
	emp2 := map[string]any{
		"ppsn": "9876543B", "employmentId": "EMP002", "occupationCode": 2,
		"employmentType": "PART_TIME", "grossEarnings": 8000.0, "basicPay": 8000.0,
		"basicHours": 312.0, "employerPRSI": 1000.0,
	}
	body["employees"] = append(employees, emp2)

	rr := post(t, r, fmt.Sprintf("/corncrake/v1/ehecs/submissions/%s/2026/1/%s/new-uuid-5%s",
		testHolding, testRunRef, q), tok, body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d\nbody: %s", rr.Code, rr.Body.String())
	}
	var resp models.APIResponse
	decode(t, rr, &resp)
	data := resp.Data.(map[string]any)
	if data["itemCount"].(float64) != 2 {
		t.Errorf("expected itemCount=2, got %v", data["itemCount"])
	}
}
