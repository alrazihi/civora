package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	"github.com/alrazihi/civora/internal/identity/domain"
	intmid "github.com/alrazihi/civora/internal/middleware"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrInvalidInput       = errors.New("invalid input")
	ErrWeakPassword       = errors.New("password must be at least 12 characters and contain at least one letter and one number")
	ErrSessionNotFound    = domain.ErrSessionNotFound
	ErrSessionRevoked     = errors.New("session revoked")
	ErrInvalidRefresh     = errors.New("invalid refresh token")
	ErrRefreshReuse       = errors.New("refresh token reuse detected")
)

const timingSafeDummyHash = "$2a$10$dummy.hash.for.timing.safety.only.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

const minPasswordLength = 12

type IdentityService struct {
	userRepo    domain.UserRepository
	roleRepo    domain.RoleRepository
	sessionRepo domain.SessionRepository
	hasher      domain.PasswordHasher
	tokenSvc    domain.TokenService
	auditor     auditdomain.EventRecorder
}

func NewIdentityService(
	userRepo domain.UserRepository,
	roleRepo domain.RoleRepository,
	sessionRepo domain.SessionRepository,
	hasher domain.PasswordHasher,
	tokenSvc domain.TokenService,
	auditor auditdomain.EventRecorder,
) *IdentityService {
	return &IdentityService{
		userRepo:    userRepo,
		roleRepo:    roleRepo,
		sessionRepo: sessionRepo,
		hasher:      hasher,
		tokenSvc:    tokenSvc,
		auditor:     auditor,
	}
}

type CreateUserParams struct {
	OrganizationID uuid.UUID
	Email          string
	Name           string
	Password       string
}

type txEventRecorder interface {
	RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error
}

type AuthenticateResult struct {
	User         *domain.User
	AccessToken  string
	RefreshToken string
	SessionID    string
	ExpiresIn    int64
}

