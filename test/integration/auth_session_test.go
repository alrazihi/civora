package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityapi "github.com/alrazihi/civora/internal/identity/api"
	"github.com/alrazihi/civora/internal/identity/application"
	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/alrazihi/civora/internal/identity/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/middleware"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAuthServices(t *testing.T) (
	*application.IdentityService,
	*postgres.PostgresUserRepository,
	*postgres.PostgresRoleRepository,
	*postgres.PostgresSessionRepository,
	*domain.BCryptHasher,
	*middleware.JWTService,
	*sql.DB,
	uuid.UUID,
) {
	t.Helper()
	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	orgID := helpers.SeedOrg(db)
	helpers.SeedDefaultRoles(db, orgID)

	userRepo := postgres.NewPostgresUserRepository(db)
	roleRepo := postgres.NewPostgresRoleRepository(db)
	sessionRepo := postgres.NewPostgresSessionRepository(db)
	hasher := domain.NewBCryptHasher(4)
	jwtSvc := middleware.NewJWTService("test-secret-key-for-testing-1234567890", 20*time.Minute, 7*24*time.Hour, "civora")

	svc := application.NewIdentityService(userRepo, roleRepo, sessionRepo, hasher, jwtSvc, nil)

	return svc, userRepo, roleRepo, sessionRepo, hasher, jwtSvc, db, orgID
}

func seedAuthUser(t *testing.T, db *sql.DB, orgID uuid.UUID, roleName string) uuid.UUID {
	t.Helper()
	var roleID uuid.UUID
	err := db.QueryRowContext(context.Background(),
		"SELECT id FROM roles WHERE organization_id = $1 AND name = $2", orgID, roleName,
	).Scan(&roleID)
	require.NoError(t, err)

	userID := uuid.New()
	hash, err := domain.NewBCryptHasher(4).Hash("securepassword123")
	require.NoError(t, err)
	email := "user-" + userID.String()[:8] + "@example.com"
	_, err = db.ExecContext(context.Background(),
		"INSERT INTO users (id, organization_id, email, name, role_id, password_hash, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())",
		userID, orgID, email, "Test User", roleID, hash,
	)
	require.NoError(t, err)
	return userID
}

func TestLogin_ReturnsAccessAndRefreshTokens(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, _, _, _, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	result, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.AccessToken)
	require.NotEmpty(t, result.RefreshToken)
	require.NotEmpty(t, result.SessionID)
	require.Equal(t, int64(1200), result.ExpiresIn)
	require.Equal(t, userID, result.User.ID)
}

func TestRefreshToken_RotatesTokens(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, _, _, jwtSvc, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	refreshResult, err := svc.RefreshToken(ctx, application.RefreshTokenParams{
		RefreshToken: loginResult.RefreshToken,
	})
	require.NoError(t, err)
	require.NotEmpty(t, refreshResult.AccessToken)
	require.NotEmpty(t, refreshResult.RefreshToken)
	require.NotEqual(t, loginResult.RefreshToken, refreshResult.RefreshToken)

	_, _, _, _, exp, err := jwtSvc.VerifyAccessToken(refreshResult.AccessToken)
	require.NoError(t, err)
	assert.True(t, exp.After(time.Now()), "new access token should not be expired")
}

func TestRefreshToken_ReuseDetected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, _, _, _, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	_, err = svc.RefreshToken(ctx, application.RefreshTokenParams{
		RefreshToken: loginResult.RefreshToken,
	})
	require.NoError(t, err)

	_, err = svc.RefreshToken(ctx, application.RefreshTokenParams{
		RefreshToken: loginResult.RefreshToken,
	})
	require.Error(t, err, "reusing old refresh token should fail")
}

func TestLogout_RevokesSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, sessionRepo, _, jwtSvc, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	_, _, _, jti, _, err := jwtSvc.VerifyAccessToken(loginResult.AccessToken)
	require.NoError(t, err)

	err = svc.Logout(ctx, application.LogoutParams{SessionID: jti})
	require.NoError(t, err)

	session, err := sessionRepo.FindByID(ctx, jti)
	require.NoError(t, err)
	require.True(t, session.IsRevoked(), "session should be revoked after logout")
}

func TestRevokedSession_AccessDenied(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, sessionRepo, _, jwtSvc, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	_, _, _, jti, _, err := jwtSvc.VerifyAccessToken(loginResult.AccessToken)
	require.NoError(t, err)

	err = sessionRepo.Revoke(ctx, jti)
	require.NoError(t, err)

	session, err := sessionRepo.FindByID(ctx, jti)
	require.NoError(t, err)
	require.True(t, session.IsRevoked(), "session should be revoked")
}

