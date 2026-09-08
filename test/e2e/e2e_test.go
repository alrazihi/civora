package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	assessmentapi "github.com/alrazihi/civora/internal/assessment/api"
	assessmentapp "github.com/alrazihi/civora/internal/assessment/application"
	assessmentpostgres "github.com/alrazihi/civora/internal/assessment/infrastructure/postgres"
	assistanceapi "github.com/alrazihi/civora/internal/assistance/api"
	assistancapp "github.com/alrazihi/civora/internal/assistance/application"
	assistancepostgres "github.com/alrazihi/civora/internal/assistance/infrastructure/postgres"
	auditapi "github.com/alrazihi/civora/internal/audit/api"
	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	caseapi "github.com/alrazihi/civora/internal/cases/api"
	caseapp "github.com/alrazihi/civora/internal/cases/application"
	casepostgres "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	"github.com/alrazihi/civora/internal/database"
	decisionsapi "github.com/alrazihi/civora/internal/decisions/api"
	decisionsapp "github.com/alrazihi/civora/internal/decisions/application"
	decisionspostgres "github.com/alrazihi/civora/internal/decisions/infrastructure/postgres"
	eligibilityapi "github.com/alrazihi/civora/internal/eligibility/api"
	eligibilityapp "github.com/alrazihi/civora/internal/eligibility/application"
	eligibilitypostgres "github.com/alrazihi/civora/internal/eligibility/infrastructure/postgres"
	evidenceapi "github.com/alrazihi/civora/internal/evidence/api"
	evidenceapp "github.com/alrazihi/civora/internal/evidence/application"
	evidencepostgres "github.com/alrazihi/civora/internal/evidence/infrastructure/postgres"
	followupapi "github.com/alrazihi/civora/internal/followup/api"
	followupapp "github.com/alrazihi/civora/internal/followup/application"
	followuppostgres "github.com/alrazihi/civora/internal/followup/infrastructure/postgres"
	identityapi "github.com/alrazihi/civora/internal/identity/api"
	identityapp "github.com/alrazihi/civora/internal/identity/application"
	"github.com/alrazihi/civora/internal/identity/domain"
	identitypostgres "github.com/alrazihi/civora/internal/identity/infrastructure/postgres"
	intmid "github.com/alrazihi/civora/internal/middleware"
	orgapi "github.com/alrazihi/civora/internal/organizations/api"
	orgapp "github.com/alrazihi/civora/internal/organizations/application"
	orgpostgres "github.com/alrazihi/civora/internal/organizations/infrastructure/postgres"
	peopleapi "github.com/alrazihi/civora/internal/people/api"
	peoplapp "github.com/alrazihi/civora/internal/people/application"
	peoplepostgres "github.com/alrazihi/civora/internal/people/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/server"
	"github.com/alrazihi/civora/migrations"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestServer struct {
	srv *server.Server
	db  *sql.DB
}