func (s *IdentityService) CreateUser(ctx context.Context, params CreateUserParams) (*domain.User, error) {
	if err := validateEmail(params.Email); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Name) == "" {
		return nil, ErrInvalidInput
	}
	if err := validatePassword(params.Password); err != nil {
		return nil, err
	}

	existing, err := s.userRepo.FindByEmail(ctx, params.OrganizationID, params.Email)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	var user *domain.User
	err = database.InTransaction(ctx, s.userRepo.DB(), func(tx *sql.Tx) error {
		count, err := s.userRepo.CountByOrganizationTx(ctx, tx, params.OrganizationID)
		if err != nil {
			return fmt.Errorf("failed to count users: %w", err)
		}

		var roleID *uuid.UUID
		if count == 0 {
			role, err := s.roleRepo.FindByName(ctx, params.OrganizationID, domain.RoleAdmin)
			if err != nil {
				return fmt.Errorf("failed to find admin role: %w", err)
			}
			roleID = &role.ID
		} else {
			role, err := s.roleRepo.FindByName(ctx, params.OrganizationID, domain.RoleStaff)
			if err != nil {
				return fmt.Errorf("failed to find staff role: %w", err)
			}
			roleID = &role.ID
		}

		user = domain.NewUser(params.OrganizationID, params.Email, params.Name, roleID)

		hash, err := s.hasher.Hash(params.Password)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		user.PasswordHash = &hash

		if err := s.userRepo.SaveTx(ctx, tx, user); err != nil {
			return fmt.Errorf("failed to save user: %w", err)
		}

		if s.auditor != nil {
			if txRecorder, ok := s.auditor.(txEventRecorder); ok {
				if err := txRecorder.RecordEventInTx(ctx, tx, auditdomain.RecordEventParams{
					OrganizationID: user.OrganizationID,
					ActorID:        &user.ID,
					Action:         "user.created",
					Resource:       "user",
					ResourceID:     strPtr(user.ID.String()),
					Outcome:        "success",
					RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
				}); err != nil {
					return fmt.Errorf("failed to record audit event: %w", err)
				}
			} else {
				if err := s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
					OrganizationID: user.OrganizationID,
					ActorID:        &user.ID,
					Action:         "user.created",
					Resource:       "user",
					ResourceID:     strPtr(user.ID.String()),
					Outcome:        "success",
					RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
				}); err != nil {
					return fmt.Errorf("failed to record audit event: %w", err)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return user, nil
}

type AuthenticateParams struct {
	OrganizationID uuid.UUID
	Email          string
	Password       string
}

func (s *IdentityService) Authenticate(ctx context.Context, params AuthenticateParams) (*AuthenticateResult, error) {
	if err := validateEmail(params.Email); err != nil {
		return nil, err
	}

	user, err := s.userRepo.FindByEmail(ctx, params.OrganizationID, params.Email)
	if err != nil {
		s.hasher.Verify(params.Password, timingSafeDummyHash)
		return nil, ErrInvalidCredentials
	}
	if user.PasswordHash == nil || *user.PasswordHash == "" {
		s.hasher.Verify(params.Password, timingSafeDummyHash)
		return nil, ErrInvalidCredentials
	}

	valid, err := s.hasher.Verify(params.Password, *user.PasswordHash)
	if err != nil || !valid {
		if s.auditor != nil {
			_ = s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				Action:         "auth.failed",
				Resource:       "user",
				ResourceID:     strPtr(user.ID.String()),
				Outcome:        "failure",
				Metadata:       map[string]interface{}{"reason": "invalid_credentials"},
				RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
			})
		}
		return nil, ErrInvalidCredentials
	}

	role := "user"
	if user.RoleID != nil {
		r, err := s.roleRepo.FindByID(ctx, params.OrganizationID, *user.RoleID)
		if err == nil {
			role = r.Name
		}
	}

	now := time.Now().UTC()
	sessionID := uuid.New()
	refreshToken, err := intmid.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}
	refreshTokenHash := intmid.HashRefreshToken(refreshToken)

	session := &domain.Session{
		ID:               sessionID,
		UserID:           user.ID,
		OrganizationID:   params.OrganizationID,
		RefreshTokenHash: refreshTokenHash,
		TokenFamily:      uuid.New(),
		UserModifiedAt:   user.UpdatedAt,
		IssuedAt:         now,
		ExpiresAt:        now.Add(s.tokenSvc.RefreshExpiry()),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	accessToken, err := s.tokenSvc.GenerateAccessToken(user.ID.String(), params.OrganizationID.String(), role, session.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	_ = sessionID

	if s.auditor != nil {
		if err := s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: params.OrganizationID,
			ActorID:        &user.ID,
			Action:         "auth.success",
			Resource:       "user",
			ResourceID:     strPtr(user.ID.String()),
			Outcome:        "success",
			RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
		}); err != nil {
			return nil, fmt.Errorf("failed to record audit event: %w", err)
		}
	}

	return &AuthenticateResult{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		SessionID:    sessionID.String(),
		ExpiresIn:    int64(s.tokenSvc.AccessExpiry().Seconds()),
	}, nil
}

type RefreshTokenParams struct {
	RefreshToken string
}

type RefreshTokenResult struct {
	AccessToken  string
	RefreshToken string
	SessionID    string
	ExpiresIn    int64
}

func (s *IdentityService) RefreshToken(ctx context.Context, params RefreshTokenParams) (*RefreshTokenResult, error) {
	if params.RefreshToken == "" {
		return nil, ErrInvalidRefresh
	}

	hash := intmid.HashRefreshToken(params.RefreshToken)
	session, err := s.sessionRepo.FindByRefreshTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return nil, ErrInvalidRefresh
		}
		return nil, fmt.Errorf("failed to lookup session: %w", err)
	}

	if session.IsRevoked() {
		return nil, ErrSessionRevoked
	}
	if session.IsExpired() {
		return nil, ErrInvalidRefresh
	}

	user, err := s.userRepo.FindByID(ctx, session.OrganizationID, session.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	role := "user"
	if user.RoleID != nil {
		r, err := s.roleRepo.FindByID(ctx, session.OrganizationID, *user.RoleID)
		if err == nil {
			role = r.Name
		}
	}

	now := time.Now().UTC()
	newAccessToken, newRefreshToken, err := s.tokenSvc.GenerateTokenPair(
		user.ID.String(),
		session.OrganizationID.String(),
		role,
		session.ID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	newHash := intmid.HashRefreshToken(newRefreshToken)
	session.RefreshTokenHash = newHash
	session.TokenFamily = uuid.New()
	session.ExpiresAt = now.Add(s.tokenSvc.RefreshExpiry())
	session.UpdatedAt = now

	if err := s.sessionRepo.Update(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	if s.auditor != nil {
		_ = s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: session.OrganizationID,
			ActorID:        &user.ID,
			Action:         "auth.refresh",
			Resource:       "session",
			ResourceID:     strPtr(session.ID.String()),
			Outcome:        "success",
			RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
		})
	}

	return &RefreshTokenResult{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		SessionID:    session.ID.String(),
		ExpiresIn:    int64(s.tokenSvc.AccessExpiry().Seconds()),
	}, nil
}

