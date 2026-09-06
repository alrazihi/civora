package postgres

import (
	"context"
	"testing"

	"github.com/alrazihi/civora/internal/organizations/domain"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationRepository_SaveAndFind(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresOrganizationRepository(db)

	org := domain.NewOrganization("Test Org", "A test organization", "test-org")
	org.ID = uuid.New()
	err := repo.Save(context.Background(), org)
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), org.ID)
	require.NoError(t, err)
	assert.Equal(t, org.ID, found.ID)
	assert.Equal(t, "Test Org", found.Name)
	assert.Equal(t, "test-org", found.Slug)
}

func TestOrganizationRepository_FindBySlug(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresOrganizationRepository(db)

	org := domain.NewOrganization("Test Org", "A test organization", "test-org")
	org.ID = uuid.New()
	err := repo.Save(context.Background(), org)
	require.NoError(t, err)

	found, err := repo.FindBySlug(context.Background(), "test-org")
	require.NoError(t, err)
	assert.Equal(t, org.ID, found.ID)
}

func TestOrganizationRepository_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresOrganizationRepository(db)

	_, err := repo.FindBySlug(context.Background(), "nonexistent")
	require.Error(t, err)
}