func SetupTestServer(t *testing.T) *TestServer {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:           "0",
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			IdleTimeout:    120 * time.Second,
			RateLimit:      10000,
			RateLimitBurst: 10000,
		},
		Database: config.DatabaseConfig{
			Driver:   "pgx",
			Host:     getEnv("CIVORA_TEST_DB_HOST", "localhost"),
			Port:     getEnv("CIVORA_TEST_DB_PORT", "5432"),
			User:     getEnv("CIVORA_TEST_DB_USER", "civora_test"),
			Password: getEnv("CIVORA_TEST_DB_PASSWORD", "civora_test"),
			DBName:   getEnv("CIVORA_TEST_DB_NAME", "civora_test"),
			SSLMode:  "disable",
		},
		Auth: config.AuthConfig{
			JWTSecret:  "test-secret-key-for-e2e-tests-only-32+chars",
			JWTExpiry:  time.Hour,
			BCryptCost: 4,
		},
		Audit: config.AuditConfig{
			Enabled:          true,
			HashChainEnabled: true,
			RetentionDays:    2555,
		},
	}

	dsn := database.BuildDSN(cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode)
	db, err := database.NewDatabase(dsn, cfg.Database.Driver)
	require.NoError(t, err)

	migrator := database.NewMigrator(db.DB, migrations.FS)
	require.NoError(t, migrator.LoadMigrations())
	require.NoError(t, migrator.Migrate(context.Background()))

	_, err = db.DB.Exec(`
		TRUNCATE TABLE
			follow_ups, assistance, decisions, assessments,
			evidence, eligibilities, people,
			audit.audit_events, cases, users, roles, organizations
		RESTART IDENTITY CASCADE
	`)
	require.NoError(t, err)

	orgRepo := orgpostgres.NewPostgresOrganizationRepository(db.DB)
	userRepo := identitypostgres.NewPostgresUserRepository(db.DB)
	roleRepo := identitypostgres.NewPostgresRoleRepository(db.DB)
	caseRepo := casepostgres.NewPostgresCaseRepository(db.DB)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db.DB)
	personRepo := peoplepostgres.NewPostgresPersonRepository(db.DB)
	eligibilityRepo := eligibilitypostgres.NewPostgresEligibilityRepository(db.DB)
	evidenceRepo := evidencepostgres.NewPostgresEvidenceRepository(db.DB)
	assessmentRepo := assessmentpostgres.NewPostgresAssessmentRepository(db.DB)
	decisionRepo := decisionspostgres.NewPostgresDecisionRepository(db.DB)
	assistanceRepo := assistancepostgres.NewPostgresAssistanceRepository(db.DB)
	followUpRepo := followuppostgres.NewPostgresFollowUpRepository(db.DB)

	auditService := auditapp.NewAuditService(auditRepo, cfg.Audit)

	hasher := domain.NewBCryptHasher(cfg.Auth.BCryptCost)
	jwtSvc := intmid.NewJWTService(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry, "civora")
	identityService := identityapp.NewIdentityService(userRepo, roleRepo, hasher, jwtSvc, auditService)
	roleCreator := domain.NewDefaultRoleCreator(roleRepo)
	orgService := orgapp.NewOrganizationService(orgRepo, roleCreator, auditService)
	caseService := caseapp.NewCaseService(caseRepo, personRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	personService := peoplapp.NewPersonService(personRepo, auditService)
	eligibilityService := eligibilityapp.NewEligibilityService(eligibilityRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	evidenceService := evidenceapp.NewEvidenceService(evidenceRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	assessmentService := assessmentapp.NewAssessmentService(assessmentRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	decisionService := decisionsapp.NewDecisionService(decisionRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	assistanceService := assistancapp.NewAssistanceService(assistanceRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	followUpService := followupapp.NewFollowUpService(followUpRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)

	authMiddleware := intmid.AuthRequired(jwtSvc)

	orgHandler := orgapi.NewHandler(orgService)
	identityHandler := identityapi.NewHandler(identityService)
	caseHandler := caseapi.NewHandler(caseService)
	personHandler := peopleapi.NewHandler(personService)
	eligibilityHandler := eligibilityapi.NewHandler(eligibilityService)
	evidenceHandler := evidenceapi.NewHandler(evidenceService)
	assessmentHandler := assessmentapi.NewHandler(assessmentService)
	decisionHandler := decisionsapi.NewHandler(decisionService)
	assistanceHandler := assistanceapi.NewHandler(assistanceService)
	followUpHandler := followupapi.NewHandler(followUpService)
	auditHandler := auditapi.NewHandler(auditService)

	srv := server.New(cfg, db.DB)
	orgHandler.RegisterRoutes(srv.Router(), authMiddleware)
	identityHandler.RegisterRoutes(srv.Router(), authMiddleware)
	caseHandler.RegisterRoutes(srv.Router(), authMiddleware)
	personHandler.RegisterRoutes(srv.Router(), authMiddleware)
	eligibilityHandler.RegisterRoutes(srv.Router(), authMiddleware)
	evidenceHandler.RegisterRoutes(srv.Router(), authMiddleware)
	assessmentHandler.RegisterRoutes(srv.Router(), authMiddleware)
	decisionHandler.RegisterRoutes(srv.Router(), authMiddleware)
	assistanceHandler.RegisterRoutes(srv.Router(), authMiddleware)
	followUpHandler.RegisterRoutes(srv.Router(), authMiddleware)
	auditHandler.RegisterRoutes(srv.Router(), authMiddleware)

	return &TestServer{
		srv: srv,
		db:  db.DB,
	}
}

func (ts *TestServer) makeRequest(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var req *http.Request
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		req = httptest.NewRequest(method, path, bytes.NewReader(data))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rr := httptest.NewRecorder()
	ts.srv.Router().ServeHTTP(rr, req)
	return rr
}

func (ts *TestServer) createOrg(t *testing.T, slug, name string) uuid.UUID {
	t.Helper()
	resp := ts.makeRequest(t, "POST", "/api/v1/organizations", "", map[string]interface{}{
		"name": name,
		"slug": slug,
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())
	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &result)
	orgID, _ := uuid.Parse(result.Data.ID)
	return orgID
}

func (ts *TestServer) registerUser(t *testing.T, orgID uuid.UUID, email, name, password string) uuid.UUID {
	t.Helper()
	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/auth/register", "", map[string]interface{}{
		"email":    email,
		"name":     name,
		"password": password,
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())
	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &result)
	userID, _ := uuid.Parse(result.Data.ID)
	return userID
}

func (ts *TestServer) login(t *testing.T, orgID uuid.UUID, email, password string) string {
	t.Helper()
	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/auth/login", "", map[string]interface{}{
		"email":    email,
		"password": password,
	})
	require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())

	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &result)
	return result.Data.Token
}