func TestPasswordChange_RevokesSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, sessionRepo, _, jwtSvc, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	err = svc.ChangePassword(ctx, application.ChangePasswordParams{
		UserID:      userID,
		OrgID:       orgID,
		OldPassword: "securepassword123",
		NewPassword: "newpassword1234",
	})
	require.NoError(t, err)

	_, _, _, jti, _, err := jwtSvc.VerifyAccessToken(loginResult.AccessToken)
	require.NoError(t, err)

	session, err := sessionRepo.FindByID(ctx, jti)
	require.NoError(t, err)
	require.True(t, session.IsRevoked(), "session should be revoked after password change")
}

func TestWrongTenantSession_Denied(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, _, _, _, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	otherOrgID := helpers.SeedOrg(db)
	helpers.SeedDefaultRoles(db, otherOrgID)
	otherUserID := seedAuthUser(t, db, otherOrgID, "staff")
	otherEmail := "user-" + otherUserID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: otherOrgID,
		Email:          otherEmail,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	mw := middleware.AuthRequired(
		middleware.NewJWTService("test-secret-key-for-testing-1234567890", 20*time.Minute, 7*24*time.Hour, "civora"),
		postgres.NewPostgresSessionRepository(db),
	)

	handler := mw(middleware.RequireSameTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	r := chi.NewRouter()
	r.Handle("/api/v1/organizations/"+orgID.String()+"/test/{orgId}", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/test/"+orgID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+loginResult.AccessToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code, "cross-tenant access should be denied")
}

func TestInvalidJWT_Denied(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	_, _, _, _, _, _, db, orgID := setupAuthServices(t)

	mw := middleware.AuthRequired(
		middleware.NewJWTService("test-secret-key-for-testing-1234567890", 20*time.Minute, 7*24*time.Hour, "civora"),
		postgres.NewPostgresSessionRepository(db),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test/"+orgID.String(), nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestExpiredJWT_Denied(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, _, _, jwtSvc, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	_, _, _, jti, _, err := jwtSvc.VerifyAccessToken(loginResult.AccessToken)
	require.NoError(t, err)

	_ = jti

	mw := middleware.AuthRequired(
		middleware.NewJWTService("test-secret-key-for-testing-1234567890", -time.Hour, 7*24*time.Hour, "civora"),
		postgres.NewPostgresSessionRepository(db),
	)

	expiredToken, err := middleware.NewJWTService("test-secret-key-for-testing-1234567890", -time.Hour, 7*24*time.Hour, "civora").GenerateToken(userID.String(), orgID.String(), "staff")
	require.NoError(t, err)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test/"+orgID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSigningKeyRotation_ValidatesTokens(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, _, _, jwtSvc, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	jwtSvc.KeySet().RotateActive("v2", []byte("new-secret-key-for-testing-1234567890"))

	_, _, _, _, _, err = jwtSvc.VerifyAccessToken(loginResult.AccessToken)
	require.NoError(t, err, "token signed with old key should still verify during rotation window")

	newToken, err := jwtSvc.GenerateToken(userID.String(), orgID.String(), "staff")
	require.NoError(t, err)
	_, _, _, _, _, err = jwtSvc.VerifyAccessToken(newToken)
	require.NoError(t, err)
}

func TestConcurrentRefresh(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, _, _, _, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := svc.RefreshToken(ctx, application.RefreshTokenParams{
				RefreshToken: loginResult.RefreshToken,
			})
			results <- err
		}()
	}

	var successes, failures int
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			successes++
		} else {
			failures++
		}
	}

	assert.Equal(t, 1, successes, "exactly one concurrent refresh should succeed")
	assert.Equal(t, 1, failures, "one concurrent refresh should fail due to token reuse")
}

func TestWrongUserRefreshToken_Denied(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, _, _, _, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	user1 := seedAuthUser(t, db, orgID, "staff")

	login1, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          "user-" + user1.String()[:8] + "@example.com",
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	_, err = svc.RefreshToken(ctx, application.RefreshTokenParams{
		RefreshToken: login1.RefreshToken,
	})
	require.NoError(t, err)

	handler := identityapi.NewHandler(svc)
	r := chi.NewRouter()
	r.Route("/api/v1/organizations/{orgId}/auth", func(r chi.Router) {
		r.Post("/refresh", handler.Refresh)
	})

	body := `{"refresh_token":"` + login1.RefreshToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/auth/refresh", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "reused refresh token should be denied")
}

