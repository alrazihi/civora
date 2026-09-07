package application

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/alrazihi/civora/internal/config"
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
		{"valid", "password1234", nil},
		{"valid complex", "SecureP@ssw0rd", nil},
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
	roles        []*domain.Role
	findByNameFn func(ctx context.Context, orgID uuid.UUID, name string) (*domain.Role, error)
}

func (m *mockRoleRepo) DB() *sql.DB {
	return nil
}

func (m *mockRoleRepo) Save(ctx context.Context, role *domain.Role) error {
	m.roles = append(m.roles, role)
	return nil
}

func (m *mockRoleRepo) SaveTx(ctx context.Context, tx *sql.Tx, role *domain.Role) error {
	m.roles = append(m.roles, role)
	return nil
}

func (m *mockRoleRepo) FindByID(ctx context.Context, orgID, roleID uuid.UUID) (*domain.Role, error) {
	return nil, nil
}

func (m *mockRoleRepo) FindByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.Role, error) {
	if m.findByNameFn != nil {
		return m.findByNameFn(ctx, orgID, name)
	}
	for _, r := range m.roles {
		if r.Name == name {
			return r, nil
		}
	}
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
		Password:       "password1234",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidEmail)
}

type mockUserRepo struct {
	users []*domain.User
}

func (m *mockUserRepo) DB() *sql.DB {
	return nil
}

func (m *mockUserRepo) Save(ctx context.Context, u *domain.User) error {
	m.users = append(m.users, u)
	return nil
}

func (m *mockUserRepo) SaveTx(ctx context.Context, tx *sql.Tx, u *domain.User) error {
	m.users = append(m.users, u)
	return nil
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, orgID uuid.UUID, email string) (*domain.User, error) {
	return nil, nil
}

func (m *mockUserRepo) FindByID(ctx context.Context, orgID, userID uuid.UUID) (*domain.User, error) {
	return nil, nil
}

func (m *mockUserRepo) FindByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.User, error) {
	return nil, nil
}

func (m *mockUserRepo) CountByOrganization(ctx context.Context, orgID uuid.UUID) (int, error) {
	return len(m.users), nil
}

func (m *mockUserRepo) CountByOrganizationTx(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) (int, error) {
	return len(m.users), nil
}

type mockHasher struct{}

func (m *mockHasher) Hash(password string) (string, error) {
	return "hashed", nil
}
func (m *mockHasher) Verify(password, hash string) (bool, error) {
	return true, nil
}

func TestCreateUser_AutoAssignsAdminRoleForFirstUser(t *testing.T) {
	roleRepo := &mockRoleRepo{}
	roleRepo.roles = append(roleRepo.roles,
		domain.NewRole(uuid.New(), "admin", "Admin role", []string{"*"}),
		domain.NewRole(uuid.New(), "staff", "Staff role", []string{"cases:*"}),
	)

	svc := &IdentityService{
		userRepo: &mockUserRepo{},
		roleRepo: roleRepo,
		hasher:   &mockHasher{},
	}

	user, err := svc.CreateUser(context.Background(), CreateUserParams{
		OrganizationID: uuid.New(),
		Email:          "first@example.com",
		Name:           "First User",
		Password:       "password1234",
	})
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.NotNil(t, user.RoleID, "first user should get a role")
}

func TestCreateUser_RoleFieldsRemovedFromParams(t *testing.T) {
	paramsType := reflect.TypeOf(CreateUserParams{})
	fieldNames := make([]string, 0, paramsType.NumField())
	for i := 0; i < paramsType.NumField(); i++ {
		fieldNames = append(fieldNames, paramsType.Field(i).Name)
	}
	assert.NotContains(t, fieldNames, "RoleName", "RoleName should be removed from CreateUserParams for RBAC safety")
}

