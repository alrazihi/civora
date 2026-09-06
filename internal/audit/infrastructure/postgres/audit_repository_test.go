package postgres

import (
	"context"
	"testing"

	"github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditRepository_SaveAndGetLastHash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresAuditRepository(db)
	orgID := helpers.SeedOrg(db)
	actorID := helpers.SeedUser(db, orgID)

	lastHash, err := repo.GetLastHash(context.Background(), orgID)
	require.NoError(t, err)
	assert.Nil(t, lastHash, "no events should mean nil last hash")

	ev1 := domain.NewAuditEvent(orgID, actorID, "case.created", "case", nil, "success", nil, nil, nil)
	err = repo.Save(context.Background(), ev1)
	require.NoError(t, err)

	lastHash, err = repo.GetLastHash(context.Background(), orgID)
	require.NoError(t, err)
	require.NotNil(t, lastHash)
	assert.Equal(t, ev1.Hash, *lastHash)

	ev2 := domain.NewAuditEvent(orgID, actorID, "case.status_changed", "case", nil, "success", nil, nil, lastHash)
	err = repo.Save(context.Background(), ev2)
	require.NoError(t, err)

	lastHash, err = repo.GetLastHash(context.Background(), orgID)
	require.NoError(t, err)
	require.NotNil(t, lastHash)
	assert.Equal(t, ev2.Hash, *lastHash)
}

func TestAuditRepository_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresAuditRepository(db)

	org1 := helpers.SeedOrg(db)
	org2 := helpers.SeedOrg(db)
	actor := helpers.SeedUser(db, org1)

	ev := domain.NewAuditEvent(org1, actor, "test.action", "test", nil, "success", nil, nil, nil)
	err := repo.Save(context.Background(), ev)
	require.NoError(t, err)

	lastHash, err := repo.GetLastHash(context.Background(), org2)
	require.NoError(t, err)
	assert.Nil(t, lastHash, "other tenant should have no events")
}

func TestAuditRepository_FindByOrganization(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresAuditRepository(db)
	orgID := helpers.SeedOrg(db)
	actor := helpers.SeedUser(db, orgID)

	for i := 0; i < 3; i++ {
		ev := domain.NewAuditEvent(orgID, actor, "test.action", "test", nil, "success", nil, nil, nil)
		err := repo.Save(context.Background(), ev)
		require.NoError(t, err)
	}

	events, err := repo.FindByOrganization(context.Background(), orgID, 20, 0)
	require.NoError(t, err)
	assert.Len(t, events, 3)

	for _, ev := range events {
		assert.True(t, ev.VerifyIntegrity(), "audit event should pass integrity check")
	}
}

func TestAuditRepository_ForeignKeyConstraint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := NewPostgresAuditRepository(db)

	fakeOrgID := uuid.New()
	actor := uuid.New()

	ev := domain.NewAuditEvent(fakeOrgID, actor, "test.action", "test", nil, "success", nil, nil, nil)
	err := repo.Save(context.Background(), ev)
	require.Error(t, err, "should fail due to foreign key constraint")
}
