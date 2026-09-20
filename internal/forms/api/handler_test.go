package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/forms/application"
	"github.com/alrazihi/civora/internal/forms/domain"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFormService struct {
	createFormFn       func(ctx context.Context, params application.CreateFormParams) (*domain.Form, error)
	updateFormFn       func(ctx context.Context, params application.UpdateFormParams) (*domain.Form, error)
	createVersionFn    func(ctx context.Context, params application.CreateVersionParams) (*domain.FormVersion, error)
	publishVersionFn   func(ctx context.Context, params application.PublishVersionParams) (*domain.FormVersion, error)
	archiveFormFn      func(ctx context.Context, params application.ArchiveFormParams) (*domain.Form, error)
	getFormFn          func(ctx context.Context, params application.GetFormParams) (*domain.Form, error)
	listFormsFn        func(ctx context.Context, params application.ListFormsParams) ([]*domain.Form, int, error)
	addFieldFn         func(ctx context.Context, params application.AddFieldParams) (*domain.FormField, error)
	updateFieldFn      func(ctx context.Context, params application.UpdateFieldParams) (*domain.FormField, error)
	deleteFieldFn      func(ctx context.Context, params application.DeleteFieldParams) (*domain.FormField, error)
	getActiveVersionFn func(ctx context.Context, params application.GetActiveVersionParams) (*domain.FormVersion, error)
}

func (m *mockFormService) CreateForm(ctx context.Context, params application.CreateFormParams) (*domain.Form, error) {
	if m.createFormFn != nil {
		return m.createFormFn(ctx, params)
	}
	return nil, nil
}

func (m *mockFormService) UpdateForm(ctx context.Context, params application.UpdateFormParams) (*domain.Form, error) {
	if m.updateFormFn != nil {
		return m.updateFormFn(ctx, params)
	}
	return nil, nil
}

func (m *mockFormService) CreateVersion(ctx context.Context, params application.CreateVersionParams) (*domain.FormVersion, error) {
	if m.createVersionFn != nil {
		return m.createVersionFn(ctx, params)
	}
	return nil, nil
}

func (m *mockFormService) PublishVersion(ctx context.Context, params application.PublishVersionParams) (*domain.FormVersion, error) {
	if m.publishVersionFn != nil {
		return m.publishVersionFn(ctx, params)
	}
	return nil, nil
}

func (m *mockFormService) ArchiveForm(ctx context.Context, params application.ArchiveFormParams) (*domain.Form, error) {
	if m.archiveFormFn != nil {
		return m.archiveFormFn(ctx, params)
	}
	return nil, nil
}

func (m *mockFormService) GetForm(ctx context.Context, params application.GetFormParams) (*domain.Form, error) {
	if m.getFormFn != nil {
		return m.getFormFn(ctx, params)
	}
	return nil, nil
}

func (m *mockFormService) ListForms(ctx context.Context, params application.ListFormsParams) ([]*domain.Form, int, error) {
	if m.listFormsFn != nil {
		return m.listFormsFn(ctx, params)
	}
	return nil, 0, nil
}

func (m *mockFormService) AddField(ctx context.Context, params application.AddFieldParams) (*domain.FormField, error) {
	if m.addFieldFn != nil {
		return m.addFieldFn(ctx, params)
	}
	return nil, nil
}

func (m *mockFormService) UpdateField(ctx context.Context, params application.UpdateFieldParams) (*domain.FormField, error) {
	if m.updateFieldFn != nil {
		return m.updateFieldFn(ctx, params)
	}
	return nil, nil
}

func (m *mockFormService) DeleteField(ctx context.Context, params application.DeleteFieldParams) (*domain.FormField, error) {
	if m.deleteFieldFn != nil {
		return m.deleteFieldFn(ctx, params)
	}
	return nil, nil
}

func (m *mockFormService) GetActiveVersion(ctx context.Context, params application.GetActiveVersionParams) (*domain.FormVersion, error) {
	if m.getActiveVersionFn != nil {
		return m.getActiveVersionFn(ctx, params)
	}
	return nil, nil
}