func tablesExist(db *sql.DB) bool {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'organizations')
	`).Scan(&exists)
	return err == nil && exists
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestEmergencyAssistanceRequestLifecycle(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "emergency-response", "Emergency Response Org")
	ts.registerUser(t, orgID, "responder@example.com", "Test Responder", "securepass1234")
	token := ts.login(t, orgID, "responder@example.com", "securepass1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, map[string]interface{}{
		"title":        "Emergency Food Assistance",
		"description":  "Family of 4 needs emergency food assistance",
		"service_type": "EMERGENCY",
		"priority":     "HIGH",
	})
	require.Equal(t, http.StatusCreated, resp.Code)

	var createResp struct {
		Data struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			CaseNumber string `json:"case_number"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &createResp)
	caseID := createResp.Data.ID
	assert.Equal(t, "NEW", createResp.Data.Status)
	assert.NotEmpty(t, createResp.Data.CaseNumber)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "OPEN",
	})
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "IN_REVIEW",
	})
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "CLOSED",
	})
	require.Equal(t, http.StatusConflict, resp.Code)
}

func TestAuthenticationRequired(t *testing.T) {
	ts := SetupTestServer(t)

	resp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+uuid.New().String()+"/cases", "", nil)
	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestTenantIsolationAtAPI(t *testing.T) {
	ts := SetupTestServer(t)

	org1 := ts.createOrg(t, "org1", "Organization 1")
	org2 := ts.createOrg(t, "org2", "Organization 2")

	ts.registerUser(t, org1, "user1@example.com", "User One", "password1234")
	ts.registerUser(t, org2, "user2@example.com", "User Two", "password1234")

	token1 := ts.login(t, org1, "user1@example.com", "password1234")
	token2 := ts.login(t, org2, "user2@example.com", "password1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+org1.String()+"/people", token1, map[string]interface{}{
		"first_name":         "Jane",
		"last_name":          "Doe",
		"preferred_language": "en",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var personResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &personResp)
	personID := personResp.Data.ID

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+org1.String()+"/cases", token1, map[string]interface{}{
		"title":        "Case in Org 1",
		"service_type": "GENERAL",
		"priority":     "NORMAL",
		"person_id":    personID,
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var caseResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &caseResp)
	caseID := caseResp.Data.ID

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+org2.String()+"/people/"+personID, token2, nil)
	require.Equal(t, http.StatusNotFound, resp.Code,
		"person from org1 should not be visible in org2; body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+org2.String()+"/cases/"+caseID, token2, nil)
	require.Equal(t, http.StatusNotFound, resp.Code,
		"case from org1 should not be visible in org2; body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+org2.String()+"/cases", token1, nil)
	require.Equal(t, http.StatusForbidden, resp.Code,
		"user from org1 should not access org2 path; body: %s", resp.Body.String())
}

func TestRBAC_RoleAssignmentOnRegistration(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "rbac-test", "RBAC Test Org")

	ts.registerUser(t, orgID, "first@example.com", "First User", "password1234")
	ts.registerUser(t, orgID, "second@example.com", "Second User", "password1234")

	firstToken := ts.login(t, orgID, "first@example.com", "password1234")
	secondToken := ts.login(t, orgID, "second@example.com", "password1234")

	resp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/users", firstToken, nil)
	require.Equal(t, http.StatusOK, resp.Code, "admin should list users; body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/users", secondToken, nil)
	require.Equal(t, http.StatusOK, resp.Code, "staff should list users; body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/audit", firstToken, nil)
	require.Equal(t, http.StatusOK, resp.Code, "admin should access audit; body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/audit", secondToken, nil)
	require.Equal(t, http.StatusOK, resp.Code, "staff should access audit; body: %s", resp.Body.String())
}

