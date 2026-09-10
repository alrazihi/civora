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
	workflowapi "github.com/alrazihi/civora/internal/workflow/api"
	"github.com/alrazihi/civora/internal/workflow/application"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/alrazihi/civora/internal/workflow/infrastructure/postgres"
	"github.com/alrazihi/civora/migrations"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestServer struct {
	srv             *server.Server
	db              *sql.DB
	workflowService *application.WorkflowService
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
	t.Cleanup(func() {
		db.DB.Close()
	})

	migrator := database.NewMigrator(db.DB, migrations.FS)
	require.NoError(t, migrator.LoadMigrations())
	require.NoError(t, migrator.Migrate(context.Background()))

	_, err = db.DB.Exec(`
		TRUNCATE TABLE
			follow_ups, assistance, decisions, assessments,
			evidence, eligibilities, people,
			workflow_transition_history,
			workflow_instances,
			workflow_transitions,
			workflow_states,
			workflow_definitions,
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

	workflowDefRepo := postgres.NewPostgresWorkflowDefinitionRepository(db.DB)
	workflowStateRepo := postgres.NewPostgresWorkflowStateRepository(db.DB)
	workflowTransitionRepo := postgres.NewPostgresWorkflowTransitionRepository(db.DB)
	workflowInstanceRepo := postgres.NewPostgresWorkflowInstanceRepository(db.DB)
	workflowHistoryRepo := postgres.NewPostgresWorkflowTransitionHistoryRepository(db.DB)
	workflowService := application.NewWorkflowService(
		workflowDefRepo, workflowStateRepo, workflowTransitionRepo,
		workflowInstanceRepo, workflowHistoryRepo, auditService,
	)

	orgID := helpers.SeedOrg(db.DB)
	seedEmergencyAssistanceWorkflow(t, db.DB, workflowService, orgID)

	caseService := caseapp.NewCaseService(caseRepo, personRepo, domain.NewOrganizationUserChecker(userRepo), auditService, auditRepo, workflowService)
	personService := peoplapp.NewPersonService(personRepo, auditService)
	eligibilityService := eligibilityapp.NewEligibilityService(eligibilityRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	evidenceService := evidenceapp.NewEvidenceService(evidenceRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	assessmentService := assessmentapp.NewAssessmentService(assessmentRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService)
	decisionService := decisionsapp.NewDecisionService(decisionRepo, caseRepo, caseRepo, domain.NewOrganizationUserChecker(userRepo), auditService, workflowService)
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
	workflowHandler := workflowapi.NewHandler(workflowService)

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
	workflowHandler.RegisterRoutes(srv.Router(), authMiddleware)

	return &TestServer{
		srv:             srv,
		db:              db.DB,
		workflowService: workflowService,
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	orgID, _ := uuid.Parse(result.Data.ID)
	seedEmergencyAssistanceWorkflow(t, ts.db, ts.workflowService, orgID)
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	return result.Data.Token
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (ts *TestServer) createPerson(t *testing.T, orgID uuid.UUID, token, firstName, lastName, language string) string {
	t.Helper()
	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/people", token, map[string]interface{}{
		"first_name":         firstName,
		"last_name":          lastName,
		"preferred_language": language,
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())
	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	return result.Data.ID
}

func (ts *TestServer) createCase(t *testing.T, orgID uuid.UUID, token, title, description, serviceType, priority, personID string) string {
	t.Helper()
	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, map[string]interface{}{
		"title":        title,
		"description":  description,
		"service_type": serviceType,
		"priority":     priority,
		"person_id":    personID,
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())
	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	return result.Data.ID
}

func (ts *TestServer) createAssistance(t *testing.T, orgID uuid.UUID, token, caseID, assistanceType, description, staffID string) string {
	t.Helper()
	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/assistance", token, map[string]interface{}{
		"service_request_id": caseID,
		"type":               assistanceType,
		"description":        description,
		"responsible_staff":  staffID,
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())
	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	return result.Data.ID
}

func (ts *TestServer) createFollowUp(t *testing.T, orgID uuid.UUID, token, caseID, scheduledDate, outcome, notes string) string {
	t.Helper()
	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/follow-ups", token, map[string]interface{}{
		"service_request_id": caseID,
		"scheduled_date":     scheduledDate,
		"outcome":            outcome,
		"notes":              notes,
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())
	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
	return result.Data.ID
}

func (ts *TestServer) transitionCase(t *testing.T, orgID uuid.UUID, caseID, token, status string) {
	t.Helper()
	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/transitions", token, map[string]interface{}{
		"status": status,
	})
	require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())
}

func seedEmergencyAssistanceWorkflow(t *testing.T, db *sql.DB, svc *application.WorkflowService, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	states := []workflowdomain.WorkflowState{
		{ID: uuid.New(), TenantID: orgID, Key: "NEW", Name: "New Request", Terminal: false, DisplayOrder: 0, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "OPEN", Name: "Open", Terminal: false, DisplayOrder: 1, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "IN_REVIEW", Name: "In Review", Terminal: false, DisplayOrder: 2, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "ASSESSMENT", Name: "Assessment", Terminal: false, DisplayOrder: 3, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "DECISION_PENDING", Name: "Decision Pending", Terminal: false, DisplayOrder: 4, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "APPROVED", Name: "Approved", Terminal: false, DisplayOrder: 5, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "REJECTED", Name: "Rejected", Terminal: false, DisplayOrder: 6, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "IN_PROGRESS", Name: "In Progress", Terminal: false, DisplayOrder: 7, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "FOLLOW_UP", Name: "Follow-up", Terminal: false, DisplayOrder: 8, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "CLOSED", Name: "Closed", Terminal: true, DisplayOrder: 9, CreatedAt: now},
	}

	transitions := []workflowdomain.WorkflowTransition{
		{ID: uuid.New(), TenantID: orgID, Key: "open", Name: "Open", FromState: "NEW", ToState: "OPEN", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "review", Name: "Review", FromState: "NEW", ToState: "IN_REVIEW", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "reopen", Name: "Reopen", FromState: "IN_REVIEW", ToState: "OPEN", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "assess", Name: "Assess", FromState: "OPEN", ToState: "IN_REVIEW", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "assess2", Name: "Assess", FromState: "IN_REVIEW", ToState: "ASSESSMENT", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "decide", Name: "Decide", FromState: "ASSESSMENT", ToState: "DECISION_PENDING", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "approve", Name: "Approve", FromState: "DECISION_PENDING", ToState: "APPROVED", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "reject", Name: "Reject", FromState: "DECISION_PENDING", ToState: "REJECTED", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "start_assistance", Name: "Start Assistance", FromState: "APPROVED", ToState: "IN_PROGRESS", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "close_rejected", Name: "Close Rejected", FromState: "REJECTED", ToState: "CLOSED", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "follow_up", Name: "Follow Up", FromState: "IN_PROGRESS", ToState: "FOLLOW_UP", Active: true, CreatedAt: now},
		{ID: uuid.New(), TenantID: orgID, Key: "complete", Name: "Complete", FromState: "FOLLOW_UP", ToState: "CLOSED", Active: true, CreatedAt: now},
	}

	def, err := svc.CreateWorkflowDefinition(ctx, application.CreateWorkflowDefinitionParams{
		TenantID:     orgID,
		ActorID:      uuid.Nil,
		Key:          "emergency_assistance",
		Name:         "Emergency Assistance",
		Description:  "Emergency assistance request workflow",
		Version:      1,
		InitialState: "NEW",
		States:       states,
		Transitions:  transitions,
		Metadata:     map[string]interface{}{},
	})
	require.NoError(t, err)

	err = svc.ActivateWorkflowDefinition(ctx, orgID, def.ID, uuid.Nil)
	require.NoError(t, err)

	return def.ID
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &createResp))
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &personResp))
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caseResp))
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &personResp))
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caseResp))
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &eligibilityResp))
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &assistanceResp))
	assistanceID := assistanceResp.Data.ID

	userResp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/users", token, nil)
	require.Equal(t, http.StatusOK, userResp.Code)
	var usersList struct {
		Data []struct {
			ID             string `json:"id"`
			OrganizationID string `json:"organization_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(userResp.Body.Bytes(), &usersList))
	if len(usersList.Data) > 0 {
		resp = ts.makeRequest(t, "PATCH", "/api/v1/organizations/"+orgID.String()+"/assistance/"+assistanceID+"/status", token, map[string]interface{}{
			"action": "start",
		})
		require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &followUpResp))
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &personResp))
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caseResp))
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
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &auditResp))
	require.NotEmpty(t, auditResp.Data)

	actions := make(map[string]bool)
	for _, ev := range auditResp.Data {
		assert.NotEmpty(t, ev.Hash)
		actions[ev.Action] = true
	}

	assert.True(t, actions["person.created"], "audit should contain person.created")
	assert.True(t, actions["case.created"], "audit should contain case.created")
	assert.True(t, actions["workflow.transition"], "audit should contain workflow.transition")
	assert.True(t, actions["decision.made"], "audit should contain decision.made")
}

