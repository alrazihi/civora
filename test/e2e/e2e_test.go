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

	auditapi "github.com/alrazihi/civora/internal/audit/api"
	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	caseapi "github.com/alrazihi/civora/internal/cases/api"
	caseapp "github.com/alrazihi/civora/internal/cases/application"
	casepostgres "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	"github.com/alrazihi/civora/internal/database"
	identityapi "github.com/alrazihi/civora/internal/identity/api"
	identityapp "github.com/alrazihi/civora/internal/identity/application"
	"github.com/alrazihi/civora/internal/identity/domain"
	identitypostgres "github.com/alrazihi/civora/internal/identity/infrastructure/postgres"
	intmid "github.com/alrazihi/civora/internal/middleware"
	orgapi "github.com/alrazihi/civora/internal/organizations/api"
	orgapp "github.com/alrazihi/civora/internal/organizations/application"
	orgpostgres "github.com/alrazihi/civora/internal/organizations/infrastructure/postgres"
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
			Port:         "0",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
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
	}

	dsn := database.BuildDSN(cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode)
	db, err := database.NewDatabase(dsn, cfg.Database.Driver)
	require.NoError(t, err)

	if !tablesExist(db.DB) {
		migrator := database.NewMigrator(db.DB, migrations.FS)
		require.NoError(t, migrator.LoadMigrations())
		require.NoError(t, migrator.Migrate(context.Background()))
	}

	_, err = db.DB.Exec("TRUNCATE TABLE audit_events, cases, users, roles, organizations RESTART IDENTITY CASCADE")
	require.NoError(t, err)

	orgRepo := orgpostgres.NewPostgresOrganizationRepository(db.DB)
	userRepo := identitypostgres.NewPostgresUserRepository(db.DB)
	roleRepo := identitypostgres.NewPostgresRoleRepository(db.DB)
	caseRepo := casepostgres.NewPostgresCaseRepository(db.DB)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db.DB)

	auditService := auditapp.NewAuditService(auditRepo)

	hasher := domain.NewBCryptHasher(cfg.Auth.BCryptCost)
	jwtSvc := intmid.NewJWTService(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry, "civora")
	identityService := identityapp.NewIdentityService(userRepo, roleRepo, hasher, jwtSvc, auditService)
	roleCreator := domain.NewDefaultRoleCreator(roleRepo)
	orgService := orgapp.NewOrganizationService(orgRepo, roleCreator, auditService)
	caseService := caseapp.NewCaseService(caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)

	authMiddleware := intmid.AuthRequired(jwtSvc)

	orgHandler := orgapi.NewHandler(orgService)
	identityHandler := identityapi.NewHandler(identityService)
	caseHandler := caseapi.NewHandler(caseService)
	auditHandler := auditapi.NewHandler(auditService)

	srv := server.New(cfg, db.DB)
	orgHandler.RegisterRoutes(srv.Router(), authMiddleware)
	identityHandler.RegisterRoutes(srv.Router(), authMiddleware)
	caseHandler.RegisterRoutes(srv.Router(), authMiddleware)
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