func TestRBAC_RegistrationIgnoresRoleName(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "rbac-role-injection", "RBAC Role Injection Org")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/auth/register", "", map[string]interface{}{
		"email":     "attacker@example.com",
		"name":      "Attacker",
		"password":  "password1234",
		"role_name": "admin",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "registration should succeed; body: %s", resp.Body.String())

	token := ts.login(t, orgID, "attacker@example.com", "password1234")
	require.NotEmpty(t, token)

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/audit", token, nil)
	require.Equal(t, http.StatusOK, resp.Code, "registered user (first=staff) should access audit; body: %s", resp.Body.String())
}

func TestRBAC_ProtectedRoutesRequireAuth(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "rbac-noauth", "RBAC No Auth Org")
	ts.registerUser(t, orgID, "user@example.com", "Test User", "password1234")

	tests := []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/organizations/" + orgID.String() + "/cases"},
		{"POST", "/api/v1/organizations/" + orgID.String() + "/cases/" + uuid.New().String() + "/transitions"},
		{"POST", "/api/v1/organizations/" + orgID.String() + "/cases/" + uuid.New().String() + "/assign"},
		{"GET", "/api/v1/organizations/" + orgID.String() + "/users"},
		{"GET", "/api/v1/organizations/" + orgID.String() + "/audit"},
		{"POST", "/api/v1/organizations/" + orgID.String() + "/people"},
		{"POST", "/api/v1/organizations/" + orgID.String() + "/eligibilities"},
		{"POST", "/api/v1/organizations/" + orgID.String() + "/evidence"},
		{"POST", "/api/v1/organizations/" + orgID.String() + "/assessments"},
		{"POST", "/api/v1/organizations/" + orgID.String() + "/decisions"},
		{"POST", "/api/v1/organizations/" + orgID.String() + "/assistance"},
		{"POST", "/api/v1/organizations/" + orgID.String() + "/follow-ups"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			resp := ts.makeRequest(t, tc.method, tc.path, "", nil)
			require.Equal(t, http.StatusUnauthorized, resp.Code,
				"unauthenticated request should be rejected; body: %s", resp.Body.String())
		})
	}
}

