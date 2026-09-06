package auth

import (
	"time"

	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTTokenService struct {
	secret []byte
	expiry time.Duration
	issuer string
}

func NewJWTTokenService(secret string, expiry time.Duration, issuer string) *JWTTokenService {
	return &JWTTokenService{
		secret: []byte(secret),
		expiry: expiry,
		issuer: issuer,
	}
}

var _ domain.TokenService = (*JWTTokenService)(nil)

func (s *JWTTokenService) GenerateToken(userID, organizationID, role string) (string, error) {
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
