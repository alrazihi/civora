package application

import (
	"context"
	"testing"

	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email    string
		expected error
	}{
		{"user@example.com", nil},
		{"test.user+tag@example.org", nil},
		{"invalid", ErrInvalidEmail},
		{"noatdomain.com", ErrInvalidEmail},
		{"noatsign.com", ErrInvalidEmail},
		{"@nodomain.com", ErrInvalidEmail},
		{"user@.com", ErrInvalidEmail},
		{"user@domain", ErrInvalidEmail},
		{"user@domain.co", nil},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			err := validateEmail(tt.email)
			if tt.expected == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.expected)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		expected error
	}{
		{"valid", "password123", nil},
		{"valid complex", "SecureP@ss1", nil},
		{"too short", "Pass1", ErrWeakPassword},
		{"no numbers", "password", ErrWeakPassword},
		{"no letters", "12345678", ErrWeakPassword},
		{"empty", "", ErrWeakPassword},
		{"only letters short", "abc1", ErrWeakPassword},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.password)
			if tt.expected == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.expected)
			}
		})
	}
}

type mockRoleRepo struct {
	roles []*domain.Role
}

func (m *mockRoleRepo) Save(ctx context.Context, role *domain.Role) error {
	m.roles = append(m.roles, role)
	return nil
}
func (m *mockRoleRepo) FindByID(ctx context.Context, orgID, roleID uuid.UUID) (*domain.Role, error) {
	return nil, nil
}
func (m *mockRoleRepo) FindByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.Role, error) {
	return nil, nil
}
func (m *mockRoleRepo) FindByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.Role, error) {
	return nil, nil
}

func TestDefaultRoleCreator_CreatesAdminAndStaff(t *testing.T) {
	creator := &mockRoleRepo{}
	roleCreator := domain.NewDefaultRoleCreator(creator)

	orgID := uuid.New()
	err := roleCreator.CreateDefaultRoles(context.Background(), orgID)
	require.NoError(t, err)

	assert.Len(t, creator.roles, 2)
	assert.Equal(t, "admin", creator.roles[0].Name)
	assert.Equal(t, "staff", creator.roles[1].Name)
	assert.Equal(t, orgID, creator.roles[0].OrganizationID)
	assert.Contains(t, creator.roles[0].Permissions, "*")
	assert.Contains(t, creator.roles[1].Permissions, "cases:*")
}

func TestCreateUser_WeakPassword(t *testing.T) {
	svc := &IdentityService{
		userRepo: &mockUserRepo{},
		roleRepo: &mockRoleRepo{},
		hasher:   &mockHasher{},
	}

	_, err := svc.CreateUser(context.Background(), CreateUserParams{
		OrganizationID: uuid.New(),
		Email:          "user@example.com",
		Name:           "Test User",
		Password:       "weak",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrWeakPassword)
}

func TestCreateUser_InvalidEmail(t *testing.T) {
	svc := &IdentityService{
		userRepo: &mockUserRepo{},
		roleRepo: &mockRoleRepo{},
		hasher:   &mockHasher{},
	}

	_, err := svc.CreateUser(context.Background(), CreateUserParams{
		OrganizationID: uuid.New(),
		Email:          "not-an-email",
		Name:           "Test User",
		Password:       "password123",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidEmail)
}

type mockUserRepo struct {
	users []*domain.User
}

func (m *mockUserRepo) Save(ctx context.Context, u *domain.User) error {
	m.users = append(m.users, u)
	return nil
}
func (m *mockUserRepo) FindByEmail(ctx context.Context, orgID uuid.UUID, email string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) FindByID(ctx context.Context, orgID, userID uuid.UUID) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) FindByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error) {
	return nil, nil
}

type mockHasher struct{}

func (m *mockHasher) Hash(password string) (string, error) {
	return "hashed", nil
}
func (m *mockHasher) Verify(password, hash string) (bool, error) {
	return true, nil
}
