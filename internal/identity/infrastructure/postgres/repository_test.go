package postgres

import (
	"context"
	"testing"

	"github.com/alrazihi/civora/internal/identity/domain"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_SaveAndFindByEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresUserRepository(db)
	roleRepo := NewPostgresRoleRepository(db)

	orgID := helpers.SeedOrg(db)

	role := domain.NewRole(orgID, "admin", "Administrator role", []string{"*"})
	err := roleRepo.Save(context.Background(), role)
	require.NoError(t, err)

	hasher := domain.NewBCryptHasher(4)
	hash, err := hasher.Hash("securepassword123")
	require.NoError(t, err)

	u := domain.NewUser(orgID, "test@example.com", "Test User", &role.ID)
	u.PasswordHash = &hash
	err = repo.Save(context.Background(), u)
	require.NoError(t, err)

	found, err := repo.FindByEmail(context.Background(), orgID, "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, u.ID, found.ID)
	assert.Equal(t, "test@example.com", found.Email)
	assert.Equal(t, "Test User", found.Name)
	assert.Equal(t, &role.ID, found.RoleID)
	require.NotNil(t, found.PasswordHash)
}

func TestUserRepository_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresUserRepository(db)

	org1 := helpers.SeedOrg(db)
	org2 := helpers.SeedOrg(db)

	u := domain.NewUser(org1, "user@example.com", "Test User", nil)
	err := repo.Save(context.Background(), u)
	require.NoError(t, err)

	_, err = repo.FindByEmail(context.Background(), org2, "user@example.com")
	require.Error(t, err, "should not find user from different organization")
}

func TestRoleRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresRoleRepository(db)

	orgID := helpers.SeedOrg(db)

	role := domain.NewRole(orgID, "admin", "Administrator", []string{"cases:*", "users:*"})
	err := repo.Save(context.Background(), role)
	require.NoError(t, err)

	found, err := repo.FindByName(context.Background(), orgID, "admin")
	require.NoError(t, err)
	assert.Equal(t, "admin", found.Name)
	assert.Contains(t, found.Permissions, "cases:*")

	foundByID, err := repo.FindByID(context.Background(), orgID, role.ID)
	require.NoError(t, err)
	assert.Equal(t, role.ID, foundByID.ID)

	roles, err := repo.FindByOrganization(context.Background(), orgID)
	require.NoError(t, err)
	assert.Len(t, roles, 1)
}

func TestUserRepository_ForeignKeyConstraint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresUserRepository(db)

	fakeOrgID := uuid.New()
	u := domain.NewUser(fakeOrgID, "user@example.com", "Test User", nil)
	err := repo.Save(context.Background(), u)
	require.Error(t, err, "should fail due to foreign key constraint")
}