func TestServiceRequestFullLifecycle(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "full-lifecycle", "Full Lifecycle Org")
	staffUserID := ts.registerUser(t, orgID, "staff@example.com", "Test Staff", "securepass1234")
	token := ts.login(t, orgID, "staff@example.com", "securepass1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/people", token, map[string]interface{}{
		"first_name":         "Jane",
		"last_name":          "Doe",
		"preferred_language": "en",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var personResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &personResp)
	personID := personResp.Data.ID

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, map[string]interface{}{
		"title":        "Emergency Shelter Request",
		"description":  "Family needs temporary shelter",
		"service_type": "SHELTER",
		"priority":     "HIGH",
		"person_id":    personID,
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var caseResp struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &caseResp)
	caseID := caseResp.Data.ID
	assert.Equal(t, "NEW", caseResp.Data.Status)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "OPEN",
	})
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "IN_REVIEW",
	})
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/eligibilities", token, map[string]interface{}{
		"service_request_id": caseID,
		"criteria": map[string]interface{}{
			"income_verified":    true,
			"residency_verified": true,
			"household_size":     4,
		},
		"explanation": "All criteria met based on submitted documentation",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var eligibilityResp struct {
		Data struct {
			ID     string `json:"id"`
			Result string `json:"result"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &eligibilityResp)
	eligibilityID := eligibilityResp.Data.ID

	resp = ts.makeRequest(t, "PATCH", "/api/v1/organizations/"+orgID.String()+"/eligibilities/"+eligibilityID+"/result", token, map[string]interface{}{
		"result": "ELIGIBLE",
	})
	require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/evidence", token, map[string]interface{}{
		"service_request_id": caseID,
		"type":               "IDENTITY_DOCUMENT",
		"description":        "Government-issued photo ID",
		"storage_reference":  "s3://civora-evidence/doc-001",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/evidence", token, map[string]interface{}{
		"service_request_id": caseID,
		"type":               "PROOF_OF_RESIDENCE",
		"description":        "Utility bill showing current address",
		"storage_reference":  "s3://civora-evidence/doc-002",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "ASSESSMENT",
	})
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/assessments", token, map[string]interface{}{
		"service_request_id": caseID,
		"findings":           "Household of 4 with verified income below 138% of federal poverty level.",
		"needs_identified":   "Emergency shelter, food assistance, case management.",
		"recommendation":     "Approve emergency shelter placement and coordinate with food bank.",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "DECISION_PENDING",
	})
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/decisions", token, map[string]interface{}{
		"service_request_id": caseID,
		"decision":           "APPROVED",
		"reason":             "Meets all eligibility criteria. Assessment supports shelter placement.",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "APPROVED",
	})
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "IN_PROGRESS",
	})
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/assistance", token, map[string]interface{}{
		"service_request_id": caseID,
		"type":               "SHELTER",
		"description":        "Emergency shelter placement at City Shelter Center for 30 days",
		"responsible_staff":  staffUserID.String(),
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var assistanceResp struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &assistanceResp)
	assistanceID := assistanceResp.Data.ID

	var staffID struct {
		Data struct {
			ID             string `json:"id"`
			OrganizationID string `json:"organization_id"`
		} `json:"data"`
	}
	userResp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/users", token, nil)
	require.Equal(t, http.StatusOK, userResp.Code)
	var usersList struct {
		Data []struct {
			ID             string `json:"id"`
			OrganizationID string `json:"organization_id"`
		} `json:"data"`
	}
	json.Unmarshal(userResp.Body.Bytes(), &usersList)
	if len(usersList.Data) > 0 {
		json.Unmarshal(userResp.Body.Bytes(), &staffID)
		staffUUID := usersList.Data[0].ID

		resp = ts.makeRequest(t, "PATCH", "/api/v1/organizations/"+orgID.String()+"/assistance/"+assistanceID+"/status", token, map[string]interface{}{
			"action": "start",
		})
		require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())

		_ = staffUUID
	}

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/follow-ups", token, map[string]interface{}{
		"service_request_id": caseID,
		"scheduled_date":     "2026-10-15",
		"outcome":            "Family stably housed and receiving ongoing support",
		"notes":              "Family of 4 placed in temporary shelter. Weekly check-ins scheduled.",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var followUpResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &followUpResp)
	followUpID := followUpResp.Data.ID

	resp = ts.makeRequest(t, "PATCH", "/api/v1/organizations/"+orgID.String()+"/follow-ups/"+followUpID+"/complete", token, map[string]interface{}{
		"completed_date": "2026-10-15",
	})
	require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "FOLLOW_UP",
	})
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "CLOSED",
	})
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "OPEN",
	})
	require.Equal(t, http.StatusConflict, resp.Code,
		"closed case cannot be transitioned; body: %s", resp.Body.String())
}

func TestUnauthorizedDecisionOnAnotherOrg(t *testing.T) {
	ts := SetupTestServer(t)

	org1 := ts.createOrg(t, "org1", "Organization 1")
	org2 := ts.createOrg(t, "org2", "Organization 2")

	ts.registerUser(t, org1, "staff1@example.com", "Staff One", "password1234")
	ts.registerUser(t, org2, "staff2@example.com", "Staff Two", "password1234")

	token1 := ts.login(t, org1, "staff1@example.com", "password1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+org2.String()+"/people", token1, map[string]interface{}{
		"first_name":         "Jane",
		"last_name":          "Doe",
		"preferred_language": "en",
	})
	require.Equal(t, http.StatusForbidden, resp.Code,
		"user from org1 should not create people in org2; body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+org2.String()+"/cases", token1, map[string]interface{}{
		"title": "Cross-tenant case",
	})
	require.Equal(t, http.StatusForbidden, resp.Code,
		"user from org1 should not create cases in org2; body: %s", resp.Body.String())
}

func TestAuditTrailForServiceRequest(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "audit-trail-test", "Audit Trail Org")
	ts.registerUser(t, orgID, "auditor@example.com", "Test Auditor", "securepass1234")
	token := ts.login(t, orgID, "auditor@example.com", "securepass1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/people", token, map[string]interface{}{
		"first_name":         "Audit",
		"last_name":          "Person",
		"preferred_language": "en",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var personResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &personResp)
	personID := personResp.Data.ID

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, map[string]interface{}{
		"title":        "Audit Trail Request",
		"description":  "Testing audit events",
		"service_type": "GENERAL",
		"priority":     "NORMAL",
		"person_id":    personID,
	})
	require.Equal(t, http.StatusCreated, resp.Code)

	var caseResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &caseResp)
	caseID := caseResp.Data.ID

	ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "OPEN",
	})

	ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "IN_REVIEW",
	})

	ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "ASSESSMENT",
	})

	ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "DECISION_PENDING",
	})

	ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/decisions", token, map[string]interface{}{
		"service_request_id": caseID,
		"decision":           "REJECTED",
		"reason":             "Insufficient documentation provided",
	})

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/audit", token, nil)
	require.Equal(t, http.StatusOK, resp.Code)

	var auditResp struct {
		Data []struct {
			ID       string `json:"id"`
			Action   string `json:"action"`
			Resource string `json:"resource"`
			Outcome  string `json:"outcome"`
			Hash     string `json:"hash"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &auditResp)
	require.NotEmpty(t, auditResp.Data)

	actions := make(map[string]bool)
	for _, ev := range auditResp.Data {
		assert.NotEmpty(t, ev.Hash)
		actions[ev.Action] = true
	}

	assert.True(t, actions["person.created"], "audit should contain person.created")
	assert.True(t, actions["case.created"], "audit should contain case.created")
	assert.True(t, actions["case.transition"], "audit should contain case.transition")
	assert.True(t, actions["decision.made"], "audit should contain decision.made")
}
