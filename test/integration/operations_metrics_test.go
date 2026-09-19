package integration

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/alrazihi/civora/internal/operations/domain"
	operationspostgres "github.com/alrazihi/civora/internal/operations/infrastructure/postgres"
	"github.com/alrazihi/civora/test/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedOperationsMetricsTestData(t *testing.T, db *sql.DB) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	orgID := uuid.New()
	userID := uuid.New()

	_, err := db.ExecContext(ctx, `
		INSERT INTO organizations (id, name, slug, created_at, updated_at)
		VALUES ($1, $2, $3, now(), now())
	`, orgID, "Metrics Test Org", "metrics-test-"+orgID.String()[:8])
	require.NoError(t, err)

	now := time.Now().UTC().Add(-48 * time.Hour)

	for i := 0; i < 5; i++ {
		caseID := uuid.New()
		status := "NEW"
		if i == 3 {
			status = "CLOSED"
		} else if i == 4 {
			status = "REJECTED"
		}
		_, err := db.ExecContext(ctx, `
			INSERT INTO cases (id, organization_id, case_number, title, description, status, service_type, priority, created_by, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
		`, caseID, orgID, fmt.Sprintf("METRICS-%d-%s", i, caseID.String()[:8]), "Test case", "Description", status, "EMERGENCY", "HIGH", userID, now.Add(time.Duration(i)*time.Hour))
		require.NoError(t, err)
	}

	return orgID, userID, uuid.New()
}

func TestOperationsMetrics_GetCaseVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	defer func() { _ = db.Close() }()

	orgID, _, _ := seedOperationsMetricsTestData(t, db)

	repo := operationspostgres.NewPostgresMetricsRepository(db)

	t.Run("daily case volume", func(t *testing.T) {
		metric, err := repo.GetCaseVolume(context.Background(), orgID, domain.MetricPeriodDaily, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, orgID, metric.OrganizationID)
		assert.Equal(t, domain.MetricPeriodDaily, metric.Period)
		assert.True(t, metric.TotalCases >= 0)
	})

	t.Run("weekly case volume", func(t *testing.T) {
		metric, err := repo.GetCaseVolume(context.Background(), orgID, domain.MetricPeriodWeekly, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, orgID, metric.OrganizationID)
		assert.Equal(t, domain.MetricPeriodWeekly, metric.Period)
	})

	t.Run("monthly case volume", func(t *testing.T) {
		metric, err := repo.GetCaseVolume(context.Background(), orgID, domain.MetricPeriodMonthly, time.Now().UTC())
		require.NoError(t, err)
		assert.Equal(t, orgID, metric.OrganizationID)
		assert.Equal(t, domain.MetricPeriodMonthly, metric.Period)
	})
}

func TestOperationsMetrics_GetPendingReviews(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	defer func() { _ = db.Close() }()

	orgID, _, _ := seedOperationsMetricsTestData(t, db)

	repo := operationspostgres.NewPostgresMetricsRepository(db)

	metric, err := repo.GetPendingReviews(context.Background(), orgID)
	require.NoError(t, err)
	assert.Equal(t, orgID, metric.OrganizationID)
	assert.True(t, metric.PendingReviews >= 0)
}

func TestOperationsMetrics_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := helpers.TestDB(t)
	defer func() { _ = db.Close() }()

	org1, _, _ := seedOperationsMetricsTestData(t, db)
	org2, _, _ := seedOperationsMetricsTestData(t, db)

	repo := operationspostgres.NewPostgresMetricsRepository(db)

	metric1, err := repo.GetPendingReviews(context.Background(), org1)
	require.NoError(t, err)
	assert.Equal(t, org1, metric1.OrganizationID)

	metric2, err := repo.GetPendingReviews(context.Background(), org2)
	require.NoError(t, err)
	assert.Equal(t, org2, metric2.OrganizationID)
}
