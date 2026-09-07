package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	tenantKey contextKey = "tenant_id"
	userIDKey contextKey = "user_id"
	roleKey   contextKey = "user_role"
	expiryKey contextKey = "token_exp"
)

type JWTService struct {
	secret []byte
	expiry time.Duration
	issuer string
}

func NewJWTService(secret string, expiry time.Duration, issuer string) *JWTService {
	return &JWTService{
		secret: []byte(secret),
		expiry: expiry,
		issuer: issuer,
	}
}

var _ domain.TokenService = (*JWTService)(nil)

func (s *JWTService) GenerateToken(userID, organizationID, role string) (string, error) {
	now := time.Now()
	expiresAt := now.Add(s.expiry)

	claims := jwt.MapClaims{
		"sub":             userID,
		"organization_id": organizationID,
		"role":            role,
		"iss":             s.issuer,
		"iat":             now.Unix(),
		"exp":             expiresAt.Unix(),
		"jti":             uuid.NewString(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *JWTService) VerifyToken(tokenString string) (userID, organizationID, role string, exp time.Time, err error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return "", "", "", time.Time{}, err
	}

	if !token.Valid {
		return "", "", "", time.Time{}, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", "", time.Time{}, errors.New("invalid claims")
	}

	userID, _ = claims["sub"].(string)
	organizationID, _ = claims["organization_id"].(string)
	role, _ = claims["role"].(string)

	if expNum, ok := claims["exp"].(float64); ok {
		exp = time.Unix(int64(expNum), 0)
	}

	return userID, organizationID, role, exp, nil
}

func AuthRequired(svc *JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
				writeUnauthorized(w)
				return
			}

			tokenString := authHeader[7:]
			userID, orgID, role, exp, err := svc.VerifyToken(tokenString)
			if err != nil {
				writeUnauthorized(w)
				return
			}

			if time.Now().After(exp.Add(-30 * time.Second)) {
				writeUnauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, tenantKey, orgID)
			ctx = context.WithValue(ctx, roleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireSameTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenOrgID := GetTenantID(r)
		if tokenOrgID == "" {
			writeUnauthorized(w)
			return
		}

		pathOrgID := chi.URLParam(r, "orgId")
		if pathOrgID == "" {
			shared.WriteError(w, http.StatusBadRequest, shared.CodeInvalidInput, "organization ID is required")
			return
		}

		tokenOrg, err1 := uuid.Parse(tokenOrgID)
		pathOrg, err2 := uuid.Parse(pathOrgID)
		if err1 != nil || err2 != nil || tokenOrg != pathOrg {
			shared.WriteError(w, http.StatusForbidden, "FORBIDDEN", "organization access denied")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := GetUserRole(r)
			if role == "" || !allowed[role] {
				shared.WriteError(w, http.StatusForbidden, "FORBIDDEN", "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := GetUserRole(r)
			if role == "" {
				shared.WriteError(w, http.StatusUnauthorized, shared.CodeUnauthorized, "authentication required")
				return
			}
			if !allowed[role] {
				shared.WriteError(w, http.StatusForbidden, "FORBIDDEN", "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func GetTenantID(r *http.Request) string {
	return getStringValue(r.Context().Value(tenantKey))
}

func GetUserID(r *http.Request) string {
	return getStringValue(r.Context().Value(userIDKey))
}

func GetUserRole(r *http.Request) string {
	return getStringValue(r.Context().Value(roleKey))
}

func getStringValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"Authentication required"}}`))
}
