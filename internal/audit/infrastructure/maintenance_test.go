package audit

import (
	"context"
	"testing"

	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/config"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditMaintenanceService_VerifyOrganization(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := auditpostgres.NewPostgresAuditRepository(db)
	svc := NewAuditMaintenanceService(repo, config.AuditConfig{HashChainEnabled: true})

	orgID := helpers.SeedOrg(db)
	actorID := helpers.SeedUser(db, orgID)

	for i := 0; i < 3; i++ {
		ev, err := domain.NewAuditEvent(orgID, actorID, "case.created", "case", nil, "success", nil, nil, nil)
		require.NoError(t, err)
		err = repo.RecordEvent(context.Background(), orgID, ev)
		require.NoError(t, err)
	}

	verified, failed, err := svc.VerifyOrganization(context.Background(), orgID)
	require.NoError(t, err)
	assert.Equal(t, 3, verified, "all events should pass verification")
	assert.Equal(t, 0, failed, "no events should fail verification")
}

func TestAuditMaintenanceService_VerifyOrganization_TamperDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := auditpostgres.NewPostgresAuditRepository(db)
	svc := NewAuditMaintenanceService(repo, config.AuditConfig{HashChainEnabled: true})

	orgID := helpers.SeedOrg(db)
	actorID := helpers.SeedUser(db, orgID)

	ev, err := domain.NewAuditEvent(orgID, actorID, "case.created", "case", nil, "success", nil, nil, nil)
	require.NoError(t, err)
	err = repo.RecordEvent(context.Background(), orgID, ev)
	require.NoError(t, err)

	// Tamper with the stored hash directly in the database.
	_, err = db.ExecContext(context.Background(),
		`UPDATE audit_events SET hash = $1 WHERE id = $2`,
		"0000000000000000000000000000000000000000000000000000000000000000",
		ev.ID,
	)
	require.NoError(t, err)

	verified, failed, err := svc.VerifyOrganization(context.Background(), orgID)
	require.NoError(t, err)
	assert.Equal(t, 0, verified, "tampered event should not pass verification")
	assert.Equal(t, 1, failed, "tampered event should be detected")
}

func TestAuditMaintenanceService_PurgeOld(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := auditpostgres.NewPostgresAuditRepository(db)
	svc := NewAuditMaintenanceService(repo, config.AuditConfig{RetentionDays: 30})

	orgID := helpers.SeedOrg(db)
	actorID := helpers.SeedUser(db, orgID)

	// Create an old event by inserting directly with an old timestamp.
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO audit_events (id, organization_id, actor_id, action, resource, outcome, timestamp, hash, previous_hash)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		uuid.New(), orgID, actorID, "case.created", "case", "success",
		"2020-01-01T00:00:00Z", "oldhash", nil,
	)
	require.NoError(t, err)

	// Create a recent event.
	ev, err := domain.NewAuditEvent(orgID, actorID, "case.created", "case", nil, "success", nil, nil, nil)
	require.NoError(t, err)
	err = repo.RecordEvent(context.Background(), orgID, ev)
	require.NoError(t, err)

	purged, err := svc.PurgeOld(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, purged, "only the old event should be purged")

	events, err := repo.FindByOrganization(context.Background(), orgID, 100, 0)
	require.NoError(t, err)
	assert.Len(t, events, 1, "only the recent event should remain")
}

func TestAuditMaintenanceService_PurgeOld_Disabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	repo := auditpostgres.NewPostgresAuditRepository(db)
	cfg := config.AuditConfig{RetentionDays: 0}
	svc := NewAuditMaintenanceService(repo, cfg)

	orgID := helpers.SeedOrg(db)
	actorID := helpers.SeedUser(db, orgID)

	ev, err := domain.NewAuditEvent(orgID, actorID, "case.created", "case", nil, "success", nil, nil, nil)
	require.NoError(t, err)
	err = repo.RecordEvent(context.Background(), orgID, ev)
	require.NoError(t, err)

	purged, err := svc.PurgeOld(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, purged, "purge should be disabled when retention days is 0")
}

func TestVerifyChain(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()

	ev1, err := domain.NewAuditEvent(orgID, actorID, "case.created", "case", nil, "success", nil, nil, nil)
	require.NoError(t, err)
	ev2, err := domain.NewAuditEvent(orgID, actorID, "case.status_changed", "case", nil, "success", nil, nil, &ev1.Hash)
	require.NoError(t, err)
	ev3, err := domain.NewAuditEvent(orgID, actorID, "case.closed", "case", nil, "success", nil, nil, &ev2.Hash)
	require.NoError(t, err)

	verified, failed := domain.VerifyChain([]*domain.AuditEvent{ev1, ev2, ev3})
	assert.Equal(t, 3, verified, "all events in a valid chain should pass")
	assert.Equal(t, 0, failed, "no events should fail in a valid chain")

	// Break the chain by changing ev2's previous hash. This breaks ev2's
	// own hash integrity, but ev3 still links to ev2's original hash.
	ev2.PreviousHash = strPtr("broken")
	verified, failed = domain.VerifyChain([]*domain.AuditEvent{ev1, ev2, ev3})
	assert.Equal(t, 2, verified, "ev1 and ev3 should still pass")
	assert.Equal(t, 1, failed, "ev2 should fail because its previous hash does not match ev1's hash")
}

func TestVerifyChain_Empty(t *testing.T) {
	verified, failed := domain.VerifyChain(nil)
	assert.Equal(t, 0, verified)
	assert.Equal(t, 0, failed)
}

func strPtr(s string) *string {
	return &s
}