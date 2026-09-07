package postgres

import (
	"context"
	"testing"

	"github.com/alrazihi/civora/internal/cases/domain"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaseRepository_SaveAndFind(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresCaseRepository(db)

	orgID := helpers.SeedOrg(db)
	userID := helpers.SeedUser(db, orgID)

	c, err := domain.NewCase(orgID, userID, "Emergency Food Request", "Family needs emergency food assistance")
	require.NoError(t, err)
	err = repo.Save(context.Background(), c)
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), orgID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, c.ID, found.ID)
	assert.Equal(t, c.CaseNumber, found.CaseNumber)
	assert.Equal(t, "Emergency Food Request", found.Title)
	assert.Equal(t, domain.CaseStatusCreated, found.Status)
}

func TestCaseRepository_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresCaseRepository(db)

	org1 := helpers.SeedOrg(db)
	user1 := helpers.SeedUser(db, org1)
	org2 := helpers.SeedOrg(db)

	c, err := domain.NewCase(org1, user1, "Case in Org 1", "Description")
	require.NoError(t, err)
	err = repo.Save(context.Background(), c)
	require.NoError(t, err)

	_, err = repo.FindByID(context.Background(), org2, c.ID)
	require.Error(t, err, "should not find case from different organization")
}

func TestCaseRepository_UpdateStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresCaseRepository(db)

	orgID := helpers.SeedOrg(db)
	userID := helpers.SeedUser(db, orgID)

	c, err := domain.NewCase(orgID, userID, "Test Case", "Description")
	require.NoError(t, err)
	err = repo.Save(context.Background(), c)
	require.NoError(t, err)

	err = repo.UpdateStatus(context.Background(), orgID, c.ID, domain.CaseStatusOpen)
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), orgID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.CaseStatusOpen, found.Status)
}

func TestCaseRepository_FindByOrganization(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresCaseRepository(db)

	orgID := helpers.SeedOrg(db)
	userID := helpers.SeedUser(db, orgID)

	for i := 0; i < 3; i++ {
		c, err := domain.NewCase(orgID, userID, "Case", "Description")
		require.NoError(t, err)
		err = repo.Save(context.Background(), c)
		require.NoError(t, err)
	}

	cases, err := repo.FindByOrganization(context.Background(), orgID, 20, 0)
	require.NoError(t, err)
	assert.Len(t, cases, 3)
}

func TestCaseRepository_Assign(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresCaseRepository(db)

	orgID := helpers.SeedOrg(db)
	creatorID := helpers.SeedUser(db, orgID)
	assigneeID := helpers.SeedUser(db, orgID)

	c, err := domain.NewCase(orgID, creatorID, "Test Case", "Description")
	require.NoError(t, err)
	err = repo.Save(context.Background(), c)
	require.NoError(t, err)

	err = repo.Assign(context.Background(), orgID, c.ID, assigneeID)
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), orgID, c.ID)
	require.NoError(t, err)
	require.NotNil(t, found.AssignedToID)
	assert.Equal(t, assigneeID, *found.AssignedToID)
}

func TestCaseRepository_ForeignKeyConstraint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresCaseRepository(db)

	fakeOrgID := uuid.New()
	fakeUserID := uuid.New()

	c, err := domain.NewCase(fakeOrgID, fakeUserID, "Test Case", "Description")
	require.NoError(t, err)
	err = repo.Save(context.Background(), c)
	require.Error(t, err, "should fail due to foreign key constraint")
}