func (ts *TestServer) registerUser(t *testing.T, orgID uuid.UUID, email, name, password string) {
	t.Helper()
	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/auth/register", "", map[string]interface{}{
		"email":    email,
		"name":     name,
		"password": password,
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())
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
	ts.registerUser(t, orgID, "responder@example.com", "Test Responder", "securepass123")
	token := ts.login(t, orgID, "responder@example.com", "securepass123")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, map[string]interface{}{
		"title":       "Emergency Food Assistance",
		"description": "Family of 4 needs emergency food assistance",
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
	assert.Equal(t, "CREATED", createResp.Data.Status)
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
		"status": "RESOLVED",
	})
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "CLOSED",
	})
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": "OPEN",
	})
	require.Equal(t, http.StatusConflict, resp.Code)

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID, token, nil)
	require.Equal(t, http.StatusOK, resp.Code)
	var getResp struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &getResp)
	assert.Equal(t, "CLOSED", getResp.Data.Status)

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/audit", token, nil)
	require.Equal(t, http.StatusOK, resp.Code)
	var auditResp struct {
		Data []struct {
			Action   string `json:"action"`
			Resource string `json:"resource"`
			Outcome  string `json:"outcome"`
			Hash     string `json:"hash"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &auditResp)
	require.NotEmpty(t, auditResp.Data)

	for _, ev := range auditResp.Data {
		assert.NotEmpty(t, ev.Hash)
	}
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

	ts.registerUser(t, org1, "user1@example.com", "User One", "password123")
	ts.registerUser(t, org2, "user2@example.com", "User Two", "password123")

	token1 := ts.login(t, org1, "user1@example.com", "password123")
	token2 := ts.login(t, org2, "user2@example.com", "password123")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+org1.String()+"/cases", token1, map[string]interface{}{
		"title": "Case in Org 1",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var createResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &createResp)
	caseID := createResp.Data.ID

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+org2.String()+"/cases/"+caseID, token2, nil)
	require.Equal(t, http.StatusNotFound, resp.Code,
		"user from org2 should not see case from org1; body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+org2.String()+"/cases", token1, nil)
	require.Equal(t, http.StatusForbidden, resp.Code,
		"user from org1 should not access org2 path; body: %s", resp.Body.String())
}

func TestRBAC_RoleAssignmentOnRegistration(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "rbac-test", "RBAC Test Org")

	ts.registerUser(t, orgID, "first@example.com", "First User", "password123")
	ts.registerUser(t, orgID, "second@example.com", "Second User", "password123")

	firstToken := ts.login(t, orgID, "first@example.com", "password123")
	secondToken := ts.login(t, orgID, "second@example.com", "password123")

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
		"password":  "password123",
		"role_name": "admin",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "registration should succeed; body: %s", resp.Body.String())

	token := ts.login(t, orgID, "attacker@example.com", "password123")
	require.NotEmpty(t, token)

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/audit", token, nil)
	require.Equal(t, http.StatusOK, resp.Code, "registered user (first=staff) should access audit; body: %s", resp.Body.String())
}

func TestRBAC_ProtectedRoutesRequireAuth(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "rbac-noauth", "RBAC No Auth Org")
	ts.registerUser(t, orgID, "user@example.com", "Test User", "password123")

	tests := []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/organizations/" + orgID.String() + "/cases"},
		{"POST", "/api/v1/organizations/" + orgID.String() + "/cases/" + uuid.New().String() + "/transitions"},
		{"POST", "/api/v1/organizations/" + orgID.String() + "/cases/" + uuid.New().String() + "/assign"},
		{"GET", "/api/v1/organizations/" + orgID.String() + "/users"},
		{"GET", "/api/v1/organizations/" + orgID.String() + "/audit"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			resp := ts.makeRequest(t, tc.method, tc.path, "", nil)
			require.Equal(t, http.StatusUnauthorized, resp.Code,
				"unauthenticated request should be rejected; body: %s", resp.Body.String())
		})
	}
}

func TestRBAC_CaseTransitionsAllowAdminAndStaff(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "rbac-transitions", "RBAC Transitions Org")
	ts.registerUser(t, orgID, "first@example.com", "First User", "password123")
	ts.registerUser(t, orgID, "second@example.com", "Second User", "password123")

	adminToken := ts.login(t, orgID, "first@example.com", "password123")
	staffToken := ts.login(t, orgID, "second@example.com", "password123")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", adminToken, map[string]interface{}{
		"title": "RBAC Case",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "admin should create case; body: %s", resp.Body.String())

	var createResp struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	json.Unmarshal(resp.Body.Bytes(), &createResp)
	caseID := createResp.Data.ID

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", staffToken, map[string]interface{}{
		"status": "OPEN",
	})
	require.Equal(t, http.StatusOK, resp.Code, "staff should be able to transition; body: %s", resp.Body.String())
}