func setupFormRouter(svc *mockFormService) http.Handler {
	jwtSvc := middleware.NewJWTService("test-secret", time.Hour, 24*time.Hour, "test-issuer")
	h := NewHandler(svc)
	r := chi.NewRouter()
	r.Route("/api/v1/organizations/{orgId}/forms", func(r chi.Router) {
		r.Use(middleware.AuthRequired(jwtSvc, nil))
		r.Use(middleware.RequireSameTenant)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin", "staff"))
			r.Get("/", h.ListForms)
			r.Get("/{formId}", h.GetForm)
			r.Get("/{formId}/active-version", h.GetActiveVersion)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAnyRole("admin"))
			r.Post("/", h.CreateForm)
			r.Put("/{formId}", h.UpdateForm)
			r.Post("/{formId}/versions", h.CreateVersion)
			r.Post("/{formId}/versions/{versionId}/publish", h.PublishVersion)
			r.Post("/{formId}/archive", h.ArchiveForm)
			r.Route("/{formId}/fields", func(r chi.Router) {
				r.Post("/", h.AddField)
				r.Put("/{fieldId}", h.UpdateField)
				r.Delete("/{fieldId}", h.DeleteField)
			})
		})
	})
	return r
}

func generateFormTestJWT(t *testing.T, secret, userID, orgID, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":             userID,
		"organization_id": orgID,
		"role":            role,
		"iss":             "test-issuer",
		"exp":             time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = "primary"
	tokenStr, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return tokenStr
}

