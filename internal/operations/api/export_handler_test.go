package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/operations/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockExportService struct {
	mock.Mock
}

func (m *mockExportService) GenerateExport(ctx context.Context, orgID uuid.UUID, req domain.ExportRequest) (*domain.ExportResult, error) {
	args := m.Called(ctx, orgID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ExportResult), args.Error(1)
}

var _ domain.ExportService = (*mockExportService)(nil)

func generateTestToken(t *testing.T, userID, orgID, role string) string {
	t.Helper()
	svc := middleware.NewJWTService("test-secret-key-for-testing-1234567890", time.Hour, "civora")
	token, err := svc.GenerateToken(userID, orgID, role)
	require.NoError(t, err)
	return token
}

func newExportTestRouter(svc domain.ExportService) *chi.Mux {
	r := chi.NewRouter()
	handler := NewExportHandler(svc)

	jwtSvc := middleware.NewJWTService("test-secret-key-for-testing-1234567890", time.Hour, "civora")
	authMiddleware := middleware.AuthRequired(jwtSvc)

	handler.RegisterRoutes(r, authMiddleware)
	return r
}

func TestExportHandler_UnauthorizedExport(t *testing.T) {
	svc := new(mockExportService)
	r := newExportTestRouter(svc)

	req := httptest.NewRequest("GET", "/api/v1/organizations/org-test/operations/export", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestExportHandler_CrossTenantExport(t *testing.T) {
	svc := new(mockExportService)
	r := newExportTestRouter(svc)

	token := generateTestToken(t, "user-1", "org-test", "admin")
	req := httptest.NewRequest("GET", "/api/v1/organizations/other-org/operations/export", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var apiResp shared.APIResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&apiResp))
	require.NotNil(t, apiResp.Error)
	assert.Equal(t, "FORBIDDEN", apiResp.Error.Code)
}

func TestExportHandler_AuthorizedExport_JSON(t *testing.T) {
	svc := new(mockExportService)
	orgID := uuid.New()
	bucket := time.Now().UTC()

	svc.On("GenerateExport", mock.Anything, orgID, mock.Anything).Return(&domain.ExportResult{
		ContentType: "application/json",
		Filename:    "civora-all-daily.json",
		Body:        []byte(`{"metadata":{}}`),
		Metadata: domain.ExportMetadata{
			GeneratedAt:    bucket,
			OrganizationID: orgID,
			MetricVersion:  domain.ExportMetricVersion,
		},
	}, nil)

	r := newExportTestRouter(svc)

	token := generateTestToken(t, "user-1", orgID.String(), "admin")
	req := httptest.NewRequest("GET", "/api/v1/organizations/"+orgID.String()+"/operations/export?format=json&period=daily&bucket="+bucket.Format(time.RFC3339), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "civora-all-daily.json")
	assert.Contains(t, w.Header().Get("X-Export-Metric-Version"), domain.ExportMetricVersion)
}

func TestExportHandler_AuthorizedExport_CSV(t *testing.T) {
	svc := new(mockExportService)
	orgID := uuid.New()
	bucket := time.Now().UTC()

	svc.On("GenerateExport", mock.Anything, orgID, mock.Anything).Return(&domain.ExportResult{
		ContentType: "text/csv",
		Filename:    "civora-operations-daily.csv",
		Body:        []byte(`{"operations_metrics":{"case_volume":{"total_cases":10,"by_status":{"open":5,"closed":5}}}}`),
		Metadata: domain.ExportMetadata{
			GeneratedAt:    bucket,
			OrganizationID: orgID,
			MetricVersion:  domain.ExportMetricVersion,
		},
	}, nil)

	r := newExportTestRouter(svc)

	token := generateTestToken(t, "user-1", orgID.String(), "admin")
	req := httptest.NewRequest("GET", "/api/v1/organizations/"+orgID.String()+"/operations/export?format=csv&scope=operations&period=daily&bucket="+bucket.Format(time.RFC3339), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "section,metric,value,unit,category")
}

func TestExportHandler_SensitiveFieldExclusion(t *testing.T) {
	svc := new(mockExportService)
	orgID := uuid.New()
	bucket := time.Now().UTC()

	svc.On("GenerateExport", mock.Anything, orgID, mock.Anything).Return(&domain.ExportResult{
		ContentType: "text/plain",
		Filename:    "civora-all-daily-report.txt",
		Body:        []byte(`{"operations_metrics":{"case_volume":{"total_cases":10,"password":"secret123"}}}`),
		Metadata: domain.ExportMetadata{
			GeneratedAt:    bucket,
			OrganizationID: orgID,
			MetricVersion:  domain.ExportMetricVersion,
		},
	}, nil)

	r := newExportTestRouter(svc)

	token := generateTestToken(t, "user-1", orgID.String(), "admin")
	req := httptest.NewRequest("GET", "/api/v1/organizations/"+orgID.String()+"/operations/export?format=report&scope=all&period=daily&bucket="+bucket.Format(time.RFC3339), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "[REDACTED]")
	assert.NotContains(t, body, "secret123")
	assert.Contains(t, body, "| total_cases | 10 |")
}
