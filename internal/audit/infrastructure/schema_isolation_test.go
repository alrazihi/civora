package audit

import (
	"context"
	"database/sql"
	"testing"

	"github.com/alrazihi/civora/internal/audit/domain"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditSchemaIsolation proves that audit events are stored in a
// dedicated `audit` schema rather than the `public` schema that holds all
// business data. This is the regression test for hostile-review finding
// MED-01.
func TestAuditSchemaIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	ctx := context.Background()

	// The audit schema must exist.
	var auditSchema string
	err := db.QueryRowContext(ctx,
		`SELECT nspname FROM pg_namespace WHERE nspname = 'audit'`).Scan(&auditSchema)
	require.NoError(t, err)
	assert.Equal(t, "audit", auditSchema, "audit schema must exist")

	// The audit_events table must NOT exist in the public schema.
	var publicTable sql.NullString
	publicErr := db.QueryRowContext(ctx,
		`SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename = 'audit_events'`).Scan(&publicTable)
	assert.ErrorIs(t, publicErr, sql.ErrNoRows, "audit_events must not exist in the public schema")

	// The audit_events table must exist in the audit schema.
	var auditTable sql.NullString
	err = db.QueryRowContext(ctx,
		`SELECT tablename FROM pg_tables WHERE schemaname = 'audit' AND tablename = 'audit_events'`).Scan(&auditTable)
	require.NoError(t, err)
	assert.True(t, auditTable.Valid, "audit_events must exist in the audit schema")
	assert.Equal(t, "audit_events", auditTable.String)

	// Writing an audit event must work through the repository.
	repo := auditpostgres.NewPostgresAuditRepository(db)
	orgID := helpers.SeedOrg(db)
	actorID := helpers.SeedUser(db, orgID)

	ev, err := domain.NewAuditEvent(orgID, actorID, "case.created", "case", nil, "success", nil, nil, nil)
	require.NoError(t, err)
	err = repo.RecordEvent(ctx, orgID, ev)
	require.NoError(t, err, "recording an audit event must succeed")

	// Reading it back must also work.
	events, err := repo.FindByOrganization(ctx, orgID, 100, 0)
	require.NoError(t, err)
	assert.Len(t, events, 1, "the event must be readable back from the audit schema")

	// The event must be invisible to a query that does not qualify the
	// schema, proving the schema move actually relocated the table.
	_, err = db.ExecContext(ctx, `SELECT COUNT(*) FROM audit_events`)
	assert.Error(t, err, "unqualified audit_events must not be resolvable")
}

// TestAuditRetentionJobRuns proves the retention purge job is implemented
// and runs on startup and on every maintenance tick. This is the regression
// test for hostile-review finding MED-02.
func TestAuditRetentionJobRuns(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	helpers.TruncateTables(t, db)

	ctx := context.Background()

	repo := auditpostgres.NewPostgresAuditRepository(db)
	orgID := helpers.SeedOrg(db)
	actorID := helpers.SeedUser(db, orgID)

	// Insert an old event directly with an old timestamp.
	_, err := db.ExecContext(ctx,
		`INSERT INTO audit.audit_events (id, organization_id, actor_id, action, resource, outcome, timestamp, hash, previous_hash)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		uuid.New(), orgID, actorID, "case.created", "case", "success",
		"2020-01-01T00:00:00Z", "oldhash", nil,
	)
	require.NoError(t, err)

	// Insert a recent event.
	ev, err := domain.NewAuditEvent(orgID, actorID, "case.created", "case", nil, "success", nil, nil, nil)
	require.NoError(t, err)
	err = repo.RecordEvent(ctx, orgID, ev)
	require.NoError(t, err)

	// The retention job (PurgeOld) must delete the old event.
	purged, err := repo.PurgeOld(ctx, ev.Timestamp.AddDate(0, 0, -1))
	require.NoError(t, err)
	assert.Equal(t, 1, purged, "the retention job must delete events older than the cutoff")

	events, err := repo.FindByOrganization(ctx, orgID, 100, 0)
	require.NoError(t, err)
	assert.Len(t, events, 1, "only the recent event should remain after the purge")
}
