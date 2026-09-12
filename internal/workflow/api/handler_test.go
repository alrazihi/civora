package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	workflowapp "github.com/alrazihi/civora/internal/workflow/application"
	workflowdomain "github.com/alrazihi/civora/internal/workflow/domain"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockWorkflowService struct {
	defs                      []*workflowdomain.WorkflowDefinition
	instances                 []*workflowdomain.WorkflowInstance
	histories                 []workflowdomain.WorkflowTransitionHistory
	mockCreateDef             func(ctx context.Context, params workflowapp.CreateWorkflowDefinitionParams) (*workflowdomain.WorkflowDefinition, error)
	mockUpdateDef             func(ctx context.Context, params workflowapp.UpdateWorkflowDefinitionParams) (*workflowdomain.WorkflowDefinition, error)
	mockDeleteDef             func(ctx context.Context, tenantID, id uuid.UUID, actorID uuid.UUID) error
	mockActivateDef           func(ctx context.Context, tenantID, id, actorID uuid.UUID) error
	mockArchiveDef            func(ctx context.Context, tenantID, id, actorID uuid.UUID) error
	mockGetDef                func(ctx context.Context, tenantID, id uuid.UUID) (*workflowdomain.WorkflowDefinition, error)
	mockListDefs              func(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*workflowdomain.WorkflowDefinition, int, error)
	mockCreateInstance        func(ctx context.Context, tenantID, caseID uuid.UUID, workflowDefKey string, actorID uuid.UUID) (*workflowdomain.WorkflowInstance, error)
	mockCreateInstanceByDefID func(ctx context.Context, tenantID, caseID uuid.UUID, workflowDefID uuid.UUID, actorID uuid.UUID) (*workflowdomain.WorkflowInstance, error)
	mockExecTransition        func(ctx context.Context, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error)
	mockExecTransitionTx      func(ctx context.Context, tx *sql.Tx, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error)
	mockGetByCaseID           func(ctx context.Context, tenantID, caseID uuid.UUID) (*workflowdomain.WorkflowInstance, error)
	mockGetValidTrans         func(ctx context.Context, tenantID, instanceID uuid.UUID) ([]workflowdomain.WorkflowTransition, error)
	mockGetHistory            func(ctx context.Context, tenantID, caseID uuid.UUID) ([]workflowdomain.WorkflowTransitionHistory, error)
	mockFindLatestActive      func(ctx context.Context, tenantID uuid.UUID, key string) (*workflowdomain.WorkflowDefinition, error)
}

