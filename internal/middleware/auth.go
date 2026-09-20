package middleware

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/alrazihi/civora/internal/shared"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TenantKey    contextKey = "tenant_id"
	UserIDKey    contextKey = "user_id"
	RoleKey      contextKey = "user_role"
	ExpiryKey    contextKey = "token_exp"
	SessionIDKey contextKey = "session_id"
)

type KeySet struct {
	mu        sync.RWMutex
	keys      map[string][]byte
	activeKid string
}

func NewKeySet(initialSecret []byte) *KeySet {
	kid := "primary"
	return &KeySet{
		keys:      map[string][]byte{kid: initialSecret},
		activeKid: kid,
	}
}

func (k *KeySet) AddKey(kid string, secret []byte) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.keys[kid] = secret
}

func (k *KeySet) RemoveKey(kid string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.keys, kid)
	if k.activeKid == kid {
		for k2 := range k.keys {
			k.activeKid = k2
			break
		}
	}
}

func (k *KeySet) RotateActive(kid string, secret []byte) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.keys[kid] = secret
	k.activeKid = kid
}

func (k *KeySet) SigningKey() (string, []byte) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.activeKid, k.keys[k.activeKid]
}

func (k *KeySet) VerifyKey(kid string) ([]byte, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	secret, ok := k.keys[kid]
	return secret, ok
}

type JWTService struct {
	keySet         *KeySet
	accessExpiry   time.Duration
	refreshExpiry  time.Duration
	issuer         string
}

func NewJWTService(secret string, accessExpiry, refreshExpiry time.Duration, issuer string) *JWTService {
	return &JWTService{
		keySet:         NewKeySet([]byte(secret)),
		accessExpiry:   accessExpiry,
		refreshExpiry:  refreshExpiry,
		issuer:         issuer,
	}
}

func (s *JWTService) KeySet() *KeySet {
	return s.keySet
}

var _ domain.TokenService = (*JWTService)(nil)

func (s *JWTService) GenerateToken(userID, organizationID, role string) (string, error) {
	return s.GenerateAccessToken(userID, organizationID, role, "")
}

func (s *JWTService) GenerateAccessToken(userID, organizationID, role, jti string) (string, error) {
	now := time.Now()
	expiresAt := now.Add(s.accessExpiry)
	kid, secret := s.keySet.SigningKey()

	if jti == "" {
		jti = uuid.NewString()
	}

	claims := jwt.MapClaims{
		"sub":             userID,
		"organization_id": organizationID,
		"role":            role,
		"iss":             s.issuer,
		"iat":             now.Unix(),
		"exp":             expiresAt.Unix(),
		"jti":             jti,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = kid
	return token.SignedString(secret)
}

func (s *JWTService) GenerateTokenPair(userID, organizationID, role, sessionID string) (string, string, error) {
	accessToken, err := s.GenerateAccessToken(userID, organizationID, role, sessionID)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

func GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func (s *JWTService) VerifyToken(tokenString string) (userID, organizationID, role, jti string, exp time.Time, err error) {
	return s.VerifyAccessToken(tokenString)
}

func (s *JWTService) VerifyAccessToken(tokenString string) (userID, organizationID, role, jti string, exp time.Time, err error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("missing kid in token header")
		}
		secret, ok := s.keySet.VerifyKey(kid)
		if !ok {
			return nil, errors.New("unknown signing key")
		}
		return secret, nil
	})
	if err != nil {
		return "", "", "", "", time.Time{}, err
	}

	if !token.Valid {
		return "", "", "", "", time.Time{}, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", "", "", time.Time{}, errors.New("invalid claims")
	}

	iss, _ := claims["iss"].(string)
	if s.issuer != "" && iss != s.issuer {
		return "", "", "", "", time.Time{}, errors.New("token issuer mismatch")
	}

	userID, _ = claims["sub"].(string)
	organizationID, _ = claims["organization_id"].(string)
	role, _ = claims["role"].(string)
	jti, _ = claims["jti"].(string)

	if expNum, ok := claims["exp"].(float64); ok {
		exp = time.Unix(int64(expNum), 0)
	}

	return userID, organizationID, role, jti, exp, nil
}

func (s *JWTService) VerifyRefreshToken(tokenString string) (string, error) {
	hash := HashRefreshToken(tokenString)
	if len(tokenString) < 32 {
		return "", errors.New("invalid refresh token")
	}
	return hash, nil
}

func (s *JWTService) RevokeSession(sessionID string) error {
	return nil
}

func (s *JWTService) RevokeAllUserSessions(userID string) error {
	return nil
}

func (s *JWTService) AccessExpiry() time.Duration {
	return s.accessExpiry
}

func (s *JWTService) RefreshExpiry() time.Duration {
	return s.refreshExpiry
}

func AuthRequired(svc *JWTService, sessionRepo domain.SessionRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
				writeUnauthorized(w)
				return
			}

			tokenString := authHeader[7:]
			userID, orgID, role, jti, exp, err := svc.VerifyAccessToken(tokenString)
			if err != nil {
				writeUnauthorized(w)
				return
			}

			if time.Now().After(exp) {
				writeUnauthorized(w)
				return
			}

			if sessionRepo != nil && jti != "" {
				session, err := sessionRepo.FindByID(r.Context(), jti)
				if err != nil || session.IsRevoked() || session.IsExpired() || session.UserID.String() != userID || session.OrganizationID.String() != orgID {
					writeUnauthorized(w)
					return
				}
				_ = sessionRepo.MarkUsed(r.Context(), jti)
				ctx := context.WithValue(r.Context(), SessionIDKey, jti)
				r = r.WithContext(ctx)
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, TenantKey, orgID)
			ctx = context.WithValue(ctx, RoleKey, role)
			ctx = context.WithValue(ctx, ExpiryKey, exp)
			ctx = context.WithValue(ctx, "jti", jti)
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
	return getStringValue(r.Context().Value(TenantKey))
}

func GetUserID(r *http.Request) string {
	return getStringValue(r.Context().Value(UserIDKey))
}

func GetUserRole(r *http.Request) string {
	return getStringValue(r.Context().Value(RoleKey))
}

func GetSessionID(r *http.Request) string {
	return getStringValue(r.Context().Value(SessionIDKey))
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
	body := []byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"Authentication required"}}`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	if _, err := w.Write(body); err != nil {
		_ = err
	}
}
