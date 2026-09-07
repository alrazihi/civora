package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alrazihi/civora/internal/organizations/application"
	"github.com/alrazihi/civora/internal/organizations/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOrgService struct {
	createOrgFn func(ctx context.Context, params application.CreateOrganizationParams) (*domain.Organization, error)
	getOrgFn    func(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
	getBySlugFn func(ctx context.Context, slug string) (*domain.Organization, error)
}

func (m *mockOrgService) CreateOrganization(ctx context.Context, params application.CreateOrganizationParams) (*domain.Organization, error) {
	if m.createOrgFn != nil {
		return m.createOrgFn(ctx, params)
	}
	return nil, nil
}

func (m *mockOrgService) GetOrganization(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	if m.getOrgFn != nil {
		return m.getOrgFn(ctx, id)
	}
	return nil, nil
}

func (m *mockOrgService) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	if m.getBySlugFn != nil {
		return m.getBySlugFn(ctx, slug)
	}
	return nil, nil
}

func TestCreateOrganization_ValidRequest(t *testing.T) {
	svc := &mockOrgService{
		createOrgFn: func(ctx context.Context, params application.CreateOrganizationParams) (*domain.Organization, error) {
			return &domain.Organization{
				ID:          uuid.New(),
				Name:        params.Name,
				Description: params.Description,
				Slug:        params.Slug,
			}, nil
		},
	}
	h := NewHandler(svc)

	body := `{"name":"Test Org","slug":"test-org","description":"Test description"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.CreateOrganization(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp shared.APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestCreateOrganization_InvalidJSON(t *testing.T) {
	h := NewHandler(&mockOrgService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	h.CreateOrganization(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateOrganization_Conflict(t *testing.T) {
	svc := &mockOrgService{
		createOrgFn: func(ctx context.Context, params application.CreateOrganizationParams) (*domain.Organization, error) {
			return nil, application.ErrOrgSlugTaken
		},
	}
	h := NewHandler(svc)

	body := `{"name":"Test Org","slug":"taken-slug"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.CreateOrganization(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}