func (m *mockWorkflowService) CreateWorkflowDefinition(ctx context.Context, params workflowapp.CreateWorkflowDefinitionParams) (*workflowdomain.WorkflowDefinition, error) {
	if m.mockCreateDef != nil {
		return m.mockCreateDef(ctx, params)
	}
	return &workflowdomain.WorkflowDefinition{ID: uuid.New(), Key: params.Key, Name: params.Name}, nil
}
func (m *mockWorkflowService) ActivateWorkflowDefinition(ctx context.Context, tenantID, id, actorID uuid.UUID) error {
	if m.mockActivateDef != nil {
		return m.mockActivateDef(ctx, tenantID, id, actorID)
	}
	return nil
}
func (m *mockWorkflowService) ArchiveWorkflowDefinition(ctx context.Context, tenantID, id, actorID uuid.UUID) error {
	if m.mockArchiveDef != nil {
		return m.mockArchiveDef(ctx, tenantID, id, actorID)
	}
	return nil
}
func (m *mockWorkflowService) GetWorkflowDefinition(ctx context.Context, tenantID, id uuid.UUID) (*workflowdomain.WorkflowDefinition, error) {
	if m.mockGetDef != nil {
		return m.mockGetDef(ctx, tenantID, id)
	}
	for _, def := range m.defs {
		if def.ID == id && def.TenantID == tenantID {
			return def, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *mockWorkflowService) ListWorkflowDefinitions(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*workflowdomain.WorkflowDefinition, int, error) {
	if m.mockListDefs != nil {
		return m.mockListDefs(ctx, tenantID, limit, offset)
	}
	return m.defs, len(m.defs), nil
}
func (m *mockWorkflowService) FindLatestActiveByKey(ctx context.Context, tenantID uuid.UUID, key string) (*workflowdomain.WorkflowDefinition, error) {
	if m.mockFindLatestActive != nil {
		return m.mockFindLatestActive(ctx, tenantID, key)
	}
	for _, def := range m.defs {
		if def.Key == key && def.TenantID == tenantID && def.Status == workflowdomain.WorkflowStatusActive {
			return def, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *mockWorkflowService) CreateInstanceForCase(ctx context.Context, tenantID, caseID uuid.UUID, workflowDefKey string, actorID uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
	if m.mockCreateInstance != nil {
		return m.mockCreateInstance(ctx, tenantID, caseID, workflowDefKey, actorID)
	}
	return &workflowdomain.WorkflowInstance{ID: uuid.New(), TenantID: tenantID, CaseID: caseID}, nil
}
func (m *mockWorkflowService) CreateInstanceForCaseTx(ctx context.Context, tx *sql.Tx, tenantID, caseID uuid.UUID, workflowDefKey string, actorID uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
	return m.CreateInstanceForCase(ctx, tenantID, caseID, workflowDefKey, actorID)
}
func (m *mockWorkflowService) CreateInstanceForCaseByDefID(ctx context.Context, tenantID, caseID uuid.UUID, workflowDefID uuid.UUID, actorID uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
	if m.mockCreateInstanceByDefID != nil {
		return m.mockCreateInstanceByDefID(ctx, tenantID, caseID, workflowDefID, actorID)
	}
	return &workflowdomain.WorkflowInstance{ID: uuid.New(), TenantID: tenantID, CaseID: caseID}, nil
}
func (m *mockWorkflowService) CreateInstanceForCaseByDefIDTx(ctx context.Context, tx *sql.Tx, tenantID, caseID uuid.UUID, workflowDefID uuid.UUID, actorID uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
	return m.CreateInstanceForCaseByDefID(ctx, tenantID, caseID, workflowDefID, actorID)
}
func (m *mockWorkflowService) ListActiveForSelection(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*workflowdomain.WorkflowDefinition, int, error) {
	var active []*workflowdomain.WorkflowDefinition
	for _, def := range m.defs {
		if def.Status == workflowdomain.WorkflowStatusActive {
			active = append(active, def)
		}
	}
	if offset >= len(active) {
		return nil, len(active), nil
	}
	end := offset + limit
	if end > len(active) {
		end = len(active)
	}
	return active[offset:end], len(active), nil
}
func (m *mockWorkflowService) ExecuteTransition(ctx context.Context, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error) {
	if m.mockExecTransition != nil {
		return m.mockExecTransition(ctx, params)
	}
	return &workflowdomain.WorkflowInstance{}, nil
}
func (m *mockWorkflowService) ExecuteTransitionInTx(ctx context.Context, tx *sql.Tx, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error) {
	if m.mockExecTransitionTx != nil {
		return m.mockExecTransitionTx(ctx, tx, params)
	}
	return &workflowdomain.WorkflowInstance{}, nil
}
func (m *mockWorkflowService) GetInstanceByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
	if m.mockGetByCaseID != nil {
		return m.mockGetByCaseID(ctx, tenantID, caseID)
	}
	for _, inst := range m.instances {
		if inst.CaseID == caseID && inst.TenantID == tenantID {
			return inst, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *mockWorkflowService) GetValidTransitions(ctx context.Context, tenantID, instanceID uuid.UUID) ([]workflowdomain.WorkflowTransition, error) {
	if m.mockGetValidTrans != nil {
		return m.mockGetValidTrans(ctx, tenantID, instanceID)
	}
	return nil, nil
}
func (m *mockWorkflowService) GetWorkflowHistoryByCaseID(ctx context.Context, tenantID, caseID uuid.UUID) ([]workflowdomain.WorkflowTransitionHistory, error) {
	if m.mockGetHistory != nil {
		return m.mockGetHistory(ctx, tenantID, caseID)
	}
	return m.histories, nil
}

func (m *mockWorkflowService) UpdateWorkflowDefinition(ctx context.Context, params workflowapp.UpdateWorkflowDefinitionParams) (*workflowdomain.WorkflowDefinition, error) {
	if m.mockUpdateDef != nil {
		return m.mockUpdateDef(ctx, params)
	}
	// Simple mock implementation - find the definition and update it
	for _, def := range m.defs {
		if def.ID == params.ID && def.TenantID == params.TenantID {
			// Update the definition with the new values
			def.Key = params.Key
			def.Name = params.Name
			def.Description = params.Description
			def.Version = params.Version
			def.InitialState = params.InitialState
			def.Metadata = params.Metadata
			// Note: In a real implementation, we would also update states and transitions
			return def, nil
		}
	}
	return nil, workflowdomain.ErrWorkflowDefinitionNotFound{DefID: params.ID}
}

func (m *mockWorkflowService) DeleteWorkflowDefinition(ctx context.Context, tenantID, id uuid.UUID, actorID uuid.UUID) error {
	if m.mockDeleteDef != nil {
		return m.mockDeleteDef(ctx, tenantID, id, actorID)
	}
	// Simple mock implementation - remove the definition
	for i, def := range m.defs {
		if def.ID == id && def.TenantID == tenantID {
			// Remove the definition from the slice
			m.defs = append(m.defs[:i], m.defs[i+1:]...)
			return nil
		}
	}
	return workflowdomain.ErrWorkflowDefinitionNotFound{DefID: id}
}

func generateTestJWT(secret, userID, orgID, role string) string {
	claims := jwt.MapClaims{
		"sub":             userID,
		"organization_id": orgID,
		"role":            role,
		"iss":             "test-issuer",
		"exp":             jwt.NewNumericDate(time.Now().Add(time.Hour)).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}

func setupWorkflowRouter(svc WorkflowService) http.Handler {
	jwtSvc := middleware.NewJWTService("test-secret", time.Hour, "test-issuer")
	h := NewHandler(svc)
	r := chi.NewRouter()
	r.Route("/api/v1/organizations/{orgId}/workflows", func(r chi.Router) {
		r.Use(middleware.AuthRequired(jwtSvc))
		r.Use(middleware.RequireSameTenant)
		r.Get("/", h.ListWorkflowDefinitions)
		r.Get("/{workflowId}", h.GetWorkflowDefinition)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin"))
			r.Post("/", h.CreateWorkflowDefinition)
			r.Put("/{workflowId}", h.UpdateWorkflowDefinition)
			r.Post("/{workflowId}/activate", h.ActivateWorkflowDefinition)
			r.Post("/{workflowId}/archive", h.ArchiveWorkflowDefinition)
			r.Delete("/{workflowId}", h.DeleteWorkflowDefinition)
		})
	})
	r.Route("/api/v1/organizations/{orgId}/cases/{caseId}/workflow", func(r chi.Router) {
		r.Use(middleware.AuthRequired(jwtSvc))
		r.Use(middleware.RequireSameTenant)
		r.Get("/", h.GetCaseWorkflow)
		r.Get("/transitions", h.GetValidTransitions)
		r.Get("/history", h.GetWorkflowHistory)
		r.Post("/transitions/{transitionKey}", h.ExecuteTransition)
	})
	return r
}

func TestCreateWorkflowDefinition_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	mockSvc := &mockWorkflowService{}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	body := `{"key":"test_wf","name":"Test Workflow","version":1,"initial_state":"NEW","states":[{"key":"NEW","name":"New","display_order":0}],"transitions":[{"key":"open","from_state":"NEW","to_state":"OPEN"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/workflows", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
}

func TestCreateWorkflowDefinition_ValidationError(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	mockSvc := &mockWorkflowService{
		mockCreateDef: func(ctx context.Context, params workflowapp.CreateWorkflowDefinitionParams) (*workflowdomain.WorkflowDefinition, error) {
			return nil, errors.New("invalid workflow definition: key is required")
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	body := `{"key":"","name":"","version":0,"initial_state":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/workflows", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestCreateWorkflowDefinition_NoAuth(t *testing.T) {
	orgID := uuid.New()
	mockSvc := &mockWorkflowService{}
	r := setupWorkflowRouter(mockSvc)

	body := `{"key":"test_wf","name":"Test Workflow"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/workflows", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestActivateWorkflowDefinition_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	mockSvc := &mockWorkflowService{}
	r := setupWorkflowRouter(mockSvc)

	workflowID := uuid.New()
	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/workflows/"+workflowID.String()+"/activate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestActivateWorkflowDefinition_NotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	mockSvc := &mockWorkflowService{
		mockActivateDef: func(ctx context.Context, tenantID, id, actorID uuid.UUID) error {
			return workflowdomain.ErrWorkflowDefinitionNotFound{DefID: id}
		},
	}
	r := setupWorkflowRouter(mockSvc)

	workflowID := uuid.New()
	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/workflows/"+workflowID.String()+"/activate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestListWorkflowDefinitions_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	mockSvc := &mockWorkflowService{
		defs: []*workflowdomain.WorkflowDefinition{
			{ID: uuid.New(), Key: "test_wf", Name: "Test", Status: workflowdomain.WorkflowStatusActive},
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/workflows", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp shared.APIResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	data := resp.Data.([]interface{})
	assert.Equal(t, 1, len(data))
}

func TestGetCaseWorkflow_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	caseID := uuid.New()
	instanceID := uuid.New()

	mockSvc := &mockWorkflowService{
		instances: []*workflowdomain.WorkflowInstance{
			{ID: instanceID, CaseID: caseID, CurrentState: "NEW"},
		},
		mockGetByCaseID: func(ctx context.Context, tenantID, caseIDParam uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
			return &workflowdomain.WorkflowInstance{ID: instanceID, CaseID: caseID, CurrentState: "NEW"}, nil
		},
		mockGetDef: func(ctx context.Context, tenantID, id uuid.UUID) (*workflowdomain.WorkflowDefinition, error) {
			return &workflowdomain.WorkflowDefinition{ID: id, Key: "test_wf", Name: "Test"}, nil
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID.String()+"/workflow", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetCaseWorkflow_NotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	caseID := uuid.New()

	mockSvc := &mockWorkflowService{
		mockGetByCaseID: func(ctx context.Context, tenantID, caseIDParam uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
			return nil, workflowdomain.ErrWorkflowInstanceNotFound{InstanceID: caseIDParam}
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID.String()+"/workflow", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestExecuteTransition_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	caseID := uuid.New()
	instanceID := uuid.New()

	mockSvc := &mockWorkflowService{
		mockGetByCaseID: func(ctx context.Context, tenantID, caseIDParam uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
			return &workflowdomain.WorkflowInstance{ID: instanceID, CaseID: caseID, CurrentState: "NEW"}, nil
		},
		mockGetValidTrans: func(ctx context.Context, tenantID, instanceIDParam uuid.UUID) ([]workflowdomain.WorkflowTransition, error) {
			return []workflowdomain.WorkflowTransition{{Key: "open", FromState: "NEW", ToState: "OPEN"}}, nil
		},
		mockExecTransition: func(ctx context.Context, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error) {
			return &workflowdomain.WorkflowInstance{ID: instanceID, CaseID: caseID, CurrentState: "OPEN"}, nil
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID.String()+"/workflow/transitions/open", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestExecuteTransition_UnauthorizedTransition(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	caseID := uuid.New()

	mockSvc := &mockWorkflowService{
		mockGetByCaseID: func(ctx context.Context, tenantID, caseIDParam uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
			return &workflowdomain.WorkflowInstance{ID: uuid.New(), CaseID: caseID, CurrentState: "NEW"}, nil
		},
		mockGetValidTrans: func(ctx context.Context, tenantID, instanceID uuid.UUID) ([]workflowdomain.WorkflowTransition, error) {
			return []workflowdomain.WorkflowTransition{{Key: "open", FromState: "NEW", ToState: "OPEN"}}, nil
		},
		mockExecTransition: func(ctx context.Context, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error) {
			return nil, workflowdomain.ErrUnauthorizedTransition{
				TransitionKey: params.TransitionKey,
				AllowedRoles:  []string{"admin"},
			}
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "staff")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID.String()+"/workflow/transitions/open", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestExecuteTransition_InvalidTransition(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	caseID := uuid.New()

	mockSvc := &mockWorkflowService{
		mockGetByCaseID: func(ctx context.Context, tenantID, caseIDParam uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
			return &workflowdomain.WorkflowInstance{ID: uuid.New(), CaseID: caseID, CurrentState: "NEW"}, nil
		},
		mockGetValidTrans: func(ctx context.Context, tenantID, instanceID uuid.UUID) ([]workflowdomain.WorkflowTransition, error) {
			return []workflowdomain.WorkflowTransition{{Key: "open", FromState: "NEW", ToState: "OPEN"}}, nil
		},
		mockExecTransition: func(ctx context.Context, params workflowapp.ExecuteTransitionParams) (*workflowdomain.WorkflowInstance, error) {
			return nil, workflowdomain.ErrTransitionNotFound{FromState: "NEW", ToState: "CLOSED"}
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID.String()+"/workflow/transitions/close", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestGetValidTransitions_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	caseID := uuid.New()

	mockSvc := &mockWorkflowService{
		mockGetByCaseID: func(ctx context.Context, tenantID, caseIDParam uuid.UUID) (*workflowdomain.WorkflowInstance, error) {
			return &workflowdomain.WorkflowInstance{ID: uuid.New(), CaseID: caseID, CurrentState: "NEW"}, nil
		},
		mockGetValidTrans: func(ctx context.Context, tenantID, instanceID uuid.UUID) ([]workflowdomain.WorkflowTransition, error) {
			return []workflowdomain.WorkflowTransition{{Key: "open", FromState: "NEW", ToState: "OPEN"}}, nil
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID.String()+"/workflow/transitions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetWorkflowHistory_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	caseID := uuid.New()

	histories := []workflowdomain.WorkflowTransitionHistory{
		{FromState: "NEW", ToState: "OPEN", TransitionKey: "open"},
	}
	mockSvc := &mockWorkflowService{
		mockGetHistory: func(ctx context.Context, tenantID, caseIDParam uuid.UUID) ([]workflowdomain.WorkflowTransitionHistory, error) {
			return histories, nil
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/cases/"+caseID.String()+"/workflow/history", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp shared.APIResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	data := resp.Data.([]interface{})
	assert.Equal(t, 1, len(data))
}

func TestWorkflowDefinition_TenantIsolation(t *testing.T) {
	ownerOrgID := uuid.New()
	otherOrgID := uuid.New()
	userID := uuid.New()
	defID := uuid.New()

	defs := []*workflowdomain.WorkflowDefinition{
		{ID: defID, Key: "test_wf", Name: "Test", TenantID: ownerOrgID, Status: workflowdomain.WorkflowStatusActive},
	}
	mockSvc := &mockWorkflowService{
		mockGetDef: func(ctx context.Context, tenantID, id uuid.UUID) (*workflowdomain.WorkflowDefinition, error) {
			for _, def := range defs {
				if def.ID == id {
					if def.TenantID != tenantID {
						return nil, workflowdomain.ErrTenantViolation{}
					}
					return def, nil
				}
			}
			return nil, workflowdomain.ErrWorkflowDefinitionNotFound{DefID: id}
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), ownerOrgID.String(), "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+otherOrgID.String()+"/workflows/"+defID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestCreateWorkflowDefinition_EmptyKeyRejected(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	mockSvc := &mockWorkflowService{
		mockCreateDef: func(ctx context.Context, params workflowapp.CreateWorkflowDefinitionParams) (*workflowdomain.WorkflowDefinition, error) {
			if params.Key == "" {
				return nil, errors.New("invalid workflow definition: key is required")
			}
			return &workflowdomain.WorkflowDefinition{ID: uuid.New(), Key: params.Key, Name: params.Name}, nil
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	body := `{"key":"","name":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/workflows", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestUpdateWorkflowDefinition_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	workflowID := uuid.New()
	mockSvc := &mockWorkflowService{
		defs: []*workflowdomain.WorkflowDefinition{
			{ID: workflowID, TenantID: orgID, Key: "test_wf", Name: "Test", Status: workflowdomain.WorkflowStatusDraft},
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	body := `{"key":"test_wf","name":"Updated Test","version":1,"initial_state":"NEW","states":[{"key":"NEW","name":"New","display_order":0}],"transitions":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/"+orgID.String()+"/workflows/"+workflowID.String(), strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestUpdateWorkflowDefinition_NotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	workflowID := uuid.New()
	mockSvc := &mockWorkflowService{
		mockUpdateDef: func(ctx context.Context, params workflowapp.UpdateWorkflowDefinitionParams) (*workflowdomain.WorkflowDefinition, error) {
			return nil, workflowdomain.ErrWorkflowDefinitionNotFound{DefID: params.ID}
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	body := `{"key":"test_wf","name":"Updated Test","version":1,"initial_state":"NEW","states":[],"transitions":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/"+orgID.String()+"/workflows/"+workflowID.String(), strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestUpdateWorkflowDefinition_NotDraft(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	workflowID := uuid.New()
	mockSvc := &mockWorkflowService{
		mockUpdateDef: func(ctx context.Context, params workflowapp.UpdateWorkflowDefinitionParams) (*workflowdomain.WorkflowDefinition, error) {
			return nil, errors.New("only draft definitions can be updated")
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	body := `{"key":"test_wf","name":"Updated Test","version":1,"initial_state":"NEW","states":[],"transitions":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/"+orgID.String()+"/workflows/"+workflowID.String(), strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestDeleteWorkflowDefinition_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	workflowID := uuid.New()
	mockSvc := &mockWorkflowService{
		defs: []*workflowdomain.WorkflowDefinition{
			{ID: workflowID, TenantID: orgID, Key: "test_wf", Name: "Test", Status: workflowdomain.WorkflowStatusDraft},
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/"+orgID.String()+"/workflows/"+workflowID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestDeleteWorkflowDefinition_NotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	workflowID := uuid.New()
	mockSvc := &mockWorkflowService{
		mockDeleteDef: func(ctx context.Context, tenantID, id uuid.UUID, actorID uuid.UUID) error {
			return workflowdomain.ErrWorkflowDefinitionNotFound{DefID: id}
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/"+orgID.String()+"/workflows/"+workflowID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDeleteWorkflowDefinition_NotDraft(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	workflowID := uuid.New()
	mockSvc := &mockWorkflowService{
		mockDeleteDef: func(ctx context.Context, tenantID, id uuid.UUID, actorID uuid.UUID) error {
			return errors.New("only draft definitions can be deleted")
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/"+orgID.String()+"/workflows/"+workflowID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestArchiveWorkflowDefinition_Success(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	workflowID := uuid.New()
	mockSvc := &mockWorkflowService{}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/workflows/"+workflowID.String()+"/archive", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestArchiveWorkflowDefinition_NotActive(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	workflowID := uuid.New()
	mockSvc := &mockWorkflowService{
		mockArchiveDef: func(ctx context.Context, tenantID, id, actorID uuid.UUID) error {
			return errors.New("only active definitions can be archived")
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/workflows/"+workflowID.String()+"/archive", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestWorkflowDefinition_UpdateTenantIsolation(t *testing.T) {
	ownerOrgID := uuid.New()
	otherOrgID := uuid.New()
	userID := uuid.New()
	defID := uuid.New()

	defs := []*workflowdomain.WorkflowDefinition{
		{ID: defID, Key: "test_wf", Name: "Test", TenantID: ownerOrgID, Status: workflowdomain.WorkflowStatusDraft},
	}
	mockSvc := &mockWorkflowService{
		defs: defs,
		mockUpdateDef: func(ctx context.Context, params workflowapp.UpdateWorkflowDefinitionParams) (*workflowdomain.WorkflowDefinition, error) {
			for _, def := range defs {
				if def.ID == params.ID {
					if def.TenantID != params.TenantID {
						return nil, workflowdomain.ErrTenantViolation{}
					}
					return def, nil
				}
			}
			return nil, workflowdomain.ErrWorkflowDefinitionNotFound{DefID: params.ID}
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), ownerOrgID.String(), "admin")
	body := `{"key":"test_wf","name":"Updated","version":1,"initial_state":"NEW","states":[],"transitions":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/"+otherOrgID.String()+"/workflows/"+defID.String(), strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestWorkflowDefinition_DeleteTenantIsolation(t *testing.T) {
	ownerOrgID := uuid.New()
	otherOrgID := uuid.New()
	userID := uuid.New()
	defID := uuid.New()

	defs := []*workflowdomain.WorkflowDefinition{
		{ID: defID, Key: "test_wf", Name: "Test", TenantID: ownerOrgID, Status: workflowdomain.WorkflowStatusDraft},
	}
	mockSvc := &mockWorkflowService{
		defs: defs,
		mockDeleteDef: func(ctx context.Context, tenantID, id uuid.UUID, actorID uuid.UUID) error {
			for _, def := range defs {
				if def.ID == id {
					if def.TenantID != tenantID {
						return workflowdomain.ErrTenantViolation{}
					}
					return nil
				}
			}
			return workflowdomain.ErrWorkflowDefinitionNotFound{DefID: id}
		},
	}
	r := setupWorkflowRouter(mockSvc)

	token := generateTestJWT("test-secret", userID.String(), ownerOrgID.String(), "admin")
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/"+otherOrgID.String()+"/workflows/"+defID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}