func TestCreateForm_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()

	svc := &mockFormService{
		createFormFn: func(ctx context.Context, params application.CreateFormParams) (*domain.Form, error) {
			assert.Equal(t, orgID, params.OrganizationID)
			assert.Equal(t, "test-form", params.Key)
			assert.Equal(t, "Test Form", params.Name)
			assert.Equal(t, userID, params.CreatedByID)
			return &domain.Form{
				ID:             formID,
				OrganizationID: orgID,
				Key:            params.Key,
				Name:           params.Name,
				Description:    params.Description,
				Status:         "ACTIVE",
				CreatedByID:    userID,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	body := `{"key":"test-form","name":"Test Form","description":"A test form"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp shared.APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestCreateForm_InvalidJSON(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	r := setupFormRouter(&mockFormService{})

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms", strings.NewReader("not json"))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateForm_InvalidOrgID(t *testing.T) {
	userID := uuid.New()
	r := setupFormRouter(&mockFormService{})

	token := generateFormTestJWT(t, "test-secret", userID.String(), uuid.New().String(), "admin")
	body := `{"key":"test-form","name":"Test Form"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/not-a-uuid/forms", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCreateForm_DuplicateKey(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	svc := &mockFormService{
		createFormFn: func(ctx context.Context, params application.CreateFormParams) (*domain.Form, error) {
			return nil, fmt.Errorf("%w: %s", application.ErrFormKeyExists, params.Key)
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	body := `{"key":"existing-form","name":"Test Form"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestGetForm_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()

	svc := &mockFormService{
		getFormFn: func(ctx context.Context, params application.GetFormParams) (*domain.Form, error) {
			assert.Equal(t, orgID, params.OrganizationID)
			assert.Equal(t, formID, params.FormID)
			return &domain.Form{
				ID:             formID,
				OrganizationID: orgID,
				Key:            "test-form",
				Name:           "Test Form",
				Status:         "ACTIVE",
				CreatedByID:    userID,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp shared.APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestGetForm_NotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()

	svc := &mockFormService{
		getFormFn: func(ctx context.Context, params application.GetFormParams) (*domain.Form, error) {
			return nil, application.ErrFormNotFound
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListForms_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()

	svc := &mockFormService{
		listFormsFn: func(ctx context.Context, params application.ListFormsParams) ([]*domain.Form, int, error) {
			assert.Equal(t, orgID, params.OrganizationID)
			assert.Equal(t, 20, params.Limit)
			assert.Equal(t, 0, params.Offset)
			return []*domain.Form{
				{ID: uuid.New(), OrganizationID: orgID, Key: "form-1", Name: "Form 1", Status: "ACTIVE", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
				{ID: uuid.New(), OrganizationID: orgID, Key: "form-2", Name: "Form 2", Status: "ACTIVE", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
			}, 2, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/forms", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdateForm_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()

	svc := &mockFormService{
		updateFormFn: func(ctx context.Context, params application.UpdateFormParams) (*domain.Form, error) {
			assert.Equal(t, orgID, params.OrganizationID)
			assert.Equal(t, formID, params.ID)
			assert.Equal(t, "Updated Name", params.Name)
			return &domain.Form{
				ID:             formID,
				OrganizationID: orgID,
				Key:            "test-form",
				Name:           params.Name,
				Status:         "ACTIVE",
				CreatedByID:    userID,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	body := `{"name":"Updated Name","description":"Updated description"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String(), strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCreateVersion_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()
	versionID := uuid.New()

	svc := &mockFormService{
		createVersionFn: func(ctx context.Context, params application.CreateVersionParams) (*domain.FormVersion, error) {
			assert.Equal(t, orgID, params.OrganizationID)
			assert.Equal(t, formID, params.FormID)
			return &domain.FormVersion{
				ID:             versionID,
				FormID:         formID,
				OrganizationID: orgID,
				Version:        1,
				Status:         "DRAFT",
				CreatedByID:    userID,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/versions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestPublishVersion_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()
	versionID := uuid.New()

	svc := &mockFormService{
		publishVersionFn: func(ctx context.Context, params application.PublishVersionParams) (*domain.FormVersion, error) {
			assert.Equal(t, orgID, params.OrganizationID)
			assert.Equal(t, versionID, params.VersionID)
			now := time.Now().UTC()
			return &domain.FormVersion{
				ID:             versionID,
				FormID:         formID,
				OrganizationID: orgID,
				Version:        1,
				Status:         "PUBLISHED",
				PublishedAt:    &now,
				CreatedByID:    userID,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/versions/"+versionID.String()+"/publish", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestArchiveForm_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()

	svc := &mockFormService{
		archiveFormFn: func(ctx context.Context, params application.ArchiveFormParams) (*domain.Form, error) {
			assert.Equal(t, orgID, params.OrganizationID)
			assert.Equal(t, formID, params.FormID)
			return &domain.Form{
				ID:             formID,
				OrganizationID: orgID,
				Key:            "test-form",
				Name:           "Test Form",
				Status:         "ARCHIVED",
				CreatedByID:    userID,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/archive", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAddField_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()
	versionID := uuid.New()
	fieldID := uuid.New()

	svc := &mockFormService{
		addFieldFn: func(ctx context.Context, params application.AddFieldParams) (*domain.FormField, error) {
			assert.Equal(t, orgID, params.OrganizationID)
			assert.Equal(t, formID, params.FormID)
			assert.Equal(t, versionID, params.VersionID)
			assert.Equal(t, "email", params.Key)
			assert.Equal(t, domain.FieldTypeEmail, params.Type)
			return &domain.FormField{
				ID:             fieldID,
				FormID:         formID,
				FormVersionID:  versionID,
				OrganizationID: orgID,
				Key:            params.Key,
				Label:          params.Label,
				Type:           params.Type,
				Required:       params.Required,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	body := `{"version_id":"` + versionID.String() + `","key":"email","label":"Email Address","type":"EMAIL","required":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/fields", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestAddField_InvalidFieldType(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()
	versionID := uuid.New()

	r := setupFormRouter(&mockFormService{})

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	body := `{"version_id":"` + versionID.String() + `","key":"test","label":"Test","type":"INVALID_TYPE","required":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/fields", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAddField_InvalidVersionID(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()

	r := setupFormRouter(&mockFormService{})

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	body := `{"version_id":"not-a-uuid","key":"test","label":"Test","type":"TEXT","required":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/fields", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAddField_DuplicateKey(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()
	versionID := uuid.New()

	svc := &mockFormService{
		addFieldFn: func(ctx context.Context, params application.AddFieldParams) (*domain.FormField, error) {
			return nil, fmt.Errorf("%w: %s", application.ErrFormFieldDuplicate, params.Key)
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	body := `{"version_id":"` + versionID.String() + `","key":"existing","label":"Test","type":"TEXT","required":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/fields", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestUpdateField_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()
	fieldID := uuid.New()
	versionID := uuid.New()

	svc := &mockFormService{
		updateFieldFn: func(ctx context.Context, params application.UpdateFieldParams) (*domain.FormField, error) {
			assert.Equal(t, orgID, params.OrganizationID)
			assert.Equal(t, fieldID, params.FieldID)
			assert.Equal(t, "Updated Label", params.Label)
			return &domain.FormField{
				ID:             fieldID,
				FormID:         formID,
				FormVersionID:  versionID,
				OrganizationID: orgID,
				Key:            "test",
				Label:          params.Label,
				Type:           domain.FieldTypeText,
				Required:       params.Required,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	body := `{"label":"Updated Label","required":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/fields/"+fieldID.String(), strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestDeleteField_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()
	fieldID := uuid.New()
	versionID := uuid.New()

	svc := &mockFormService{
		deleteFieldFn: func(ctx context.Context, params application.DeleteFieldParams) (*domain.FormField, error) {
			assert.Equal(t, orgID, params.OrganizationID)
			assert.Equal(t, fieldID, params.FieldID)
			return &domain.FormField{
				ID:             fieldID,
				FormID:         formID,
				FormVersionID:  versionID,
				OrganizationID: orgID,
				Key:            "deleted",
				Label:          "Deleted",
				Type:           domain.FieldTypeText,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/fields/"+fieldID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetActiveVersion_ValidRequest(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()
	versionID := uuid.New()
	now := time.Now().UTC()

	svc := &mockFormService{
		getActiveVersionFn: func(ctx context.Context, params application.GetActiveVersionParams) (*domain.FormVersion, error) {
			assert.Equal(t, orgID, params.OrganizationID)
			assert.Equal(t, formID, params.FormID)
			return &domain.FormVersion{
				ID:             versionID,
				FormID:         formID,
				OrganizationID: orgID,
				Version:        1,
				Status:         "PUBLISHED",
				PublishedAt:    &now,
				CreatedByID:    userID,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/active-version", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetActiveVersion_NotFound(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()

	svc := &mockFormService{
		getActiveVersionFn: func(ctx context.Context, params application.GetActiveVersionParams) (*domain.FormVersion, error) {
			return nil, application.ErrFormVersionNotFound
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/active-version", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAuthorization_StaffCanReadForms(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()

	svc := &mockFormService{
		getFormFn: func(ctx context.Context, params application.GetFormParams) (*domain.Form, error) {
			return &domain.Form{
				ID:             formID,
				OrganizationID: orgID,
				Key:            "test-form",
				Name:           "Test Form",
				Status:         "ACTIVE",
				CreatedByID:    userID,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "staff")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthorization_StaffCannotCreateForm(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	r := setupFormRouter(&mockFormService{})

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "staff")
	body := `{"key":"test-form","name":"Test Form"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAuthorization_StaffCannotAddField(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()
	versionID := uuid.New()
	r := setupFormRouter(&mockFormService{})

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "staff")
	body := `{"version_id":"` + versionID.String() + `","key":"test","label":"Test","type":"TEXT","required":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/fields", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAuthorization_StaffCannotArchiveForm(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()
	r := setupFormRouter(&mockFormService{})

	token := generateFormTestJWT(t, "test-secret", userID.String(), orgID.String(), "staff")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String()+"/archive", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAuthorization_CrossTenantGetForm(t *testing.T) {
	tokenOrgID := uuid.New()
	requestOrgID := uuid.New()
	userID := uuid.New()
	formID := uuid.New()

	svc := &mockFormService{
		getFormFn: func(ctx context.Context, params application.GetFormParams) (*domain.Form, error) {
			return &domain.Form{
				ID:             formID,
				OrganizationID: requestOrgID,
				Key:            "test-form",
				Name:           "Test Form",
				Status:         "ACTIVE",
				CreatedByID:    userID,
				CreatedAt:      time.Now().UTC(),
				UpdatedAt:      time.Now().UTC(),
			}, nil
		},
	}
	r := setupFormRouter(svc)

	token := generateFormTestJWT(t, "test-secret", userID.String(), tokenOrgID.String(), "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+requestOrgID.String()+"/forms/"+formID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAuthorization_CrossTenantCreateForm(t *testing.T) {
	tokenOrgID := uuid.New()
	requestOrgID := uuid.New()
	userID := uuid.New()

	r := setupFormRouter(&mockFormService{})

	token := generateFormTestJWT(t, "test-secret", userID.String(), tokenOrgID.String(), "admin")
	body := `{"key":"test-form","name":"Test Form"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+requestOrgID.String()+"/forms", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAuthorization_NoToken(t *testing.T) {
	orgID := uuid.New()
	formID := uuid.New()
	r := setupFormRouter(&mockFormService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/forms/"+formID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
