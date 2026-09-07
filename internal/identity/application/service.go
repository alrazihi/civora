package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	auditdomain "github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/database"
	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrInvalidInput       = errors.New("invalid input")
	ErrWeakPassword       = errors.New("password must be at least 8 characters and contain at least one letter and one number")
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

const minPasswordLength = 8

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
}

type txEventRecorder interface {
	RecordEventInTx(ctx context.Context, tx *sql.Tx, params auditdomain.RecordEventParams) error
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
	if err != nil {
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
			if err == nil {
				roleID = &role.ID
			}
		} else {
			role, err := s.roleRepo.FindByName(ctx, params.OrganizationID, domain.RoleStaff)
			if err == nil {
				roleID = &role.ID
			}
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
		if err := s.auditor.RecordEvent(ctx, auditdomain.RecordEventParams{
			OrganizationID: params.OrganizationID,
			ActorID:        &user.ID,
			Action:         "auth.success",
			Resource:       "user",
			ResourceID:     strPtr(user.ID.String()),
			Outcome:        "success",
		}); err != nil {
			return nil, fmt.Errorf("failed to record audit event: %w", err)
		}
	}

	return &AuthenticateResult{
		User:  user,
		Token: token,
	}, nil
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