func TestOIDCConfigFieldsRemoved(t *testing.T) {
	authType := reflect.TypeOf(config.AuthConfig{})
	_, exists := authType.FieldByName("OIDCIssuer")
	assert.False(t, exists, "OIDCIssuer field must be removed from AuthConfig")

	_, hasClientID := authType.FieldByName("OIDCClientID")
	assert.False(t, hasClientID, "OIDCClientID field must be removed from AuthConfig")

	_, hasRedirectURL := authType.FieldByName("OIDCRedirectURL")
	assert.False(t, hasRedirectURL, "OIDCRedirectURL field must be removed from AuthConfig")
}

func TestPasswordPolicy_RejectsWeakPasswords(t *testing.T) {
	weakPasswords := []string{"short", "12345678", "abcdefgh", ""}
	for _, pw := range weakPasswords {
		err := validatePassword(pw)
		assert.ErrorIs(t, err, ErrWeakPassword, "password %q should be rejected", pw)
	}
}

func TestPasswordPolicy_Enforces12CharMinimum(t *testing.T) {
	t.Run("11 chars rejected", func(t *testing.T) {
		err := validatePassword("password1")
		assert.ErrorIs(t, err, ErrWeakPassword)
	})
	t.Run("12 chars accepted", func(t *testing.T) {
		err := validatePassword("password1234")
		assert.NoError(t, err)
	})
	t.Run("12 chars no number rejected", func(t *testing.T) {
		err := validatePassword("twelvechars")
		assert.ErrorIs(t, err, ErrWeakPassword)
	})
}

func TestCreateUser_RoleLookupErrorIsPropagated(t *testing.T) {
	orgID := uuid.New()
	adminRoleID := uuid.New()

	roleRepo := &mockRoleRepo{}
	roleRepo.roles = append(roleRepo.roles,
		domain.NewRole(adminRoleID, "admin", "Admin role", []string{"*"}),
	)

	fr := &failingRoleRepo{mockRoleRepo: roleRepo}

	svc := &IdentityService{
		userRepo: &mockUserRepo{users: []*domain.User{
			{ID: uuid.New(), OrganizationID: orgID, Email: "existing@example.com"},
		}},
		roleRepo: &roleRepoWithError{
			base:        fr,
			returnError: true,
			errorOnName: "staff",
		},
		hasher: &mockHasher{},
	}

	_, err := svc.CreateUser(context.Background(), CreateUserParams{
		OrganizationID: orgID,
		Email:          "new@example.com",
		Name:           "New User",
		Password:       "password1234",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find staff role")
}

func TestCreateUser_AdminRoleLookupErrorIsPropagated(t *testing.T) {
	orgID := uuid.New()

	roleRepo := &mockRoleRepo{}

	svc := &IdentityService{
		userRepo: &mockUserRepo{},
		roleRepo: &roleRepoWithError{
			base:        &failingRoleRepo{mockRoleRepo: roleRepo},
			returnError: true,
		},
		hasher: &mockHasher{},
	}

	_, err := svc.CreateUser(context.Background(), CreateUserParams{
		OrganizationID: orgID,
		Email:          "first@example.com",
		Name:           "First User",
		Password:       "password1234",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find admin role")
}

type failingRoleRepo struct {
	*mockRoleRepo
}

type roleRepoWithError struct {
	base        *failingRoleRepo
	returnError bool
	errorOnName string
}

func (r *roleRepoWithError) DB() *sql.DB {
	return nil
}

func (r *roleRepoWithError) Save(ctx context.Context, role *domain.Role) error {
	return r.base.Save(ctx, role)
}

func (r *roleRepoWithError) SaveTx(ctx context.Context, tx *sql.Tx, role *domain.Role) error {
	return r.base.SaveTx(ctx, tx, role)
}

func (r *roleRepoWithError) FindByID(ctx context.Context, orgID, roleID uuid.UUID) (*domain.Role, error) {
	return r.base.FindByID(ctx, orgID, roleID)
}

func (r *roleRepoWithError) FindByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.Role, error) {
	if r.returnError && (r.errorOnName == "" || r.errorOnName == name) {
		return nil, assert.AnError
	}
	return r.base.FindByName(ctx, orgID, name)
}

func (r *roleRepoWithError) FindByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.Role, error) {
	return r.base.FindByOrganization(ctx, orgID)
}