func TestWorkflowInstanceCreatedWithCase(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "wf-instance-"+uuid.New().String()[:8], "Workflow Instance Org")
	ts.registerUser(t, orgID, "staff@example.com", "Test Staff", "securepass1234")
	token := ts.login(t, orgID, "staff@example.com", "securepass1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, map[string]interface{}{
		"title":        "Workflow Instance Test",
		"service_type": "EMERGENCY",
		"priority":     "HIGH",
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var caseResp struct {
		Data struct {
			ID                 string `json:"id"`
			WorkflowInstanceID string `json:"workflow_instance_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caseResp))
	caseID := caseResp.Data.ID
	require.NotEmpty(t, caseResp.Data.WorkflowInstanceID, "case should have a workflow instance")

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow", token, nil)
	require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())

	var workflowResp struct {
		Data struct {
			Instance struct {
				CurrentState string `json:"current_state"`
			} `json:"instance"`
			Definition struct {
				InitialState string `json:"initial_state"`
			} `json:"definition"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &workflowResp))
	assert.Equal(t, "NEW", workflowResp.Data.Instance.CurrentState)
	assert.Equal(t, "NEW", workflowResp.Data.Definition.InitialState)
}

func TestWorkflowTransitionViaGenericAPI(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "wf-transitions-"+uuid.New().String()[:8], "Workflow Transitions Org")
	ts.registerUser(t, orgID, "staff@example.com", "Test Staff", "securepass1234")
	token := ts.login(t, orgID, "staff@example.com", "securepass1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, map[string]interface{}{
		"title":        "Workflow Transition Test",
		"service_type": "EMERGENCY",
		"priority":     "HIGH",
	})
	require.Equal(t, http.StatusCreated, resp.Code)

	var caseResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caseResp))
	caseID := caseResp.Data.ID

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow/transitions", token, nil)
	require.Equal(t, http.StatusOK, resp.Code)

	var transitionsResp struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &transitionsResp))
	require.NotEmpty(t, transitionsResp.Data)

	transitionKeys := make([]string, 0, len(transitionsResp.Data))
	for _, t := range transitionsResp.Data {
		transitionKeys = append(transitionKeys, t["key"].(string))
	}
	assert.Contains(t, transitionKeys, "open")

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow/transitions/open", token, map[string]interface{}{
		"reason": "Opening case for processing",
	})
	require.Equal(t, http.StatusOK, resp.Code, "response body: %s", resp.Body.String())

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow", token, nil)
	require.Equal(t, http.StatusOK, resp.Code)

	var workflowResp struct {
		Data struct {
			Instance struct {
				CurrentState string `json:"current_state"`
			} `json:"instance"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &workflowResp))
	assert.Equal(t, "OPEN", workflowResp.Data.Instance.CurrentState)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow/transitions/invalid_transition", token, nil)
	require.Equal(t, http.StatusConflict, resp.Code, "invalid transition should be rejected")
}

func TestWorkflowHistoryRecorded(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "wf-history-"+uuid.New().String()[:8], "Workflow History Org")
	ts.registerUser(t, orgID, "staff@example.com", "Test Staff", "securepass1234")
	token := ts.login(t, orgID, "staff@example.com", "securepass1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, map[string]interface{}{
		"title":        "Workflow History Test",
		"service_type": "EMERGENCY",
		"priority":     "HIGH",
	})
	require.Equal(t, http.StatusCreated, resp.Code)

	var caseResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caseResp))
	caseID := caseResp.Data.ID

	ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow/transitions/open", token, nil)
	ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow/transitions/assess", token, nil)

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow/history", token, nil)
	require.Equal(t, http.StatusOK, resp.Code)

	var historyResp struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &historyResp))
	history := historyResp.Data
	assert.Len(t, history, 2, "should have 2 transition history entries")
	assert.Equal(t, "NEW", history[0]["from_state"])
	assert.Equal(t, "OPEN", history[0]["to_state"])
	assert.Equal(t, "open", history[0]["transition_key"])
	assert.Equal(t, "OPEN", history[1]["from_state"])
	assert.Equal(t, "IN_REVIEW", history[1]["to_state"])
	assert.Equal(t, "assess", history[1]["transition_key"])
}

func TestWorkflowTerminalStateBlocksTransitions(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "wf-terminal-"+uuid.New().String()[:8], "Workflow Terminal Org")
	ts.registerUser(t, orgID, "staff@example.com", "Test Staff", "securepass1234")
	token := ts.login(t, orgID, "staff@example.com", "securepass1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases", token, map[string]interface{}{
		"title":        "Terminal State Test",
		"service_type": "EMERGENCY",
		"priority":     "HIGH",
	})
	require.Equal(t, http.StatusCreated, resp.Code)

	var caseResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caseResp))
	caseID := caseResp.Data.ID

	transitions := []string{"open", "assess", "decide", "approve", "start_assistance", "follow_up", "complete"}
	for _, tr := range transitions {
		ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow/transitions/"+tr, token, nil)
	}

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/workflow/transitions/open", token, nil)
	require.Equal(t, http.StatusConflict, resp.Code, "terminal state should block further transitions")
}

func TestWorkflowDefinitionActivation(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "wf-def-"+uuid.New().String()[:8], "Workflow Definition Org")
	ts.registerUser(t, orgID, "admin@example.com", "Test Admin", "securepass1234")
	token := ts.login(t, orgID, "admin@example.com", "securepass1234")

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/workflows", token, map[string]interface{}{
		"key":           "custom_workflow",
		"name":          "Custom Workflow",
		"version":       1,
		"initial_state": "START",
		"states": []map[string]interface{}{
			{"key": "START", "name": "Start", "terminal": false, "display_order": 0},
			{"key": "END", "name": "End", "terminal": true, "display_order": 1},
		},
		"transitions": []map[string]interface{}{
			{"key": "finish", "name": "Finish", "from_state": "START", "to_state": "END", "active": true},
		},
	})
	require.Equal(t, http.StatusCreated, resp.Code, "response body: %s", resp.Body.String())

	var defResp struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &defResp))
	defID := defResp.Data.ID
	assert.Equal(t, "DRAFT", defResp.Data.Status)

	resp = ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/workflows/"+defID+"/activate", token, nil)
	require.Equal(t, http.StatusOK, resp.Code)

	resp = ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/workflows/"+defID, token, nil)
	require.Equal(t, http.StatusOK, resp.Code)

	var activatedResp struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &activatedResp))
	assert.Equal(t, "ACTIVE", activatedResp.Data.Status)
}

func TestEvidenceByServiceRequest(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "evidence-sr-"+uuid.New().String()[:8], "Evidence SR Org")
	ts.registerUser(t, orgID, "staff@example.com", "Test Staff", "securepass1234")
	token := ts.login(t, orgID, "staff@example.com", "securepass1234")

	personID := ts.createPerson(t, orgID, token, "Evidence", "Person", "en")
	caseID := ts.createCase(t, orgID, token, "Evidence SR Case", "Testing evidence by service request", "EMERGENCY", "HIGH", personID)

	ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/evidence", token, map[string]interface{}{
		"service_request_id": caseID,
		"type":               "IDENTITY_DOCUMENT",
		"description":        "Passport scan",
		"storage_reference":  "s3://civora-evidence/passport-001",
	})

	resp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/evidence/by-service-request/"+caseID, token, nil)
	require.Equal(t, http.StatusOK, resp.Code)

	var evidenceResp struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &evidenceResp))
	assert.Len(t, evidenceResp.Data, 1)
	assert.Equal(t, "IDENTITY_DOCUMENT", evidenceResp.Data[0]["type"])
}

func TestEligibilityByServiceRequest(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "eligibility-sr-"+uuid.New().String()[:8], "Eligibility SR Org")
	ts.registerUser(t, orgID, "staff@example.com", "Test Staff", "securepass1234")
	token := ts.login(t, orgID, "staff@example.com", "securepass1234")

	personID := ts.createPerson(t, orgID, token, "Eligibility", "Person", "en")
	caseID := ts.createCase(t, orgID, token, "Eligibility SR Case", "Testing eligibility by service request", "GENERAL", "NORMAL", personID)

	ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/eligibilities", token, map[string]interface{}{
		"service_request_id": caseID,
		"criteria": map[string]interface{}{
			"income_verified":    true,
			"residency_verified": true,
			"household_size":     3,
		},
		"explanation": "All criteria verified",
	})

	resp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/eligibilities/by-service-request/"+caseID, token, nil)
	require.Equal(t, http.StatusOK, resp.Code)

	var eligibilityResp struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &eligibilityResp))
	assert.Equal(t, "REQUIRES_MORE_INFORMATION", eligibilityResp.Data["result"])
}

func TestAssistanceByServiceRequest(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "assistance-sr-"+uuid.New().String()[:8], "Assistance SR Org")
	ts.registerUser(t, orgID, "staff@example.com", "Test Staff", "securepass1234")
	token := ts.login(t, orgID, "staff@example.com", "securepass1234")

	personID := ts.createPerson(t, orgID, token, "Assistance", "Person", "en")
	caseID := ts.createCase(t, orgID, token, "Assistance SR Case", "Testing assistance by service request", "SHELTER", "HIGH", personID)
	staffID := ts.registerUser(t, orgID, "staff2@example.com", "Staff Two", "securepass1234")

	assistanceID := ts.createAssistance(t, orgID, token, caseID, "SHELTER", "Emergency shelter placement", staffID.String())

	resp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/assistance/by-service-request/"+caseID, token, nil)
	require.Equal(t, http.StatusOK, resp.Code)

	var assistanceResp struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &assistanceResp))
	assert.Len(t, assistanceResp.Data, 1)
	assert.Equal(t, assistanceID, assistanceResp.Data[0]["id"])
}

func TestFollowUpByServiceRequest(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "followup-sr-"+uuid.New().String()[:8], "Follow-up SR Org")
	ts.registerUser(t, orgID, "staff@example.com", "Test Staff", "securepass1234")
	token := ts.login(t, orgID, "staff@example.com", "securepass1234")

	personID := ts.createPerson(t, orgID, token, "FollowUp", "Person", "en")
	caseID := ts.createCase(t, orgID, token, "Follow-up SR Case", "Testing follow-up by service request", "GENERAL", "NORMAL", personID)
	ts.transitionCase(t, orgID, caseID, token, "OPEN")
	ts.transitionCase(t, orgID, caseID, token, "IN_REVIEW")
	ts.transitionCase(t, orgID, caseID, token, "ASSESSMENT")
	ts.transitionCase(t, orgID, caseID, token, "DECISION_PENDING")
	ts.transitionCase(t, orgID, caseID, token, "APPROVED")
	ts.transitionCase(t, orgID, caseID, token, "IN_PROGRESS")

	followUpID := ts.createFollowUp(t, orgID, token, caseID, "2026-10-20", "Client doing well", "Weekly check-in completed")

	resp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/follow-ups/by-service-request/"+caseID, token, nil)
	require.Equal(t, http.StatusOK, resp.Code)

	var followUpResp struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &followUpResp))
	assert.Len(t, followUpResp.Data, 1)
	assert.Equal(t, followUpID, followUpResp.Data[0]["id"])
}

func TestDecisionByServiceRequest(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "decision-sr-"+uuid.New().String()[:8], "Decision SR Org")
	ts.registerUser(t, orgID, "staff@example.com", "Test Staff", "securepass1234")
	token := ts.login(t, orgID, "staff@example.com", "securepass1234")

	personID := ts.createPerson(t, orgID, token, "Decision", "Person", "en")
	caseID := ts.createCase(t, orgID, token, "Decision SR Case", "Testing decision by service request", "GENERAL", "NORMAL", personID)
	ts.transitionCase(t, orgID, caseID, token, "OPEN")
	ts.transitionCase(t, orgID, caseID, token, "IN_REVIEW")
	ts.transitionCase(t, orgID, caseID, token, "ASSESSMENT")
	ts.transitionCase(t, orgID, caseID, token, "DECISION_PENDING")

	ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/decisions", token, map[string]interface{}{
		"service_request_id": caseID,
		"decision":           "APPROVED",
		"reason":             "Meets all criteria",
	})

	resp := ts.makeRequest(t, "GET", "/api/v1/organizations/"+orgID.String()+"/decisions/by-service-request/"+caseID, token, nil)
	require.Equal(t, http.StatusOK, resp.Code)

	var decisionResp struct {
		Data map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &decisionResp))
	require.NotNil(t, decisionResp.Data)
	assert.Equal(t, "APPROVED", decisionResp.Data["decision"])
}

func TestCaseAssignmentWorkflow(t *testing.T) {
	ts := SetupTestServer(t)

	orgID := ts.createOrg(t, "case-assign-"+uuid.New().String()[:8], "Case Assign Org")
	staffID := ts.registerUser(t, orgID, "staff@example.com", "Test Staff", "securepass1234")
	_ = ts.registerUser(t, orgID, "admin@example.com", "Test Admin", "securepass1234")
	adminToken := ts.login(t, orgID, "admin@example.com", "securepass1234")

	personID := ts.createPerson(t, orgID, adminToken, "Assign", "Person", "en")
	caseID := ts.createCase(t, orgID, adminToken, "Assignment Case", "Testing case assignment", "GENERAL", "NORMAL", personID)

	resp := ts.makeRequest(t, "POST", "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID+"/assign", adminToken, map[string]interface{}{
		"user_id": staffID.String(),
	})
	require.Equal(t, http.StatusOK, resp.Code)

	var assignResp struct {
		Data struct {
			AssignedTo *string `json:"assigned_to"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &assignResp))
	require.NotNil(t, assignResp.Data.AssignedTo)
	assert.Equal(t, staffID.String(), *assignResp.Data.AssignedTo)
}
