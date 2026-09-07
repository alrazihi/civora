package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/cases/application"
	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCaseService struct {
	createCaseFn   func(ctx context.Context, params application.CreateCaseParams) (*domain.Case, error)
	changeStatusFn func(ctx context.Context, params application.ChangeCaseStatusParams) (*domain.Case, error)
	assignCaseFn   func(ctx context.Context, params application.AssignCaseParams) (*domain.Case, error)
	getCaseFn      func(ctx context.Context, orgID, id uuid.UUID) (*domain.Case, error)
	listCasesFn    func(ctx context.Context, orgID uuid.UUID, limit, offset int, filter domain.CaseFilter) ([]*domain.Case, int, error)
}

func (m *mockCaseService) CreateCase(ctx context.Context, params application.CreateCaseParams) (*domain.Case, error) {
	if m.createCaseFn != nil {
		return m.createCaseFn(ctx, params)
	}
	return nil, nil
}

func (m *mockCaseService) ListCases(ctx context.Context, orgID uuid.UUID, limit, offset int, filter domain.CaseFilter) ([]*domain.Case, int, error) {
	if m.listCasesFn != nil {
		return m.listCasesFn(ctx, orgID, limit, offset, filter)
	}
	return nil, 0, nil
}

func (m *mockCaseService) GetCase(ctx context.Context, orgID, id uuid.UUID) (*domain.Case, error) {
	if m.getCaseFn != nil {
		return m.getCaseFn(ctx, orgID, id)
	}
	return nil, nil
}

func (m *mockCaseService) ChangeStatus(ctx context.Context, params application.ChangeCaseStatusParams) (*domain.Case, error) {
	if m.changeStatusFn != nil {
		return m.changeStatusFn(ctx, params)
	}
	return nil, nil
}

func (m *mockCaseService) AssignCase(ctx context.Context, params application.AssignCaseParams) (*domain.Case, error) {
	if m.assignCaseFn != nil {
		return m.assignCaseFn(ctx, params)
	}
	return nil, nil
}

func setupCaseRouter(svc CaseService) http.Handler {
	jwtSvc := middleware.NewJWTService("test-secret", time.Hour, "test-issuer")
	h := NewHandler(svc)
	r := chi.NewRouter()
	r.Route("/api/v1/organizations/{orgId}/cases", func(r chi.Router) {
		r.Use(middleware.AuthRequired(jwtSvc))
		r.Use(middleware.RequireSameTenant)
		r.Post("/", h.CreateCase)
		r.Get("/", h.ListCases)
		r.Get("/{caseId}", h.GetCase)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Post("/{caseId}/transitions", h.ChangeCaseStatus)
			r.Post("/{caseId}/assign", h.AssignCase)
		})
	})
	return r
}

func generateTestJWT(t *testing.T, secret, userID, orgID, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":             userID,
		"organization_id": orgID,
		"role":            role,
		"iss":             "test-issuer",
		"exp":             time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return tokenStr
}

func TestCreateCase_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	svc := &mockCaseService{
		createCaseFn: func(ctx context.Context, params application.CreateCaseParams) (*domain.Case, error) {
			return domain.NewCase(orgID, userID, params.Title, params.Description, params.ServiceType, params.Priority, params.PersonID)
		},
	}
	r := setupCaseRouter(svc)

	token := generateTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	body := `{"title":"Emergency Request","description":"Need help","service_type":"EMERGENCY","priority":"HIGH"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/cases", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp shared.APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestCreateCase_InvalidJSON(t *testing.T) {
	r := setupCaseRouter(&mockCaseService{})

	orgID := uuid.New()
	token := generateTestJWT(t, "test-secret", uuid.New().String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/cases", strings.NewReader("not json"))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateCase_TitleTooLong(t *testing.T) {
	svc := &mockCaseService{
		createCaseFn: func(ctx context.Context, params application.CreateCaseParams) (*domain.Case, error) {
			return domain.NewCase(uuid.New(), uuid.New(), params.Title, params.Description, params.ServiceType, params.Priority, params.PersonID)
		},
	}
	r := setupCaseRouter(svc)

	orgID := uuid.New()
	userID := uuid.New()
	token := generateTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	longTitle := strings.Repeat("x", 201)
	body := `{"title":"` + longTitle + `","description":"desc"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/cases", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateCase_DescriptionTooLong(t *testing.T) {
	svc := &mockCaseService{
		createCaseFn: func(ctx context.Context, params application.CreateCaseParams) (*domain.Case, error) {
			return domain.NewCase(uuid.New(), uuid.New(), params.Title, params.Description, params.ServiceType, params.Priority, params.PersonID)
		},
	}
	r := setupCaseRouter(svc)

	orgID := uuid.New()
	token := generateTestJWT(t, "test-secret", uuid.New().String(), orgID.String(), "admin")
	longDesc := strings.Repeat("x", 10001)
	body := `{"title":"Valid Title","description":"` + longDesc + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/cases", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateCase_EmptyTitle(t *testing.T) {
	svc := &mockCaseService{
		createCaseFn: func(ctx context.Context, params application.CreateCaseParams) (*domain.Case, error) {
			if strings.TrimSpace(params.Title) == "" {
				return nil, application.ErrCaseInvalidInput
			}
			return domain.NewCase(uuid.New(), uuid.New(), params.Title, params.Description, params.ServiceType, params.Priority, params.PersonID)
		},
	}
	r := setupCaseRouter(svc)

	orgID := uuid.New()
	token := generateTestJWT(t, "test-secret", uuid.New().String(), orgID.String(), "admin")
	body := `{"title":"","description":"desc"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/cases", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