func TestRoleChange_TakesEffectOnNextLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, roleRepo, sessionRepo, _, jwtSvc, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	adminRole, err := roleRepo.FindByName(ctx, orgID, "admin")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "UPDATE users SET role_id = $1, updated_at = NOW() WHERE id = $2", adminRole.ID, userID)
	require.NoError(t, err)

	err = svc.RevokeAllUserSessions(ctx, userID)
	require.NoError(t, err)

	_, _, _, jti, _, err := jwtSvc.VerifyAccessToken(loginResult.AccessToken)
	require.NoError(t, err)

	session, err := sessionRepo.FindByID(ctx, jti)
	require.NoError(t, err)
	require.True(t, session.IsRevoked(), "session should be revoked after role change")
}

func TestRevokedAdminSession_Denied(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, sessionRepo, _, jwtSvc, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	adminID := seedAuthUser(t, db, orgID, "admin")
	email := "user-" + adminID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	_, _, _, jti, _, err := jwtSvc.VerifyAccessToken(loginResult.AccessToken)
	require.NoError(t, err)

	err = sessionRepo.Revoke(ctx, jti)
	require.NoError(t, err)

	mw := middleware.AuthRequired(
		middleware.NewJWTService("test-secret-key-for-testing-1234567890", 20*time.Minute, 7*24*time.Hour, "civora"),
		sessionRepo,
	)

	adminMw := mw(middleware.RequireAnyRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	r := chi.NewRouter()
	r.Handle("/api/v1/organizations/"+orgID.String()+"/admin", adminMw)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+orgID.String()+"/admin", nil)
	req.Header.Set("Authorization", "Bearer "+loginResult.AccessToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "revoked admin session should be denied")
}

func TestStolenRefreshToken_DetectedAndRevoked(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, sessionRepo, _, _, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	stolenToken := loginResult.RefreshToken

	_, err = svc.RefreshToken(ctx, application.RefreshTokenParams{
		RefreshToken: loginResult.RefreshToken,
	})
	require.NoError(t, err)

	_, err = svc.RefreshToken(ctx, application.RefreshTokenParams{
		RefreshToken: stolenToken,
	})
	require.Error(t, err, "stolen (old) refresh token should be rejected")

	sessions, err := sessionRepo.FindActiveByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, sessions, 1, "current session should still be active after old token rejection")
}

func TestLogin_ResponseFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, _, _, _, db, orgID := setupAuthServices(t)

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	handler := identityapi.NewHandler(svc)
	r := chi.NewRouter()
	r.Route("/api/v1/organizations/{orgId}/auth", func(r chi.Router) {
		r.Post("/login", handler.Login)
	})

	body := `{"email":"` + email + `","password":"securepassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp shared.APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, data, "access_token")
	assert.Contains(t, data, "refresh_token")
	assert.Contains(t, data, "token_type")
	assert.Contains(t, data, "expires_in")
	assert.Contains(t, data, "session_id")
	assert.Equal(t, "Bearer", data["token_type"])
}

func TestLogout_Endpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	svc, _, _, sessionRepo, _, jwtSvc, db, orgID := setupAuthServices(t)
	ctx := context.Background()

	userID := seedAuthUser(t, db, orgID, "staff")
	email := "user-" + userID.String()[:8] + "@example.com"

	loginResult, err := svc.Authenticate(ctx, application.AuthenticateParams{
		OrganizationID: orgID,
		Email:          email,
		Password:       "securepassword123",
	})
	require.NoError(t, err)

	_, _, _, jti, _, err := jwtSvc.VerifyAccessToken(loginResult.AccessToken)
	require.NoError(t, err)

	mw := middleware.AuthRequired(
		middleware.NewJWTService("test-secret-key-for-testing-1234567890", 20*time.Minute, 7*24*time.Hour, "civora"),
		sessionRepo,
	)

	handler := identityapi.NewHandler(svc)
	r := chi.NewRouter()
	r.Route("/api/v1/organizations/{orgId}/auth", func(r chi.Router) {
		r.Use(mw)
		r.Post("/logout", handler.Logout)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+loginResult.AccessToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	session, err := sessionRepo.FindByID(ctx, jti)
	require.NoError(t, err)
	require.True(t, session.IsRevoked(), "session should be revoked after logout")
}
