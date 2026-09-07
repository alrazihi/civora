package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTService_VerifyToken(t *testing.T) {
	svc := NewJWTService("test-secret-key-for-testing-1234567890")

	userID := uuid.New().String()
	orgID := uuid.New().String()

	h := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":             userID,
		"organization_id": orgID,
		"role":            "admin",
		"exp":             time.Now().Add(time.Hour).Unix(),
	})
	token, err := h.SignedString([]byte("test-secret-key-for-testing-1234567890"))
	require.NoError(t, err)

	uid, oid, role, exp, err := svc.VerifyToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, uid)
	assert.Equal(t, orgID, oid)
	assert.Equal(t, "admin", role)
	assert.True(t, exp.After(time.Now()))
}

func TestJWTService_VerifyToken_InvalidSignature(t *testing.T) {
	svc := NewJWTService("secret-a")

	h := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":             "user",
		"organization_id": "org",
		"role":            "admin",
		"exp":             time.Now().Add(time.Hour).Unix(),
	})
	token, err := h.SignedString([]byte("secret-b"))
	require.NoError(t, err)

	_, _, _, _, err = svc.VerifyToken(token)
	assert.Error(t, err)
}

func TestJWTService_VerifyToken_Expired(t *testing.T) {
	svc := NewJWTService("secret")

	h := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":             "user",
		"organization_id": "org",
		"role":            "admin",
		"exp":             time.Now().Add(-time.Hour).Unix(),
	})
	token, err := h.SignedString([]byte("secret"))
	require.NoError(t, err)

	_, _, _, _, err = svc.VerifyToken(token)
	assert.Error(t, err)
}

func TestAuthRequired_MissingToken(t *testing.T) {
	svc := NewJWTService("secret")
	mw := AuthRequired(svc)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthRequired_InvalidToken(t *testing.T) {
	svc := NewJWTService("secret")
	mw := AuthRequired(svc)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireSameTenant_AllowsMatchingOrg(t *testing.T) {
	svc := NewJWTService("secret")
	orgID := uuid.New().String()

	handler := AuthRequired(svc)(RequireSameTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	r := chi.NewRouter()
	r.Handle("/test/{orgId}", handler)

	token := generateTestToken(t, "secret", "user-1", orgID, "admin")
	req := httptest.NewRequest(http.MethodGet, "/test/"+orgID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireSameTenant_BlocksMismatchedOrg(t *testing.T) {
	svc := NewJWTService("secret")
	jwtOrgID := uuid.New().String()
	pathOrgID := uuid.New().String()

	handler := AuthRequired(svc)(RequireSameTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})))

	r := chi.NewRouter()
	r.Handle("/test/{orgId}", handler)

	token := generateTestToken(t, "secret", "user-1", jwtOrgID, "admin")
	req := httptest.NewRequest(http.MethodGet, "/test/"+pathOrgID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireRole_AllowsAuthorized(t *testing.T) {
	svc := NewJWTService("secret")
	orgID := uuid.New().String()

	handler := AuthRequired(svc)(RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	r := chi.NewRouter()
	r.Handle("/test/{orgId}", handler)

	token := generateTestToken(t, "secret", "user-1", orgID, "admin")
	req := httptest.NewRequest(http.MethodGet, "/test/"+orgID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRole_BlocksUnauthorized(t *testing.T) {
	svc := NewJWTService("secret")
	orgID := uuid.New().String()

	handler := AuthRequired(svc)(RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	})))

	r := chi.NewRouter()
	r.Handle("/test/{orgId}", handler)

	token := generateTestToken(t, "secret", "user-1", orgID, "staff")
	req := httptest.NewRequest(http.MethodGet, "/test/"+orgID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireAnyRole_AllowsAnyListed(t *testing.T) {
	svc := NewJWTService("secret")
	orgID := uuid.New().String()

	handler := AuthRequired(svc)(RequireAnyRole("admin", "staff")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	r := chi.NewRouter()
	r.Handle("/test/{orgId}", handler)

	token := generateTestToken(t, "secret", "user-1", orgID, "staff")
	req := httptest.NewRequest(http.MethodGet, "/test/"+orgID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequestID_GeneratesID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r)
		assert.NotEmpty(t, id)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
}

func TestRequestID_UsesClientID(t *testing.T) {
	clientID := "client-provided-id"
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r)
		assert.Equal(t, clientID, id)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", clientID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, clientID, rec.Header().Get("X-Request-ID"))
}

func TestGetTenantID_NotAuthenticated(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	assert.Empty(t, GetTenantID(r))
}

func TestGetUserID_NotAuthenticated(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	assert.Empty(t, GetUserID(r))
}

func TestGetUserRole_NotAuthenticated(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	assert.Empty(t, GetUserRole(r))
}

func generateTestToken(t *testing.T, secret, userID, orgID, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":             userID,
		"organization_id": orgID,
		"role":            role,
		"exp":             time.Now().Add(time.Hour).Unix(),
	}
	h := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := h.SignedString([]byte(secret))
	require.NoError(t, err)
	return token
}

func TestRateLimit_AllowsRequests(t *testing.T) {
	rl := NewRateLimiter(1000, 100)
	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := chi.NewRouter()
	r.Handle("/test", handler)

	for i := 0; i < 15; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should succeed", i)
	}
}

func TestRateLimit_RejectsExcess(t *testing.T) {
	rl := NewRateLimiter(100, 2)
	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := chi.NewRouter()
	r.Handle("/test", handler)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}