type LogoutParams struct {
	SessionID string
}

func (s *IdentityService) Logout(ctx context.Context, params LogoutParams) error {
	if params.SessionID == "" {
		return ErrInvalidInput
	}

	session, err := s.sessionRepo.FindByID(ctx, params.SessionID)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return nil
		}
		return fmt.Errorf("failed to lookup session: %w", err)
	}

	if err := s.sessionRepo.Revoke(ctx, session.ID.String()); err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	if s.auditor != nil {
		_ = s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: session.OrganizationID,
			ActorID:        &session.UserID,
			Action:         "auth.logout",
			Resource:       "session",
			ResourceID:     strPtr(session.ID.String()),
			Outcome:        "success",
			RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
		})
	}

	return nil
}

func (s *IdentityService) RevokeAllUserSessions(ctx context.Context, userID, orgID uuid.UUID) error {
	if s.sessionRepo == nil {
		return nil
	}
	return s.sessionRepo.RevokeAllByUserIDForOrganization(ctx, userID, orgID)
}

type ChangePasswordParams struct {
	UserID      uuid.UUID
	OrgID       uuid.UUID
	OldPassword string
	NewPassword string
}

func (s *IdentityService) ChangePassword(ctx context.Context, params ChangePasswordParams) error {
	if len(params.NewPassword) < minPasswordLength {
		return ErrWeakPassword
	}
	hasLetter := false
	hasNumber := false
	for _, c := range params.NewPassword {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasLetter = true
		}
		if c >= '0' && c <= '9' {
			hasNumber = true
		}
	}
	if !hasLetter || !hasNumber {
		return ErrWeakPassword
	}

	user, err := s.userRepo.FindByID(ctx, params.OrgID, params.UserID)
	if err != nil {
		return ErrUserNotFound
	}
	if user.PasswordHash == nil {
		return ErrInvalidCredentials
	}

	valid, err := s.hasher.Verify(params.OldPassword, *user.PasswordHash)
	if err != nil || !valid {
		return ErrInvalidCredentials
	}

	newHash, err := s.hasher.Hash(params.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	now := time.Now().UTC()
	_, err = s.userRepo.DB().ExecContext(ctx,
		"UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3",
		newHash, now, user.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	if err := s.RevokeAllUserSessions(ctx, user.ID, user.OrganizationID); err != nil {
		return fmt.Errorf("failed to revoke sessions: %w", err)
	}

	if s.auditor != nil {
		_ = s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: user.OrganizationID,
			ActorID:        &user.ID,
			Action:         "auth.password_changed",
			Resource:       "user",
			ResourceID:     strPtr(user.ID.String()),
			Outcome:        "success",
			RequestID:      strPtr(intmid.RequestIDFromContext(ctx)),
		})
	}

	return nil
}

func (s *IdentityService) GetUser(ctx context.Context, orgID, userID uuid.UUID) (*domain.User, error) {
	user, err := s.userRepo.FindByID(ctx, orgID, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *IdentityService) ListUsers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.User, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.userRepo.CountByOrganization(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	users, err := s.userRepo.FindByOrganization(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	return users, total, nil
}

func validateEmail(email string) error {
	if !emailRegex.MatchString(email) {
		return ErrInvalidEmail
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLength {
		return ErrWeakPassword
	}
	hasLetter := false
	hasNumber := false
	for _, c := range password {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasLetter = true
		}
		if c >= '0' && c <= '9' {
			hasNumber = true
		}
	}
	if !hasLetter || !hasNumber {
		return ErrWeakPassword
	}
	return nil
}

func strPtr(s string) *string {
	return &s
}
