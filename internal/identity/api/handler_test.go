package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alrazihi/civora/internal/identity/application"
	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockIdentityService struct {
	createUserFn func(ctx context.Context, params application.CreateUserParams) (*domain.User, error)
	listUsersFn  func(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.User, int, error)
}

func (m *mockIdentityService) CreateUser(ctx context.Context, params application.CreateUserParams) (*domain.User, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, params)
	}
	return nil, nil
}

func (m *mockIdentityService) Authenticate(ctx context.Context, params application.AuthenticateParams) (*application.AuthenticateResult, error) {
	return nil, nil
}

func (m *mockIdentityService) ListUsers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.User, int, error) {
	if m.listUsersFn != nil {
		return m.listUsersFn(ctx, orgID, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockIdentityService) GetUser(ctx context.Context, orgID, userID uuid.UUID) (*domain.User, error) {
	return nil, nil
}

func setupRegisterRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1/organizations/{orgId}/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
	})
	return r
}

func TestRegister_ValidRequest(t *testing.T) {
	svc := &mockIdentityService{
		createUserFn: func(ctx context.Context, params application.CreateUserParams) (*domain.User, error) {
			return &domain.User{
				ID:             uuid.New(),
				OrganizationID: params.OrganizationID,
				Email:          params.Email,
				Name:           params.Name,
			}, nil
		},
	}
	r := setupRegisterRouter(NewHandler(svc))

	orgID := uuid.New().String()
	body := `{"email":"user@example.com","name":"Test User","password":"password1234"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID+"/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp shared.APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestRegister_InvalidJSON(t *testing.T) {
	r := setupRegisterRouter(NewHandler(&mockIdentityService{}))

	orgID := uuid.New().String()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID+"/auth/register", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegister_WeakPasswordRejected(t *testing.T) {
	svc := &mockIdentityService{
		createUserFn: func(ctx context.Context, params application.CreateUserParams) (*domain.User, error) {
			return nil, application.ErrWeakPassword
		},
	}
	r := setupRegisterRouter(NewHandler(svc))

	orgID := uuid.New().String()
	body := `{"email":"user@example.com","name":"Test","password":"short1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID+"/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var resp shared.APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error.Message, "password")
}

func TestRegister_EmailAlreadyExists(t *testing.T) {
	svc := &mockIdentityService{
		createUserFn: func(ctx context.Context, params application.CreateUserParams) (*domain.User, error) {
			return nil, application.ErrEmailAlreadyExists
		},
	}
	r := setupRegisterRouter(NewHandler(svc))

	orgID := uuid.New().String()
	body := `{"email":"user@example.com","name":"Test","password":"password1234"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID+"/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
	var resp shared.APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error.Message, "email")
}

func TestRegister_RoleNameIgnored(t *testing.T) {
	created := false
	svc := &mockIdentityService{
		createUserFn: func(ctx context.Context, params application.CreateUserParams) (*domain.User, error) {
			created = true
			_ = ctx
			return &domain.User{
				ID:             uuid.New(),
				OrganizationID: params.OrganizationID,
				Email:          params.Email,
				Name:           params.Name,
			}, nil
		},
	}
	r := setupRegisterRouter(NewHandler(svc))

	orgID := uuid.New().String()
	body := `{"email":"user@example.com","name":"Test","password":"password1234","role_name":"admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID+"/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.True(t, created, "CreateUser should be called")
}
