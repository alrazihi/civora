package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrInvalidInput       = errors.New("invalid input")
)

type IdentityService struct {
	userRepo domain.UserRepository
	roleRepo domain.RoleRepository
	hasher   domain.PasswordHasher
	tokenSvc domain.TokenService
	auditor  auditdomain.EventRecorder
}

func NewIdentityService(
	userRepo domain.UserRepository,
	roleRepo domain.RoleRepository,
	hasher domain.PasswordHasher,
	tokenSvc domain.TokenService,
	auditor auditdomain.EventRecorder,
) *IdentityService {
	return &IdentityService{
		userRepo: userRepo,
		roleRepo: roleRepo,
		hasher:   hasher,
		tokenSvc: tokenSvc,
		auditor:  auditor,
	}
}

type CreateUserParams struct {
	OrganizationID uuid.UUID
	Email          string
	Name           string
	Password       string
	RoleName       string
}

func (s *IdentityService) CreateUser(ctx context.Context, params CreateUserParams) (*domain.User, error) {
	if err := validateEmail(params.Email); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Name) == "" {
		return nil, ErrInvalidInput
	}
	if strings.TrimSpace(params.Password) == "" {
		return nil, ErrInvalidInput
	}

	existing, _ := s.userRepo.FindByEmail(ctx, params.OrganizationID, params.Email)
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	var roleID *uuid.UUID
	if params.RoleName != "" {
		role, err := s.roleRepo.FindByName(ctx, params.OrganizationID, params.RoleName)
		if err == nil {
			roleID = &role.ID
		}
	}

	user := domain.NewUser(params.OrganizationID, params.Email, params.Name, roleID)

	hash, err := s.hasher.Hash(params.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	user.PasswordHash = &hash

	if err := s.userRepo.Save(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	if s.auditor != nil {
		_ = s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: user.OrganizationID,
			ActorID:        &user.ID,
			Action:         "user.created",
			Resource:       "user",
			ResourceID:     strPtr(user.ID.String()),
			Outcome:        "success",
			Metadata:       map[string]interface{}{"email": user.Email},
		})
	}

	return user, nil
}

type AuthenticateParams struct {
	OrganizationID uuid.UUID
	Email          string
	Password       string
}

type AuthenticateResult struct {
	User  *domain.User
	Token string
}

func (s *IdentityService) Authenticate(ctx context.Context, params AuthenticateParams) (*AuthenticateResult, error) {
	if err := validateEmail(params.Email); err != nil {
		return nil, err
	}

	user, err := s.userRepo.FindByEmail(ctx, params.OrganizationID, params.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if user.PasswordHash == nil || *user.PasswordHash == "" {
		return nil, ErrInvalidCredentials
	}

	valid, err := s.hasher.Verify(params.Password, *user.PasswordHash)
	if err != nil || !valid {
		if s.auditor != nil {
			_ = s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
				OrganizationID: params.OrganizationID,
				Action:         "auth.failed",
				Resource:       "user",
				ResourceID:     strPtr(params.Email),
				Outcome:        "failure",
				Metadata:       map[string]interface{}{"reason": "invalid_credentials"},
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

	token, err := s.tokenSvc.GenerateToken(user.ID.String(), params.OrganizationID.String(), role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	if s.auditor != nil {
		_ = s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: params.OrganizationID,
			ActorID:        &user.ID,
			Action:         "auth.success",
			Resource:       "user",
			ResourceID:     strPtr(user.ID.String()),
			Outcome:        "success",
		})
	}

	return &AuthenticateResult{
		User:  user,
		Token: token,
	}, nil
}

func validateEmail(email string) error {
	email = strings.TrimSpace(email)
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return ErrInvalidEmail
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

func (s *IdentityService) ListUsers(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error) {
	return s.userRepo.FindByOrganization(ctx, orgID)
}

func strPtr(s string) *string {
	return &s
}
